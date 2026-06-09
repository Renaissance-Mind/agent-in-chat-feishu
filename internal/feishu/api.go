package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sariel/agent-in-chat-feishu/internal/contextbuilder"
	"github.com/sariel/agent-in-chat-feishu/internal/identity"
)

const maxFeishuPageSize = 50
const maxRequestAttempts = 3

type API struct {
	appID     string
	appSecret string
	baseURL   string
	client    *http.Client
	mu        sync.Mutex
	token     string
	tokenExp  time.Time
}

type BotInfo struct {
	OpenID  string
	AppName string
}

func NewAPI(appID, appSecret, baseURL string) *API {
	if baseURL == "" {
		baseURL = "https://open.feishu.cn"
	}
	return &API{
		appID:     appID,
		appSecret: appSecret,
		baseURL:   strings.TrimRight(baseURL, "/"),
		client:    &http.Client{Timeout: 20 * time.Second},
	}
}

func (a *API) AppID() string {
	return a.appID
}

func (a *API) FetchBotInfo(ctx context.Context) (BotInfo, error) {
	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Bot  struct {
			OpenID  string `json:"open_id"`
			AppName string `json:"app_name"`
		} `json:"bot"`
	}
	if err := a.get(ctx, "/open-apis/bot/v3/info", nil, &resp); err != nil {
		return BotInfo{}, err
	}
	if resp.Code != 0 {
		return BotInfo{}, fmt.Errorf("bot info code=%d msg=%s", resp.Code, resp.Msg)
	}
	return BotInfo{OpenID: resp.Bot.OpenID, AppName: resp.Bot.AppName}, nil
}

func (a *API) RefreshIdentities(ctx context.Context, chatID string, cache *identity.Cache) error {
	if cache == nil || chatID == "" {
		return nil
	}
	members, err := a.FetchMembers(ctx, chatID)
	if err != nil {
		return err
	}
	for id, name := range members {
		cache.PutUser(id, name)
	}
	bots, err := a.FetchBots(ctx, chatID)
	if err != nil {
		return err
	}
	for id, name := range bots {
		cache.PutBot(id, name)
	}
	return cache.Save()
}

func (a *API) FetchMembers(ctx context.Context, chatID string) (map[string]string, error) {
	path := fmt.Sprintf("/open-apis/im/v1/chats/%s/members", url.PathEscape(chatID))
	out := make(map[string]string)
	pageToken := ""
	for {
		params := url.Values{}
		params.Set("member_id_type", "open_id")
		params.Set("page_size", "100")
		if pageToken != "" {
			params.Set("page_token", pageToken)
		}
		var resp struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
			Data struct {
				Items []struct {
					MemberID string `json:"member_id"`
					Name     string `json:"name"`
				} `json:"items"`
				HasMore   bool   `json:"has_more"`
				PageToken string `json:"page_token"`
			} `json:"data"`
		}
		if err := a.get(ctx, path, params, &resp); err != nil {
			return nil, err
		}
		if resp.Code != 0 {
			return nil, fmt.Errorf("chat members code=%d msg=%s", resp.Code, resp.Msg)
		}
		for _, item := range resp.Data.Items {
			if strings.TrimSpace(item.MemberID) != "" && strings.TrimSpace(item.Name) != "" {
				out[item.MemberID] = item.Name
			}
		}
		if !resp.Data.HasMore || resp.Data.PageToken == "" {
			break
		}
		pageToken = resp.Data.PageToken
	}
	return out, nil
}

func (a *API) FetchBots(ctx context.Context, chatID string) (map[string]string, error) {
	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Items []struct {
				BotID   string `json:"bot_id"`
				BotName string `json:"bot_name"`
			} `json:"items"`
		} `json:"data"`
	}
	path := fmt.Sprintf("/open-apis/im/v1/chats/%s/members/bots", url.PathEscape(chatID))
	if err := a.get(ctx, path, nil, &resp); err != nil {
		return nil, err
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("chat bots code=%d msg=%s", resp.Code, resp.Msg)
	}
	out := make(map[string]string)
	for _, item := range resp.Data.Items {
		if strings.TrimSpace(item.BotID) != "" && strings.TrimSpace(item.BotName) != "" {
			out[item.BotID] = item.BotName
		}
	}
	return out, nil
}

func (a *API) FetchHistory(ctx context.Context, chatID string, maxMessages int, maxAge time.Duration) ([]contextbuilder.Entry, error) {
	if maxMessages < 1 {
		maxMessages = maxFeishuPageSize
	}
	now := time.Now()
	since := now.Add(-24 * time.Hour)
	if maxAge > 0 {
		since = now.Add(-maxAge)
	}
	var entries []contextbuilder.Entry
	pageToken := ""
	for len(entries) < maxMessages {
		pageSize := maxMessages - len(entries)
		if pageSize > maxFeishuPageSize {
			pageSize = maxFeishuPageSize
		}
		params := url.Values{}
		params.Set("container_id_type", "chat")
		params.Set("container_id", chatID)
		params.Set("start_time", strconv.FormatInt(since.Unix(), 10))
		params.Set("end_time", strconv.FormatInt(now.Add(5*time.Second).Unix(), 10))
		params.Set("sort_type", "ByCreateTimeDesc")
		params.Set("page_size", strconv.Itoa(pageSize))
		if pageToken != "" {
			params.Set("page_token", pageToken)
		}
		var resp messageListResponse
		if err := a.get(ctx, "/open-apis/im/v1/messages", params, &resp); err != nil {
			return nil, err
		}
		if resp.Code != 0 {
			return nil, fmt.Errorf("messages list code=%d msg=%s", resp.Code, resp.Msg)
		}
		for _, item := range resp.Data.Items {
			if entry, ok := item.entry(); ok {
				entries = append(entries, entry)
			}
		}
		if !resp.Data.HasMore || resp.Data.PageToken == "" {
			break
		}
		pageToken = resp.Data.PageToken
	}
	return entries, nil
}

func (a *API) ReplyText(ctx context.Context, messageID, text string) error {
	content, _ := json.Marshal(map[string]string{"text": text})
	body := map[string]string{
		"msg_type": "text",
		"content":  string(content),
	}
	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	path := fmt.Sprintf("/open-apis/im/v1/messages/%s/reply", url.PathEscape(messageID))
	if err := a.post(ctx, path, body, &resp); err != nil {
		return err
	}
	if resp.Code != 0 {
		return fmt.Errorf("reply code=%d msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

func (a *API) CreateReaction(ctx context.Context, messageID, emojiType string) (string, error) {
	messageID = strings.TrimSpace(messageID)
	emojiType = strings.TrimSpace(emojiType)
	if messageID == "" || emojiType == "" {
		return "", nil
	}
	body := map[string]any{
		"reaction_type": map[string]string{
			"emoji_type": emojiType,
		},
	}
	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			ReactionID string `json:"reaction_id"`
		} `json:"data"`
	}
	path := fmt.Sprintf("/open-apis/im/v1/messages/%s/reactions", url.PathEscape(messageID))
	if err := a.post(ctx, path, body, &resp); err != nil {
		return "", err
	}
	if resp.Code != 0 {
		return "", fmt.Errorf("create reaction code=%d msg=%s", resp.Code, resp.Msg)
	}
	return strings.TrimSpace(resp.Data.ReactionID), nil
}

func (a *API) DeleteReaction(ctx context.Context, messageID, reactionID string) error {
	messageID = strings.TrimSpace(messageID)
	reactionID = strings.TrimSpace(reactionID)
	if messageID == "" || reactionID == "" {
		return nil
	}
	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	path := fmt.Sprintf("/open-apis/im/v1/messages/%s/reactions/%s", url.PathEscape(messageID), url.PathEscape(reactionID))
	if err := a.delete(ctx, path, &resp); err != nil {
		return err
	}
	if resp.Code != 0 {
		return fmt.Errorf("delete reaction code=%d msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

func (a *API) tenantToken(ctx context.Context) (string, error) {
	a.mu.Lock()
	if a.token != "" && time.Now().Before(a.tokenExp) {
		token := a.token
		a.mu.Unlock()
		return token, nil
	}
	a.mu.Unlock()

	body := map[string]string{"app_id": a.appID, "app_secret": a.appSecret}
	var resp struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
		Expire            int    `json:"expire"`
	}
	if err := a.postNoAuth(ctx, "/open-apis/auth/v3/tenant_access_token/internal", body, &resp); err != nil {
		return "", err
	}
	if resp.Code != 0 || resp.TenantAccessToken == "" {
		return "", fmt.Errorf("tenant token code=%d msg=%s", resp.Code, resp.Msg)
	}

	a.mu.Lock()
	a.token = resp.TenantAccessToken
	expire := resp.Expire
	if expire <= 0 {
		expire = 3600
	}
	a.tokenExp = time.Now().Add(time.Duration(expire-60) * time.Second)
	a.mu.Unlock()
	return resp.TenantAccessToken, nil
}

func (a *API) get(ctx context.Context, path string, params url.Values, out any) error {
	return a.authedRequest(ctx, http.MethodGet, path, params, nil, out)
}

func (a *API) post(ctx context.Context, path string, body any, out any) error {
	return a.authedRequest(ctx, http.MethodPost, path, nil, body, out)
}

func (a *API) delete(ctx context.Context, path string, out any) error {
	return a.authedRequest(ctx, http.MethodDelete, path, nil, nil, out)
}

func (a *API) authedRequest(ctx context.Context, method, path string, params url.Values, body any, out any) error {
	var lastErr error
	for attempt := 0; attempt < maxRequestAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		token, err := a.tenantToken(ctx)
		if err != nil {
			return err
		}
		req, err := a.newRequest(ctx, method, path, params, body)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		if err := a.do(req, out); err != nil {
			lastErr = err
			if isTransientError(err) && attempt < maxRequestAttempts-1 {
				time.Sleep(requestBackoff(attempt))
				continue
			}
			return err
		}
		if code, ok := responseCode(out); ok && isTenantTokenInvalidCode(code) && attempt < maxRequestAttempts-1 {
			a.clearToken()
			continue
		}
		return nil
	}
	return lastErr
}

func (a *API) newRequest(ctx context.Context, method, path string, params url.Values, body any) (*http.Request, error) {
	u := a.baseURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	var reader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

func (a *API) postNoAuth(ctx context.Context, path string, body any, out any) error {
	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return a.do(req, out)
}

func (a *API) do(req *http.Request, out any) error {
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return statusError{method: req.Method, path: req.URL.Path, status: resp.StatusCode}
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (a *API) clearToken() {
	a.mu.Lock()
	a.token = ""
	a.tokenExp = time.Time{}
	a.mu.Unlock()
}

func responseCode(out any) (int, bool) {
	if out == nil {
		return 0, false
	}
	value := reflect.ValueOf(out)
	if value.Kind() != reflect.Pointer || value.IsNil() {
		return 0, false
	}
	value = value.Elem()
	if value.Kind() != reflect.Struct {
		return 0, false
	}
	field := value.FieldByName("Code")
	if !field.IsValid() || field.Kind() != reflect.Int {
		return 0, false
	}
	return int(field.Int()), true
}

func isTenantTokenInvalidCode(code int) bool {
	return code == 99991663
}

type statusError struct {
	method string
	path   string
	status int
}

func (e statusError) Error() string {
	return fmt.Sprintf("%s %s: http %d", e.method, e.path, e.status)
}

func isTransientError(err error) bool {
	if err == nil {
		return false
	}
	var status statusError
	if errors.As(err, &status) {
		return status.status == http.StatusTooManyRequests || status.status >= 500
	}
	if errors.Is(err, io.EOF) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func requestBackoff(attempt int) time.Duration {
	return time.Duration(attempt+1) * 200 * time.Millisecond
}

type messageListResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Items     []messageItem `json:"items"`
		HasMore   bool          `json:"has_more"`
		PageToken string        `json:"page_token"`
	} `json:"data"`
}

type messageItem struct {
	MessageID  string `json:"message_id"`
	MsgType    string `json:"msg_type"`
	CreateTime string `json:"create_time"`
	Sender     struct {
		ID         string `json:"id"`
		SenderType string `json:"sender_type"`
	} `json:"sender"`
	Body struct {
		Content string `json:"content"`
	} `json:"body"`
	Mentions []mentionItem `json:"mentions"`
	Deleted  bool          `json:"deleted"`
}

type mentionItem struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

func (m messageItem) entry() (contextbuilder.Entry, bool) {
	if m.Deleted || m.MsgType == "interactive" {
		return contextbuilder.Entry{}, false
	}
	text := messageText(m.MsgType, m.Body.Content, m.Mentions)
	if strings.TrimSpace(text) == "" {
		return contextbuilder.Entry{}, false
	}
	createdAt := time.Now()
	if raw := strings.TrimSpace(m.CreateTime); raw != "" {
		if ms, err := strconv.ParseInt(raw, 10, 64); err == nil && ms > 0 {
			createdAt = time.UnixMilli(ms)
		}
	}
	return contextbuilder.Entry{
		MessageID:  m.MessageID,
		SenderID:   m.Sender.ID,
		SenderType: m.Sender.SenderType,
		MsgType:    m.MsgType,
		CreatedAt:  createdAt,
		Content:    text,
	}, true
}

func messageText(msgType, content string, mentions []mentionItem) string {
	switch msgType {
	case "text":
		var body struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal([]byte(content), &body); err != nil {
			return ""
		}
		return replaceMentions(body.Text, mentions)
	case "post":
		return extractPostPlainText(content)
	case "image":
		return "[image]"
	case "audio":
		return "[audio]"
	case "file":
		var body struct {
			FileName string `json:"file_name"`
		}
		if err := json.Unmarshal([]byte(content), &body); err == nil && strings.TrimSpace(body.FileName) != "" {
			return "[file: " + strings.TrimSpace(body.FileName) + "]"
		}
		return "[file]"
	case "merge_forward":
		return "[forwarded messages]"
	default:
		return ""
	}
}

func replaceMentions(text string, mentions []mentionItem) string {
	for _, mention := range mentions {
		if mention.Key != "" && mention.Name != "" {
			text = strings.ReplaceAll(text, mention.Key, "@"+mention.Name)
		}
	}
	return strings.TrimSpace(text)
}

func extractPostPlainText(content string) string {
	var post struct {
		Title   string `json:"title"`
		Content [][]struct {
			Tag  string `json:"tag"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal([]byte(content), &post); err != nil || len(post.Content) == 0 {
		var wrapped map[string]json.RawMessage
		if err := json.Unmarshal([]byte(content), &wrapped); err != nil {
			return ""
		}
		for _, raw := range wrapped {
			if err := json.Unmarshal(raw, &post); err == nil && len(post.Content) > 0 {
				break
			}
		}
	}
	var parts []string
	if strings.TrimSpace(post.Title) != "" {
		parts = append(parts, strings.TrimSpace(post.Title))
	}
	for _, line := range post.Content {
		var words []string
		for _, elem := range line {
			if (elem.Tag == "text" || elem.Tag == "a") && strings.TrimSpace(elem.Text) != "" {
				words = append(words, strings.TrimSpace(elem.Text))
			}
		}
		if len(words) > 0 {
			parts = append(parts, strings.Join(words, ""))
		}
	}
	return strings.Join(parts, "\n")
}
