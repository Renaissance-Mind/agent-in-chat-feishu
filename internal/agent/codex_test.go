package agent

import "testing"

func TestParseCodexJSONEventsCollectsThreadAndFinalText(t *testing.T) {
	lines := []string{
		`{"type":"thread.started","thread_id":"019e-test"}`,
		`{"type":"turn.started"}`,
		`{"type":"item.completed","item":{"type":"agent_message","text":"先想一下"}}`,
		`{"type":"item.started","item":{"type":"command_execution","command":"pwd"}}`,
		`{"type":"item.completed","item":{"type":"agent_message","text":"最终回答"}}`,
		`{"type":"turn.completed"}`,
	}

	result, err := ParseCodexJSONLines(lines)
	if err != nil {
		t.Fatalf("ParseCodexJSONLines() error = %v", err)
	}
	if result.ThreadID != "019e-test" {
		t.Fatalf("ThreadID = %q, want 019e-test", result.ThreadID)
	}
	if result.Text != "先想一下\n最终回答" {
		t.Fatalf("Text = %q, want joined agent messages", result.Text)
	}
}
