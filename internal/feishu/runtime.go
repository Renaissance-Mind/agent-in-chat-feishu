package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
	"github.com/sariel/agent-in-chat-feishu/internal/agent"
	"github.com/sariel/agent-in-chat-feishu/internal/config"
	"github.com/sariel/agent-in-chat-feishu/internal/contextbuilder"
	"github.com/sariel/agent-in-chat-feishu/internal/identity"
	"github.com/sariel/agent-in-chat-feishu/internal/store"
)

const dedupeTTL = 6 * time.Hour

type Runner interface {
	Run(ctx context.Context, prompt, resumeThreadID string) (agent.Result, error)
}

type Runtime struct {
	cfg        config.Config
	api        *API
	identity   *identity.Cache
	sessions   *store.SessionStore
	runner     Runner
	botOpenID  string
	botAppName string

	seenMu sync.Mutex
	seen   map[string]time.Time

	chatLocksMu sync.Mutex
	chatLocks   map[string]*sync.Mutex
}

type inboundMessage struct {
	messageID  string
	chatID     string
	senderID   string
	senderType string
	text       string
	createdAt  time.Time
}

func NewRuntime(cfg config.Config) (*Runtime, error) {
	cfg.ApplyDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if err := config.EnsureDataDirs(cfg.DataDir); err != nil {
		return nil, err
	}
	names, err := identity.Open(identity.CachePath(cfg.DataDir))
	if err != nil {
		return nil, err
	}
	sessions, err := store.Open(store.SessionsPath(cfg.DataDir))
	if err != nil {
		return nil, err
	}
	runner := agent.NewCodexRunner(agent.Config{
		Command:         cfg.Agent.Command,
		WorkDir:         cfg.Agent.WorkDir,
		Model:           cfg.Agent.Model,
		Mode:            cfg.Agent.Mode,
		ReasoningEffort: cfg.Agent.ReasoningEffort,
		CodexHome:       cfg.Agent.CodexHome,
		Timeout:         cfg.AgentTimeout(),
	})
	return &Runtime{
		cfg:       cfg,
		api:       NewAPI(cfg.Feishu.AppID, cfg.Feishu.AppSecret, cfg.Feishu.BaseURL),
		identity:  names,
		sessions:  sessions,
		runner:    runner,
		seen:      make(map[string]time.Time),
		chatLocks: make(map[string]*sync.Mutex),
	}, nil
}

func (r *Runtime) Start(ctx context.Context) error {
	if err := r.initialize(ctx); err != nil {
		return err
	}
	eventHandler := dispatcher.NewEventDispatcher("", "").
		OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
			return r.onMessage(ctx, event)
		})

	opts := []larkws.ClientOption{
		larkws.WithEventHandler(eventHandler),
		larkws.WithLogLevel(larkcore.LogLevelInfo),
	}
	if baseURL := strings.TrimRight(r.cfg.Feishu.BaseURL, "/"); baseURL != "" && baseURL != "https://open.feishu.cn" {
		opts = append(opts, larkws.WithDomain(baseURL))
	}
	client := larkws.NewClient(r.cfg.Feishu.AppID, r.cfg.Feishu.AppSecret, opts...)
	slog.Info("feishu websocket starting")
	return client.Start(ctx)
}

func (r *Runtime) initialize(ctx context.Context) error {
	bot, err := r.api.FetchBotInfo(ctx)
	if err != nil {
		return fmt.Errorf("fetch bot info: %w", err)
	}
	if strings.TrimSpace(bot.OpenID) == "" {
		return fmt.Errorf("fetch bot info: empty bot open_id")
	}
	r.botOpenID = bot.OpenID
	r.botAppName = bot.AppName
	if bot.AppName != "" {
		r.identity.PutBot(bot.OpenID, bot.AppName)
		r.identity.PutApp(r.cfg.Feishu.AppID, bot.AppName)
		if err := r.identity.Save(); err != nil {
			return fmt.Errorf("save bot identity: %w", err)
		}
	}
	slog.Info("feishu bot identified", "bot_open_id", bot.OpenID, "app_name", bot.AppName)
	return nil
}

func (r *Runtime) onMessage(_ context.Context, event *larkim.P2MessageReceiveV1) error {
	msg, ok := r.extractInbound(event)
	if !ok {
		return nil
	}
	go r.processMessage(context.Background(), msg)
	return nil
}

func (r *Runtime) extractInbound(event *larkim.P2MessageReceiveV1) (inboundMessage, bool) {
	if event == nil || event.Event == nil || event.Event.Message == nil {
		return inboundMessage{}, false
	}
	msg := event.Event.Message
	if ptrString(msg.MessageType) != "text" {
		return inboundMessage{}, false
	}
	if ptrString(msg.ChatType) != "group" {
		return inboundMessage{}, false
	}
	chatID := ptrString(msg.ChatId)
	if chatID == "" || !r.chatAllowed(chatID) {
		return inboundMessage{}, false
	}
	messageID := ptrString(msg.MessageId)
	if messageID == "" || !r.markSeen(messageID) {
		return inboundMessage{}, false
	}
	if !isBotMentioned(msg.Mentions, r.botOpenID) {
		return inboundMessage{}, false
	}
	if msg.Content == nil {
		return inboundMessage{}, false
	}
	text, err := parseTextContent(*msg.Content)
	if err != nil {
		slog.Warn("failed to parse text message content", "message_id", messageID, "error", err)
		return inboundMessage{}, false
	}
	text = stripMentions(text, msg.Mentions, r.botOpenID)
	if text == "" {
		return inboundMessage{}, false
	}
	senderID, senderType := eventSender(event.Event.Sender)
	return inboundMessage{
		messageID:  messageID,
		chatID:     chatID,
		senderID:   senderID,
		senderType: senderType,
		text:       text,
		createdAt:  parseEventTime(ptrString(msg.CreateTime)),
	}, true
}

func (r *Runtime) processMessage(ctx context.Context, msg inboundMessage) {
	lock := r.lockForChat(msg.chatID)
	lock.Lock()
	defer lock.Unlock()

	timeout := r.cfg.AgentTimeout() + 2*time.Minute
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	reactionID := r.addReaction(msg.messageID, r.cfg.Feishu.ReactionEmoji)
	defer func() {
		if reactionID != "" {
			r.removeReaction(msg.messageID, reactionID)
		}
	}()

	if err := r.api.RefreshIdentities(ctx, msg.chatID, r.identity); err != nil {
		slog.Warn("identity refresh failed", "chat_id", msg.chatID, "error", err)
	}
	history, err := r.api.FetchHistory(ctx, msg.chatID, r.cfg.Context.MaxMessages, r.cfg.ContextMaxAge())
	if err != nil {
		slog.Warn("history fetch failed", "chat_id", msg.chatID, "error", err)
	}

	historyText := contextbuilder.RenderHistory(history, msg.messageID, r.identity)
	sender := r.identity.Resolve(msg.senderID, msg.senderType)
	if sender == "" {
		sender = fallbackSenderName(msg.senderType)
	}
	prompt := buildPrompt(historyText, sender, msg.text)
	resumeThreadID := r.sessions.Get(msg.chatID)
	slog.Info("running codex for feishu mention", "chat_id", msg.chatID, "message_id", msg.messageID, "resume", resumeThreadID != "")
	result, err := r.runner.Run(ctx, prompt, resumeThreadID)
	if err != nil {
		slog.Error("codex run failed", "chat_id", msg.chatID, "message_id", msg.messageID, "error", err)
		_ = r.api.ReplyText(ctx, msg.messageID, "Codex 执行失败: "+compactForReply(err.Error(), 500))
		return
	}
	if result.ThreadID != "" {
		if err := r.sessions.Set(msg.chatID, result.ThreadID); err != nil {
			slog.Warn("session save failed", "chat_id", msg.chatID, "thread_id", result.ThreadID, "error", err)
		}
	}
	reply := strings.TrimSpace(result.Text)
	if reply == "" {
		return
	}
	if err := r.api.ReplyText(ctx, msg.messageID, compactForReply(reply, 30000)); err != nil {
		slog.Error("feishu reply failed", "chat_id", msg.chatID, "message_id", msg.messageID, "error", err)
		return
	}
	if reactionID != "" {
		r.removeReaction(msg.messageID, reactionID)
		reactionID = ""
	}
	r.addReaction(msg.messageID, r.cfg.Feishu.DoneEmoji)
}

func (r *Runtime) addReaction(messageID, emojiType string) string {
	if strings.TrimSpace(messageID) == "" || strings.TrimSpace(emojiType) == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	reactionID, err := r.api.CreateReaction(ctx, messageID, emojiType)
	if err != nil {
		slog.Debug("feishu add reaction failed", "message_id", messageID, "emoji", emojiType, "error", err)
		return ""
	}
	return reactionID
}

func (r *Runtime) removeReaction(messageID, reactionID string) {
	if strings.TrimSpace(messageID) == "" || strings.TrimSpace(reactionID) == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := r.api.DeleteReaction(ctx, messageID, reactionID); err != nil {
		slog.Debug("feishu remove reaction failed", "message_id", messageID, "reaction_id", reactionID, "error", err)
	}
}

func (r *Runtime) chatAllowed(chatID string) bool {
	if len(r.cfg.Feishu.AllowedChats) == 0 {
		return true
	}
	for _, allowed := range r.cfg.Feishu.AllowedChats {
		if strings.TrimSpace(allowed) == chatID {
			return true
		}
	}
	return false
}

func (r *Runtime) markSeen(messageID string) bool {
	now := time.Now()
	r.seenMu.Lock()
	defer r.seenMu.Unlock()
	if r.seen == nil {
		r.seen = make(map[string]time.Time)
	}
	if _, ok := r.seen[messageID]; ok {
		return false
	}
	for id, seenAt := range r.seen {
		if seenAt.Add(dedupeTTL).Before(now) {
			delete(r.seen, id)
		}
	}
	r.seen[messageID] = now
	return true
}

func (r *Runtime) lockForChat(chatID string) *sync.Mutex {
	r.chatLocksMu.Lock()
	defer r.chatLocksMu.Unlock()
	if r.chatLocks == nil {
		r.chatLocks = make(map[string]*sync.Mutex)
	}
	lock := r.chatLocks[chatID]
	if lock == nil {
		lock = &sync.Mutex{}
		r.chatLocks[chatID] = lock
	}
	return lock
}

func buildPrompt(historyText, senderName, text string) string {
	var parts []string
	if strings.TrimSpace(historyText) != "" {
		parts = append(parts, strings.TrimSpace(historyText))
	}
	senderName = strings.TrimSpace(senderName)
	if senderName == "" {
		senderName = "User"
	}
	parts = append(parts, "[Current trigger]\n"+senderName+": "+strings.TrimSpace(text))
	return strings.Join(parts, "\n\n")
}

func parseTextContent(content string) (string, error) {
	var body struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(content), &body); err != nil {
		return "", err
	}
	return body.Text, nil
}

func isBotMentioned(mentions []*larkim.MentionEvent, botOpenID string) bool {
	if botOpenID == "" {
		return false
	}
	for _, mention := range mentions {
		if mention != nil && mention.Id != nil && mention.Id.OpenId != nil && *mention.Id.OpenId == botOpenID {
			return true
		}
	}
	return false
}

func stripMentions(text string, mentions []*larkim.MentionEvent, botOpenID string) string {
	for _, mention := range mentions {
		if mention == nil || mention.Key == nil {
			continue
		}
		key := *mention.Key
		if botOpenID != "" && mention.Id != nil && mention.Id.OpenId != nil && *mention.Id.OpenId == botOpenID {
			text = strings.ReplaceAll(text, key, "")
			continue
		}
		if mention.Name != nil && strings.TrimSpace(*mention.Name) != "" {
			text = strings.ReplaceAll(text, key, "@"+strings.TrimSpace(*mention.Name))
		} else {
			text = strings.ReplaceAll(text, key, "")
		}
	}
	return contextbuilder.CompactText(text)
}

func eventSender(sender *larkim.EventSender) (string, string) {
	if sender == nil {
		return "", ""
	}
	senderType := ptrString(sender.SenderType)
	senderID := ""
	if sender.SenderId != nil && sender.SenderId.OpenId != nil {
		senderID = *sender.SenderId.OpenId
	}
	return senderID, senderType
}

func fallbackSenderName(senderType string) string {
	switch strings.ToLower(strings.TrimSpace(senderType)) {
	case "app":
		return "App"
	case "user":
		return "User"
	default:
		return "Unknown"
	}
}

func parseEventTime(raw string) time.Time {
	if raw == "" {
		return time.Now()
	}
	ms, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || ms <= 0 {
		return time.Now()
	}
	return time.UnixMilli(ms)
}

func ptrString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func compactForReply(text string, maxRunes int) string {
	text = strings.TrimSpace(text)
	if maxRunes <= 0 {
		return text
	}
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	return string(runes[:maxRunes]) + "\n\n[truncated]"
}
