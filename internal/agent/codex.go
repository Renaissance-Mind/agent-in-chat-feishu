package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

type Config struct {
	Command         string
	WorkDir         string
	Model           string
	Mode            string
	ReasoningEffort string
	CodexHome       string
	Timeout         time.Duration
}

type Result struct {
	ThreadID string
	Text     string
}

type CodexRunner struct {
	cfg Config
}

func NewCodexRunner(cfg Config) *CodexRunner {
	if strings.TrimSpace(cfg.Command) == "" {
		cfg.Command = "codex"
	}
	if strings.TrimSpace(cfg.WorkDir) == "" {
		cfg.WorkDir = "."
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Minute
	}
	return &CodexRunner{cfg: cfg}
}

func (r *CodexRunner) Run(ctx context.Context, prompt, resumeThreadID string) (Result, error) {
	ctx, cancel := context.WithTimeout(ctx, r.cfg.Timeout)
	defer cancel()

	args := r.buildArgs(resumeThreadID)
	cmd := exec.CommandContext(ctx, r.cfg.Command, args...)
	cmd.Dir = r.cfg.WorkDir
	cmd.Stdin = strings.NewReader(prompt)
	if r.cfg.CodexHome != "" {
		cmd.Env = append(os.Environ(), "CODEX_HOME="+r.cfg.CodexHome)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Result{}, err
	}
	if err := cmd.Start(); err != nil {
		return Result{}, err
	}
	result, parseErr := ParseCodexJSONReader(stdout)
	waitErr := cmd.Wait()
	if parseErr != nil {
		return result, parseErr
	}
	if waitErr != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = waitErr.Error()
		}
		return result, fmt.Errorf("codex failed: %s", msg)
	}
	if result.Text == "" {
		return result, fmt.Errorf("codex returned no assistant text")
	}
	return result, nil
}

func (r *CodexRunner) buildArgs(resumeThreadID string) []string {
	var args []string
	if strings.TrimSpace(resumeThreadID) != "" {
		args = []string{"exec", "resume", "--skip-git-repo-check"}
	} else {
		args = []string{"exec", "--skip-git-repo-check"}
	}
	switch strings.ToLower(strings.TrimSpace(r.cfg.Mode)) {
	case "auto-edit", "full-auto":
		args = append(args, "--full-auto")
	case "yolo":
		args = append(args, "--dangerously-bypass-approvals-and-sandbox")
	}
	if r.cfg.Model != "" {
		args = append(args, "--model", r.cfg.Model)
	}
	if r.cfg.ReasoningEffort != "" {
		args = append(args, "-c", fmt.Sprintf("model_reasoning_effort=%q", r.cfg.ReasoningEffort))
	}
	if resumeThreadID != "" {
		args = append(args, resumeThreadID, "--json", "-")
	} else {
		args = append(args, "--json", "--cd", r.cfg.WorkDir, "-")
	}
	return args
}

func ParseCodexJSONReader(r io.Reader) (Result, error) {
	var lines []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			lines = append(lines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return Result{}, err
	}
	return ParseCodexJSONLines(lines)
}

func ParseCodexJSONLines(lines []string) (Result, error) {
	var result Result
	var texts []string
	for _, line := range lines {
		var raw map[string]any
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}
		switch raw["type"] {
		case "thread.started":
			if id, ok := raw["thread_id"].(string); ok {
				result.ThreadID = id
			}
		case "item.completed":
			item, ok := raw["item"].(map[string]any)
			if !ok {
				continue
			}
			itemType, _ := item["type"].(string)
			if itemType != "agent_message" && itemType != "message" {
				continue
			}
			text := firstString(item, "text", "content")
			if text == "" {
				text = contentPartsText(item)
			}
			if strings.TrimSpace(text) != "" {
				texts = append(texts, strings.TrimSpace(text))
			}
		}
	}
	result.Text = strings.Join(texts, "\n")
	return result, nil
}

func firstString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := m[key].(string); ok {
			return value
		}
	}
	return ""
}

func contentPartsText(item map[string]any) string {
	parts, ok := item["content"].([]any)
	if !ok {
		return ""
	}
	var out []string
	for _, part := range parts {
		obj, ok := part.(map[string]any)
		if !ok {
			continue
		}
		if text, ok := obj["text"].(string); ok && strings.TrimSpace(text) != "" {
			out = append(out, strings.TrimSpace(text))
		}
	}
	return strings.Join(out, "\n")
}
