package core

import (
	"path/filepath"
	"testing"
)

func TestProjectState_SaveLoadAndClear(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "projects", "demo.state.json")

	store := NewProjectStateStore(statePath)
	store.SetWorkDirOverride("/tmp/demo")
	store.Save()

	reloaded := NewProjectStateStore(statePath)
	if got := reloaded.WorkDirOverride(); got != "/tmp/demo" {
		t.Fatalf("WorkDirOverride() = %q, want %q", got, "/tmp/demo")
	}

	reloaded.ClearWorkDirOverride()
	reloaded.Save()

	cleared := NewProjectStateStore(statePath)
	if got := cleared.WorkDirOverride(); got != "" {
		t.Fatalf("WorkDirOverride() after clear = %q, want empty", got)
	}
}

func TestWorkspaceDirOverride(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "projects", "demo.state.json")
	workspaceA := "/tmp/workspace-a"
	workspaceB := "/tmp/workspace-b"
	channelA := "feishu:oc_a"
	channelB := "feishu:oc_b"

	store := NewProjectStateStore(statePath)
	store.SetWorkDirOverride("/tmp/global")
	store.SetWorkspaceDirOverride(workspaceA, "/tmp/workspace-a/override")
	store.SetWorkspaceDirOverride(workspaceB, "/tmp/workspace-b/override")
	store.SetChannelDirOverride(channelA, "/tmp/channel-a/override")
	store.SetChannelDirOverride(channelB, "/tmp/channel-b/override")
	store.Save()

	reloaded := NewProjectStateStore(statePath)
	if got := reloaded.WorkDirOverride(); got != "/tmp/global" {
		t.Fatalf("WorkDirOverride() = %q, want %q", got, "/tmp/global")
	}
	if got := reloaded.WorkspaceDirOverride(workspaceA); got != "/tmp/workspace-a/override" {
		t.Fatalf("WorkspaceDirOverride(%q) = %q, want %q", workspaceA, got, "/tmp/workspace-a/override")
	}
	if got := reloaded.WorkspaceDirOverride(workspaceB); got != "/tmp/workspace-b/override" {
		t.Fatalf("WorkspaceDirOverride(%q) = %q, want %q", workspaceB, got, "/tmp/workspace-b/override")
	}
	if got := reloaded.WorkspaceDirOverride("/tmp/missing"); got != "" {
		t.Fatalf("WorkspaceDirOverride(missing) = %q, want empty", got)
	}
	if got := reloaded.ChannelDirOverride(channelA); got != "/tmp/channel-a/override" {
		t.Fatalf("ChannelDirOverride(%q) = %q, want %q", channelA, got, "/tmp/channel-a/override")
	}
	if got := reloaded.ChannelDirOverride(channelB); got != "/tmp/channel-b/override" {
		t.Fatalf("ChannelDirOverride(%q) = %q, want %q", channelB, got, "/tmp/channel-b/override")
	}
	if got := reloaded.ChannelDirOverride("feishu:missing"); got != "" {
		t.Fatalf("ChannelDirOverride(missing) = %q, want empty", got)
	}

	reloaded.ClearWorkspaceDirOverride(workspaceA)
	reloaded.ClearChannelDirOverride(channelA)
	reloaded.Save()

	cleared := NewProjectStateStore(statePath)
	if got := cleared.WorkDirOverride(); got != "/tmp/global" {
		t.Fatalf("WorkDirOverride() after workspace clear = %q, want %q", got, "/tmp/global")
	}
	if got := cleared.WorkspaceDirOverride(workspaceA); got != "" {
		t.Fatalf("WorkspaceDirOverride(%q) after clear = %q, want empty", workspaceA, got)
	}
	if got := cleared.WorkspaceDirOverride(workspaceB); got != "/tmp/workspace-b/override" {
		t.Fatalf("WorkspaceDirOverride(%q) after clearing other workspace = %q, want %q", workspaceB, got, "/tmp/workspace-b/override")
	}
	if got := cleared.ChannelDirOverride(channelA); got != "" {
		t.Fatalf("ChannelDirOverride(%q) after clear = %q, want empty", channelA, got)
	}
	if got := cleared.ChannelDirOverride(channelB); got != "/tmp/channel-b/override" {
		t.Fatalf("ChannelDirOverride(%q) after clearing other channel = %q, want %q", channelB, got, "/tmp/channel-b/override")
	}
}
