package core

import (
	"net/url"
	"regexp"
	"sort"
	"strings"
)

var feishuPermissionScopePattern = regexp.MustCompile(`\b[a-z][a-z0-9_]*(?::[a-z0-9_.-]+)+\b`)

// FeishuRecommendedBotScopes returns the app scopes used by the Feishu/Lark
// runtime for normal chat operation, group context, progress cards, reactions,
// attachments, and readable identity mapping.
func FeishuRecommendedBotScopes() []string {
	return []string{
		"im:message",
		"im:message:readonly",
		"im:message:send_as_bot",
		"im:message:update",
		"im:message:recall",
		"im:message.group_msg",
		"im:message.reactions:write_only",
		"im:resource",
		"im:resource:upload",
		"im:chat:readonly",
		"im:chat.members:read",
		"contact:user.base:readonly",
	}
}

// FeishuRecommendedBotEvents returns event subscriptions the runtime consumes.
func FeishuRecommendedBotEvents() []string {
	return []string{
		"im.message.receive_v1",
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
	scopeParam := strings.ReplaceAll(url.QueryEscape(strings.Join(scopes, " ")), "+", "%20")
	return FeishuOpenPlatformBase(platformType) + "/page/scope-apply?clientID=" + url.QueryEscape(appID) + "&scopes=" + scopeParam
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
