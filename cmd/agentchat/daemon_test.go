package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Renaissance-Mind/agent-in-chat-feishu/daemon"
)

type recordingDaemonManager struct {
	status        *daemon.Status
	installErr    error
	installCalled bool
	installCfg    daemon.Config
}

func (m *recordingDaemonManager) Install(cfg daemon.Config) error {
	m.installCalled = true
	m.installCfg = cfg
	return m.installErr
}

func (m *recordingDaemonManager) Uninstall() error { return nil }
func (m *recordingDaemonManager) Start() error     { return nil }
func (m *recordingDaemonManager) Stop() error      { return nil }
func (m *recordingDaemonManager) Restart() error   { return nil }

func (m *recordingDaemonManager) Status() (*daemon.Status, error) {
	if m.status != nil {
		return m.status, nil
	}
	return &daemon.Status{Platform: "test"}, nil
}

func (m *recordingDaemonManager) Platform() string {
	if m.status != nil && m.status.Platform != "" {
		return m.status.Platform
	}
	return "test"
}

func TestParseDaemonInstallArgs_ConfigSetsWorkDir(t *testing.T) {
	cfg, force, err := parseDaemonInstallArgs([]string{"--config", "/tmp/example/config.toml"})
	if err != nil {
		t.Fatalf("parseDaemonInstallArgs returned error: %v", err)
	}
	if force {
		t.Fatalf("force = true, want false")
	}

	want := filepath.Clean("/tmp/example")
	if cfg.WorkDir != want {
		t.Fatalf("cfg.WorkDir = %q, want %q", cfg.WorkDir, want)
	}
}

func TestParseDaemonInstallArgs_ConfigEqualsFormSetsWorkDir(t *testing.T) {
	cfg, _, err := parseDaemonInstallArgs([]string{"--config=/tmp/example/config.toml"})
	if err != nil {
		t.Fatalf("parseDaemonInstallArgs returned error: %v", err)
	}

	want := filepath.Clean("/tmp/example")
	if cfg.WorkDir != want {
		t.Fatalf("cfg.WorkDir = %q, want %q", cfg.WorkDir, want)
	}
}

func TestParseDaemonInstallArgs_WorkDirOverridesConfig(t *testing.T) {
	cfg, force, err := parseDaemonInstallArgs([]string{
		"--config", "/tmp/example/config.toml",
		"--work-dir", "/tmp/override",
		"--force",
	})
	if err != nil {
		t.Fatalf("parseDaemonInstallArgs returned error: %v", err)
	}
	if !force {
		t.Fatalf("force = false, want true")
	}

	want := filepath.Clean("/tmp/override")
	if cfg.WorkDir != want {
		t.Fatalf("cfg.WorkDir = %q, want %q", cfg.WorkDir, want)
	}
}

func TestParseDaemonInstallArgs_EnvPath(t *testing.T) {
	cfg, _, err := parseDaemonInstallArgs([]string{"--env-path", "/custom/bin:/usr/bin"})
	if err != nil {
		t.Fatalf("parseDaemonInstallArgs returned error: %v", err)
	}
	if cfg.EnvPATH != "/custom/bin:/usr/bin" {
		t.Fatalf("cfg.EnvPATH = %q, want explicit env path", cfg.EnvPATH)
	}
}

func TestParseDaemonInstallArgs_EnvPathEqualsForm(t *testing.T) {
	cfg, _, err := parseDaemonInstallArgs([]string{"--env-path=/custom/bin:/usr/bin"})
	if err != nil {
		t.Fatalf("parseDaemonInstallArgs returned error: %v", err)
	}
	if cfg.EnvPATH != "/custom/bin:/usr/bin" {
		t.Fatalf("cfg.EnvPATH = %q, want explicit env path", cfg.EnvPATH)
	}
}

func TestInstallDaemonServiceInstallsAndSavesMetadata(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	workDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workDir, "config.toml"), []byte("language = \"zh\"\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	mgr := &recordingDaemonManager{status: &daemon.Status{Platform: "testd"}}
	prevFactory := daemonManagerFactory
	daemonManagerFactory = func() (daemon.Manager, error) { return mgr, nil }
	t.Cleanup(func() { daemonManagerFactory = prevFactory })

	result, err := installDaemonService(daemon.Config{WorkDir: workDir, EnvPATH: "/custom/bin"}, false)
	if err != nil {
		t.Fatalf("installDaemonService returned error: %v", err)
	}
	if !mgr.installCalled {
		t.Fatal("expected daemon manager Install to be called")
	}
	if mgr.installCfg.WorkDir != workDir {
		t.Fatalf("WorkDir = %q, want %q", mgr.installCfg.WorkDir, workDir)
	}
	if mgr.installCfg.EnvPATH != "/custom/bin" {
		t.Fatalf("EnvPATH = %q, want explicit PATH", mgr.installCfg.EnvPATH)
	}
	if result.Platform != "testd" || result.WorkDir != workDir {
		t.Fatalf("result = %+v, want platform testd and work dir %q", result, workDir)
	}

	meta, err := daemon.LoadMeta()
	if err != nil {
		t.Fatalf("LoadMeta returned error: %v", err)
	}
	if meta.WorkDir != workDir || meta.LogFile != result.LogFile || meta.BinaryPath != result.BinaryPath {
		t.Fatalf("metadata = %+v, result = %+v", meta, result)
	}
}

func TestInstallDaemonServiceRejectsMissingConfig(t *testing.T) {
	_, err := installDaemonService(daemon.Config{WorkDir: t.TempDir()}, false)
	if err == nil {
		t.Fatal("expected missing config error, got nil")
	}
	if !strings.Contains(err.Error(), "config.toml not found") {
		t.Fatalf("error = %v, want missing config message", err)
	}
}

func TestInstallDaemonServiceRequiresForceWhenInstalled(t *testing.T) {
	workDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workDir, "config.toml"), []byte("language = \"zh\"\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	mgr := &recordingDaemonManager{status: &daemon.Status{Installed: true, Platform: "testd"}}
	prevFactory := daemonManagerFactory
	daemonManagerFactory = func() (daemon.Manager, error) { return mgr, nil }
	t.Cleanup(func() { daemonManagerFactory = prevFactory })

	_, err := installDaemonService(daemon.Config{WorkDir: workDir}, false)
	if err == nil {
		t.Fatal("expected already-installed error, got nil")
	}
	if mgr.installCalled {
		t.Fatal("Install should not be called when service is already installed without force")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Fatalf("error = %v, want --force guidance", err)
	}
}
