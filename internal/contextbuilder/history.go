package contextbuilder

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/sariel/agent-in-chat-feishu/internal/identity"
)

type Entry struct {
	MessageID  string
	SenderID   string
	SenderType string
	MsgType    string
	CreatedAt  time.Time
	Content    string
}

func RenderHistory(entries []Entry, currentMessageID string, names *identity.Cache) string {
	if len(entries) == 0 {
		return ""
	}
	ordered := append([]Entry(nil), entries...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].CreatedAt.Equal(ordered[j].CreatedAt) {
			return ordered[i].MessageID < ordered[j].MessageID
		}
		return ordered[i].CreatedAt.Before(ordered[j].CreatedAt)
	})

	lines := []string{
		"[Feishu group history]",
		"Recent group messages fetched at trigger time. Use them as background and answer the current trigger.",
	}
	for _, entry := range ordered {
		if entry.MessageID != "" && entry.MessageID == currentMessageID {
			continue
		}
		if strings.EqualFold(entry.MsgType, "interactive") {
			continue
		}
		content := CompactText(entry.Content)
		if content == "" {
			continue
		}
		name := senderName(entry, names)
		lines = append(lines, fmt.Sprintf("[%s %s] %s", entry.CreatedAt.Local().Format("15:04"), name, content))
	}
	if len(lines) == 2 {
		return ""
	}
	return strings.Join(lines, "\n")
}

func senderName(entry Entry, names *identity.Cache) string {
	if names != nil {
		if name := names.Resolve(entry.SenderID, entry.SenderType); name != "" {
			return CompactText(name)
		}
	}
	switch strings.ToLower(strings.TrimSpace(entry.SenderType)) {
	case "app":
		return "App"
	case "user":
		return "User"
	default:
		return "Unknown"
	}
}

func CompactText(text string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
}
