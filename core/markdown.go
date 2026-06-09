package core

import (
	"regexp"
	"strings"
)

var (
	reCodeBlock  = regexp.MustCompile("(?s)```[a-zA-Z]*\n?(.*?)```")
	reInlineCode = regexp.MustCompile("`([^`]+)`")
	reBoldAst    = regexp.MustCompile(`\*\*(.+?)\*\*`)
	reBoldUnd    = regexp.MustCompile(`__(.+?)__`)
	reItalicAst  = regexp.MustCompile(`\*(.+?)\*`)
	reItalicUnd  = regexp.MustCompile(`_(.+?)_`)
	reStrike     = regexp.MustCompile(`~~(.+?)~~`)
	reLink       = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	reHeading    = regexp.MustCompile(`(?m)^#{1,6}\s+`)
	reHorizontal = regexp.MustCompile(`(?m)^---+\s*$`)
	reBlockquote = regexp.MustCompile(`(?m)^>\s?`)
)

// StripMarkdown converts Markdown-formatted text to clean plain text.
// Useful for fallback surfaces that do not support Markdown rendering.
func StripMarkdown(s string) string {
	// Preserve code block content but remove fences
	s = reCodeBlock.ReplaceAllString(s, "$1")

	// Inline code — remove backticks
	s = reInlineCode.ReplaceAllString(s, "$1")

	// Bold / italic / strikethrough — keep text
	s = reBoldAst.ReplaceAllString(s, "$1")
	s = reBoldUnd.ReplaceAllString(s, "$1")
	s = reItalicAst.ReplaceAllString(s, "$1")
	s = reItalicUnd.ReplaceAllString(s, "$1")
	s = reStrike.ReplaceAllString(s, "$1")

	// Links [text](url) → text (url)
	s = reLink.ReplaceAllString(s, "$1 ($2)")

	// Headings — remove # prefix
	s = reHeading.ReplaceAllString(s, "")

	// Horizontal rules
	s = reHorizontal.ReplaceAllString(s, "")

	// Blockquotes
	s = reBlockquote.ReplaceAllString(s, "")

	// Collapse 3+ consecutive blank lines into 2
	s = regexp.MustCompile(`\n{3,}`).ReplaceAllString(s, "\n\n")

	return strings.TrimSpace(s)
}

// SplitMessageCodeFenceAware splits text into chunks while preserving Markdown
// code fences. If a boundary falls inside a fenced block, the current chunk is
// closed and the next chunk reopens the same fence.
func SplitMessageCodeFenceAware(text string, maxLen int) []string {
	if maxLen <= 0 || len(text) <= maxLen {
		return []string{text}
	}

	const closingFence = "\n```"

	lines := strings.Split(text, "\n")
	var chunks []string
	var current []string
	currentLen := 0
	openFence := ""

	for _, line := range lines {
		lineLen := len(line) + 1
		limit := maxLen
		if openFence != "" {
			limit -= len(closingFence)
		}

		if currentLen+lineLen > limit && len(current) > 0 {
			chunk := strings.Join(current, "\n")
			if openFence != "" {
				chunk += closingFence
			}
			chunks = append(chunks, chunk)

			current = nil
			currentLen = 0
			if openFence != "" {
				current = append(current, openFence)
				currentLen = len(openFence) + 1
			}
		}

		current = append(current, line)
		currentLen += lineLen

		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if openFence != "" {
				openFence = ""
			} else {
				openFence = trimmed
			}
		}
	}

	if len(current) > 0 {
		chunk := strings.Join(current, "\n")
		if openFence != "" {
			chunk += closingFence
		}
		chunks = append(chunks, chunk)
	}

	return chunks
}
