//go:build darwin

package daemon

import (
	"errors"
	"strings"
	"testing"
)

func TestBuildPlist_KeepAliveDoesNotRestartOnCleanExit(t *testing.T) {
	cfg := Config{
		BinaryPath: "/opt/agentchat/agentchat",
		WorkDir:    "/tmp/wd",
		LogFile:    "/tmp/log",
		LogMaxSize: 10485760,
		EnvPATH:    "/usr/bin",
	}
	xml := buildPlist(cfg)
	if !strings.Contains(xml, "<key>SuccessfulExit</key>") {
		t.Fatal("plist should use KeepAlive dict with SuccessfulExit so exit 0 does not respawn")
	}
	// Boolean KeepAlive causes launchd to restart after every exit, including SIGTERM shutdown.
	if strings.Contains(xml, "<key>KeepAlive</key>\n\t<true/>") {
		t.Fatal("plist must not use boolean KeepAlive true")
	}
}

func TestBootstrapLaunchdWithRetryRetriesTransientBootstrapFailure(t *testing.T) {
	prevRun := runLaunchctl
	prevDelay := launchdBootstrapRetryDelay
	attempts := 0
	runLaunchctl = func(args ...string) (string, error) {
		attempts++
		if attempts == 1 {
			return "Bootstrap failed: 5: Input/output error", errors.New("exit status 5")
		}
		if strings.Join(args, " ") != "bootstrap gui/501 /tmp/agentchat.plist" {
			t.Fatalf("args = %v", args)
		}
		return "", nil
	}
	launchdBootstrapRetryDelay = 0
	t.Cleanup(func() {
		runLaunchctl = prevRun
		launchdBootstrapRetryDelay = prevDelay
	})

	out, err := bootstrapLaunchdWithRetry("gui/501", "/tmp/agentchat.plist")
	if err != nil {
		t.Fatalf("bootstrapLaunchdWithRetry returned error: %v output=%q", err, out)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}
