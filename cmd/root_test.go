package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestCommandErrorDoesNotPrintUsage(t *testing.T) {
	var output bytes.Buffer
	stdout, stderr := rootCmd.OutOrStdout(), rootCmd.ErrOrStderr()
	rootCmd.SetOut(&output)
	rootCmd.SetErr(&output)
	rootCmd.SetArgs([]string{"--invalid-test-flag"})
	t.Cleanup(func() {
		rootCmd.SetOut(stdout)
		rootCmd.SetErr(stderr)
		rootCmd.SetArgs(nil)
		_ = rootCmd.Flags().Set("help", "false")
	})
	if err := rootCmd.Execute(); err == nil {
		t.Fatal("invalid flag must return an error")
	}
	if !strings.Contains(output.String(), "unknown flag") {
		t.Fatalf("error was hidden: %q", output.String())
	}
	if strings.Contains(output.String(), "Usage:") {
		t.Fatalf("error printed Usage: %s", output.String())
	}
	output.Reset()
	rootCmd.SetArgs([]string{"--help"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Usage:") {
		t.Fatal("--help must still show usage")
	}
}
