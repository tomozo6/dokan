package aws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

const (
	pluginShutdownTimeout = 5 * time.Second
	sessionCleanupTimeout = 5 * time.Second
)

// runSSMSession owns both the local plugin and the AWS session it creates.
func runSSMSession(c *ssm.Client, input *ssm.StartSessionInput, region, profile string) (err error) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer stop()

	output, err := c.StartSession(ctx, input)
	if err != nil {
		return fmt.Errorf("start SSM session: %w", err)
	}
	// Cleanup must also run on lookup/start failure, nonzero exit, and interruption.
	// Its context is independent of the canceled session context.
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), sessionCleanupTimeout)
		defer cancel()
		_, cleanupErr := c.TerminateSession(cleanupCtx, &ssm.TerminateSessionInput{SessionId: output.SessionId})
		if cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("terminate SSM session: %w", cleanupErr))
		}
	}()

	encodedInput, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("encode session request: %w", err)
	}
	encodedOutput, err := json.Marshal(output)
	if err != nil {
		return fmt.Errorf("encode session response: %w", err)
	}
	plugin, err := resolveSessionManagerPluginCommand()
	if err != nil {
		return err
	}
	cmd := exec.Command(plugin, string(encodedOutput), region, "StartSession", resolvePluginProfile(profile), string(encodedInput))
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return runSessionPlugin(ctx, cmd, pluginShutdownTimeout)
}

// Wait is called exactly once, including when the child ignores interruption.
func runSessionPlugin(ctx context.Context, cmd *exec.Cmd, grace time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start session-manager-plugin: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("session-manager-plugin: %w", err)
		}
		return nil
	case <-ctx.Done():
	}

	// A terminal Ctrl+C normally reaches both processes. Forward it as well to
	// cover a signal delivered only to dokan. Windows cannot send os.Interrupt
	// to a child, so fall back to Kill there if Signal is unsupported.
	signalErr := cmd.Process.Signal(os.Interrupt)
	timer := time.NewTimer(grace)
	defer timer.Stop()
	if signalErr != nil && !errors.Is(signalErr, os.ErrProcessDone) {
		timer.Reset(0)
	}
	select {
	case <-done:
		return ctx.Err()
	case <-timer.C:
		killErr := cmd.Process.Kill()
		// Reap the child before returning, even after forced termination.
		<-done
		if killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
			return errors.Join(ctx.Err(), fmt.Errorf("kill session-manager-plugin: %w", killErr))
		}
		return ctx.Err()
	}
}
