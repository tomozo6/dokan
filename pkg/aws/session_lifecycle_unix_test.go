//go:build !windows

package aws

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	sdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

type sessionTransport func(*http.Request) (*http.Response, error)

func (f sessionTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func sessionTestClient(onTerminate func(context.Context) error) *ssm.Client {
	return ssm.NewFromConfig(sdk.Config{Region: "us-east-1", RetryMaxAttempts: 1, Credentials: sdk.AnonymousCredentials{}, HTTPClient: &http.Client{Transport: sessionTransport(func(r *http.Request) (*http.Response, error) {
		body := `{"SessionId":"test-session","StreamUrl":"wss://example.invalid","TokenValue":"test-token"}`
		if strings.HasSuffix(r.Header.Get("X-Amz-Target"), "TerminateSession") {
			if err := onTerminate(r.Context()); err != nil {
				return nil, err
			}
			body = `{}`
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/x-amz-json-1.1"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}})
}

func invokeTestSession(mode string, c *ssm.Client) error {
	target, host, region, profile := "test-target", "example.invalid", "us-east-1", "test"
	port := int32(5432)
	if mode == "login" {
		return SSMEC2Login(c, &target, &region, &profile)
	} else {
		return SSMPortForwarding(c, &target, &host, &port, &port, &region, &profile)
	}
}

func writeTestPlugin(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "session-manager-plugin"), []byte("#!/bin/sh\n"+body+"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestSessionCleanupOnPluginExit(t *testing.T) {
	for _, mode := range []string{"forward", "login"} {
		for _, body := range []string{"exit 0", "exit 1"} {
			t.Run(mode+"/"+body, func(t *testing.T) {
				t.Setenv("PATH", writeTestPlugin(t, body))
				var calls atomic.Int32
				err := invokeTestSession(mode, sessionTestClient(func(ctx context.Context) error { calls.Add(1); return nil }))
				if (err != nil) != (body == "exit 1") {
					t.Fatalf("session error = %v for %s", err, body)
				}
				if calls.Load() != 1 {
					t.Fatalf("TerminateSession calls = %d, want 1", calls.Load())
				}
			})
		}
	}
}

func TestSessionCleanupWhenPluginMissing(t *testing.T) {
	for _, mode := range []string{"forward", "login"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("PATH", t.TempDir())
			var calls atomic.Int32
			invokeTestSession(mode, sessionTestClient(func(ctx context.Context) error { calls.Add(1); return nil }))
			if calls.Load() != 1 {
				t.Fatalf("TerminateSession calls = %d, want 1", calls.Load())
			}
		})
	}
}

// A separate parent process receives the same group SIGINT as its plugin.
func TestSessionProcessHelper(t *testing.T) {
	if os.Getenv("DOKAN_SESSION_TEST_HELPER") != "1" {
		return
	}
	c := sessionTestClient(func(ctx context.Context) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if _, ok := ctx.Deadline(); !ok {
			return fmt.Errorf("cleanup has no deadline")
		}
		return os.WriteFile(os.Getenv("DOKAN_SESSION_CLEANUP"), []byte("terminated"), 0600)
	})
	invokeTestSession(os.Getenv("DOKAN_SESSION_MODE"), c)
	os.Exit(0)
}

func TestSessionInterruptReapsUnresponsivePlugin(t *testing.T) {
	dir := t.TempDir()
	pidfile := filepath.Join(dir, "pid")
	cleanup := filepath.Join(dir, "cleanup")
	plugin := writeTestPlugin(t, "trap '' INT\necho $$ > \"$DOKAN_SESSION_PIDFILE\"\nexec /bin/sleep 30")
	cmd := exec.Command(os.Args[0], "-test.run=^TestSessionProcessHelper$")
	cmd.Env = append(os.Environ(), "DOKAN_SESSION_TEST_HELPER=1", "DOKAN_SESSION_MODE=forward", "DOKAN_SESSION_PIDFILE="+pidfile, "DOKAN_SESSION_CLEANUP="+cleanup, "PATH="+plugin)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// Direct descriptors avoid pipes kept open by a leaked plugin.
	null, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer null.Close()
	cmd.Stdout = null
	cmd.Stderr = null
	cmd.Stdin = null
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	var pid int
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		data, _ := os.ReadFile(pidfile)
		pid, _ = strconv.Atoi(strings.TrimSpace(string(data)))
		if pid > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pid == 0 {
		t.Fatal("plugin never became ready")
	}
	child, _ := os.FindProcess(pid)
	defer child.Kill()
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(8 * time.Second):
		t.Fatal("dokan did not finish shutdown")
	}
	if err := child.Signal(syscall.Signal(0)); err == nil {
		t.Error("plugin remains alive or unreaped after dokan exits")
	}
	if _, err := os.Stat(cleanup); err != nil {
		t.Error("AWS session cleanup did not finish with an independent deadline")
	}
}

func TestSessionReportsPluginAndCleanupFailures(t *testing.T) {
	t.Setenv("PATH", writeTestPlugin(t, "exit 1"))
	cleanupFailure := errors.New("test cleanup failure")
	err := invokeTestSession("forward", sessionTestClient(func(ctx context.Context) error {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > sessionCleanupTimeout || ctx.Err() != nil {
			t.Error("cleanup context must have its own live deadline")
		}
		return cleanupFailure
	}))
	var exit *exec.ExitError
	if !errors.As(err, &exit) || !strings.Contains(err.Error(), "test cleanup failure") {
		t.Fatalf("plugin and cleanup errors must both be preserved: %v", err)
	}
}

func TestSessionCleanupWhenPluginCannotStart(t *testing.T) {
	dir := writeTestPlugin(t, "")
	if err := os.WriteFile(filepath.Join(dir, "session-manager-plugin"), []byte("#!/nonexistent/dokan-test-interpreter\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	var calls atomic.Int32
	err := invokeTestSession("login", sessionTestClient(func(ctx context.Context) error { calls.Add(1); return nil }))
	if err == nil || calls.Load() != 1 {
		t.Fatalf("start failure: err=%v, cleanup calls=%d", err, calls.Load())
	}
}
