package core

import (
	"strings"
	"testing"
)

func TestFeishuScopeApplyURL_DefaultScopes(t *testing.T) {
	got := FeishuScopeApplyURL("feishu", "cli_test", nil)
	if !strings.HasPrefix(got, "https://open.feishu.cn/page/scope-apply?") {
		t.Fatalf("url = %q, want feishu scope apply URL", got)
	}
	for _, want := range []string{
		"clientID=cli_test",
		"im%3Amessage",
		"im%3Amessage%3Areadonly",
		"im%3Amessage%3Asend_as_bot",
		"im%3Amessage.group_at_msg%3Areadonly",
		"im%3Amessage.group_msg",
		"im%3Amessage.p2p_msg%3Areadonly",
		"im%3Amessage.reactions%3Awrite_only",
		"im%3Aresource",
		"im%3Achat.access_event.bot_p2p_chat%3Aread",
		"im%3Achat%3Aread",
		"im%3Achat.members%3Abot_access",
		"im%3Achat.members%3Aread",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("url = %q, missing %q", got, want)
		}
	}
	for _, deprecated := range []string{
		"im%3Achat%3Areadonly",
		"im%3Amessage%3Aupdate",
		"im%3Amessage%3Arecall",
		"im%3Aresource%3Aupload",
	} {
		if strings.Contains(got, deprecated) {
			t.Fatalf("url = %q, contains deprecated scope %q", got, deprecated)
		}
	}
}

func TestFeishuPermissionAuthURL_DefaultScopes(t *testing.T) {
	got := FeishuPermissionAuthURL("feishu", "cli_test", nil)
	if !strings.HasPrefix(got, "https://open.feishu.cn/app/cli_test/auth?") {
		t.Fatalf("url = %q, want feishu app auth URL", got)
	}
	for _, want := range []string{
		"q=",
		"im%3Amessage",
		"im%3Amessage.group_at_msg%3Areadonly",
		"im%3Amessage.p2p_msg%3Areadonly",
		"im%3Achat.access_event.bot_p2p_chat%3Aread",
		"im%3Achat%3Aread",
		"op_from=openapi",
		"token_type=tenant",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("url = %q, missing %q", got, want)
		}
	}
}

func TestFeishuScopeApplyURL_LarkCustomScopes(t *testing.T) {
	got := FeishuScopeApplyURL("lark", "cli test", []string{"im:message.group_msg", "im:message"})
	if !strings.HasPrefix(got, "https://open.larksuite.com/page/scope-apply?") {
		t.Fatalf("url = %q, want lark scope apply URL", got)
	}
	if !strings.Contains(got, "clientID=cli+test") {
		t.Fatalf("url = %q, missing encoded clientID", got)
	}
	if !strings.Contains(got, "scopes=im%3Amessage%20im%3Amessage.group_msg") {
		t.Fatalf("url = %q, missing space-separated scopes", got)
	}
}

func TestFeishuRecommendedBotEvents(t *testing.T) {
	got := strings.Join(FeishuRecommendedBotEvents(), ",")
	for _, want := range []string{
		"im.message.receive_v1",
		"im.message.message_read_v1",
		"im.chat.access_event.bot_p2p_chat_entered_v1",
		"card.action.trigger",
		"application.bot.menu_v6",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("events = %q, missing %q", got, want)
		}
	}
}

func TestFeishuScopesFromPermissionError(t *testing.T) {
	got := FeishuScopesFromPermissionError("list messages code=230027 msg=need scope: im:message.group_msg")
	if len(got) != 1 || got[0] != "im:message.group_msg" {
		t.Fatalf("scopes = %#v, want im:message.group_msg", got)
	}

	got = FeishuScopesFromPermissionError("permission_violations: im:chat:readonly, im:chat.members:read")
	if strings.Join(got, ",") != "im:chat.members:read,im:chat:read" {
		t.Fatalf("scopes = %#v, want canonical chat scopes", got)
	}

	got = FeishuScopesFromPermissionError("need im:message:update and im:resource:upload")
	if strings.Join(got, ",") != "im:message,im:resource" {
		t.Fatalf("scopes = %#v, want canonical replacement scopes", got)
	}
}
