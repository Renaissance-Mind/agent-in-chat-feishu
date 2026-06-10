package core

import (
	"net/url"
	"regexp"
	"sort"
	"strings"
)

var feishuPermissionScopePattern = regexp.MustCompile(`\b[a-z][a-z0-9_]*(?::[a-z0-9_.-]+)+\b`)

var feishuDeprecatedScopeAliases = map[string]string{
	"im:chat:readonly":               "im:chat:read",
	"im:message:send":                "im:message",
	"im:message.history:readonly":    "im:message",
	"im:message.revert_msg:readonly": "im:message",
	"im:message:basic":               "im:message:readonly",
	"im:resource:upload":             "im:resource",
}

// FeishuRecommendedBotScopes returns the app scopes used by the Feishu/Lark
// runtime for normal chat operation, group context, progress cards, reactions,
// attachments, and readable identity mapping.
func FeishuRecommendedBotScopes() []string {
	return []string{
		"application:bot.basic_info:read",
		"cardkit:card:write",
		"contact:contact.base:readonly",
		"im:chat.access_event.bot_p2p_chat:read",
		"im:chat.members:bot_access",
		"im:chat.members:read",
		"im:chat:read",
		"im:message",
		"im:message.group_at_msg.include_bot:readonly",
		"im:message.group_at_msg:readonly",
		"im:message.group_msg",
		"im:message.p2p_msg:readonly",
		"im:message.reactions:write_only",
		"im:message:readonly",
		"im:message:recall",
		"im:message:send_as_bot",
		"im:message:update",
		"im:resource",
	}
}

// FeishuRecommendedBotEvents returns event subscriptions the runtime consumes.
func FeishuRecommendedBotEvents() []string {
	return []string{
		"im.message.receive_v1",
		"im.message.message_read_v1",
		"im.chat.access_event.bot_p2p_chat_entered_v1",
		"card.action.trigger",
		"application.bot.menu_v6",
	}
}

func FeishuOpenPlatformBase(platformType string) string {
	if strings.EqualFold(strings.TrimSpace(platformType), "lark") {
		return "https://open.larksuite.com"
	}
	return "https://open.feishu.cn"
}

func FeishuScopeApplyURL(platformType, appID string, scopes []string) string {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return ""
	}
	scopes = normalizeFeishuScopes(scopes)
	if len(scopes) == 0 {
		scopes = FeishuRecommendedBotScopes()
	}
	scopeParam := url.QueryEscape(strings.Join(scopes, ","))
	return FeishuOpenPlatformBase(platformType) + "/page/scope-apply?clientID=" + url.QueryEscape(appID) + "&scopes=" + scopeParam
}

func FeishuPermissionAuthURL(platformType, appID string, scopes []string) string {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return ""
	}
	scopes = normalizeFeishuScopes(scopes)
	if len(scopes) == 0 {
		scopes = FeishuRecommendedBotScopes()
	}
	query := url.Values{}
	query.Set("q", strings.Join(scopes, ","))
	query.Set("op_from", "openapi")
	query.Set("token_type", "tenant")
	return FeishuOpenPlatformBase(platformType) + "/app/" + url.PathEscape(appID) + "/auth?" + query.Encode()
}

func FeishuDeveloperConsoleURL(platformType, appID string) string {
	appID = strings.TrimSpace(appID)
	base := FeishuOpenPlatformBase(platformType)
	if appID == "" {
		return base + "/app"
	}
	return base + "/app/" + url.PathEscape(appID)
}

func FeishuPermissionConsoleURL(platformType, appID string) string {
	base := FeishuDeveloperConsoleURL(platformType, appID)
	if strings.TrimSpace(appID) == "" {
		return base
	}
	return base + "/permission"
}

func FeishuEventConsoleURL(platformType, appID string) string {
	base := FeishuDeveloperConsoleURL(platformType, appID)
	if strings.TrimSpace(appID) == "" {
		return base
	}
	return base + "/event"
}

func FeishuScopesFromPermissionError(message string) []string {
	message = strings.TrimSpace(message)
	if message == "" {
		return nil
	}
	lower := strings.ToLower(message)
	candidates := feishuPermissionScopePattern.FindAllString(message, -1)
	if strings.Contains(lower, "im:message.group_msg") ||
		strings.Contains(lower, "group_msg") ||
		strings.Contains(lower, "230027") {
		candidates = append(candidates, "im:message.group_msg")
	}
	if strings.Contains(lower, "im:chat.members:read") ||
		strings.Contains(lower, "chat.members") ||
		strings.Contains(lower, "chat members") ||
		strings.Contains(lower, "99991672") {
		candidates = append(candidates, "im:chat.members:read")
	}
	return normalizeFeishuScopes(candidates)
}

func normalizeFeishuScopes(scopes []string) []string {
	if len(scopes) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(scopes))
	out := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		scope = strings.Trim(strings.TrimSpace(scope), ".,;，；。")
		if scope == "" {
			continue
		}
		if replacement, ok := feishuDeprecatedScopeAliases[scope]; ok {
			scope = replacement
		}
		if !strings.Contains(scope, ":") {
			continue
		}
		if _, ok := seen[scope]; ok {
			continue
		}
		seen[scope] = struct{}{}
		out = append(out, scope)
	}
	sort.Strings(out)
	return out
}
