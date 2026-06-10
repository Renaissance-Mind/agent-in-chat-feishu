# Installation

[English](README.md) | [中文](README.zh-CN.md)

`agent-in-chat-feishu` installs the `agentchat` CLI. The package is Feishu/Lark-only, but keeps the cc-connect agent runtime and chat command surface.

## 1. Install

```bash
npm install -g @renaissancemind/agent-in-chat-feishu
agentchat --help
```

From source:

```bash
git clone https://github.com/Renaissance-Mind/agent-in-chat-feishu.git
cd agent-in-chat-feishu
make build
./agentchat --help
```

## 2. Prepare An Agent

Install at least one supported local agent:

- Codex
- Claude Code
- OpenCode
- Gemini
- Kimi
- Qoder
- iFlow
- Cursor
- ACP-compatible agents
- Pi

Example for a Codex project:

```toml
language = "zh"
idle_timeout_mins = 30

[display]
tool_messages = false

[stream_preview]
enabled = true
interval_ms = 1000
min_delta_chars = 10
max_chars = 4000

[[projects]]
name = "my-project"
show_context_indicator = false

[projects.agent]
type = "codex"

[projects.agent.options]
work_dir = "/absolute/path/to/my-project"
mode = "yolo"
reasoning_effort = "medium"
model = "gpt-5.5"
```

## 3. Create Or Connect A Feishu Bot

Recommended:

```bash
agentchat feishu setup
```

Connect an existing Feishu/Lark app:

```bash
agentchat feishu setup --app cli_xxx:sec_xxx
```

The `setup` command is the default path. Without `--project`, it creates a local bot profile named `feishu` and sets its initial work directory to `~/.agentchat/feishu/` next to the config file. That directory is only the starting workspace; users can switch to the real code repository later from chat with `/dir` or `/workspace`. The command creates the project/platform config if needed, writes credentials into `config.toml`, and prints direct permission/event links for the app.

For QR onboarding, Feishu usually provisions the bot app and core capabilities during the registration flow. For an existing app, run `setup --app ...`, open the printed scope-apply link to confirm the preselected scopes, verify long-connection event delivery, and publish a new version if Feishu asks for one.

New Feishu projects default to chat binding, not allow-all. If `admin_from` is set, the first valid trigger from that admin auto-binds the group or DM and persists the `chat_id`. Non-admin triggers receive the `chat_id` so it can be added manually.

Reprint the direct permission/event links later:

```bash
agentchat feishu permissions
```

## 4. Verify Feishu Capabilities

Enable robot capability and long-connection event delivery.

For full behavior, the app should be able to:

- receive direct messages and group mentions via `im.message.receive_v1`
- handle interactive card callbacks via `card.action.trigger`
- handle bot custom menu callbacks via `application.bot.menu_v6` when using event-based menu items
- fetch recent group history and quoted messages via `im:message`, `im:message:readonly`, and `im:message.group_msg`
- send and reply to messages via `im:message` or `im:message:send_as_bot`
- update interactive/progress cards via `im:message:update`
- recall transient preview messages via `im:message:recall`
- add/remove reactions via `im:message.reactions:write_only`
- upload/download images/files via `im:resource` and `im:resource:upload`
- read group member names for identity mapping via `im:chat.members:read`

Useful official docs:

- [Send messages](https://open.feishu.cn/document/server-docs/im-v1/message/create)
- [Reply to messages](https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/reference/im-v1/message/reply)
- [Receive message event](https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/reference/im-v1/message/events/receive)
- [Get conversation history](https://open.feishu.cn/document/server-docs/im-v1/message/list)
- [Add reactions](https://open.feishu.cn/document/server-docs/im-v1/message-reaction/create?lang=zh-CN)
- [Get group members](https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/reference/im-v1/chat-members/get)
- [Upload images](https://open.feishu.cn/document/server-docs/im-v1/image/create)

## 5. Start

```bash
agentchat
```

Daemon mode:

```bash
agentchat daemon install --work-dir ~/.agentchat
agentchat daemon status
agentchat daemon logs
```

Daemon install captures the current `PATH`, matching cc-connect behavior. If you
install from a non-interactive shell or your agent CLI, Node.js, or `lark-cli`
lives in a custom path manager, pass it explicitly:

```bash
agentchat daemon install --work-dir ~/.agentchat --env-path "$PATH"
```

## 6. Try It In Feishu

Mention the bot in a group:

```text
@agentchat summarize the latest discussion and suggest the next step
```

Useful chat commands:

```text
/help
/model
/stop
/new
/history
/provider
/cron
/mode
/usage
```

## 7. Runtime Data

By default, config, sessions, Feishu identity cache, and runtime state live under:

```text
~/.agentchat
```

The Feishu identity cache lets prompts use names instead of long user/app IDs where possible.
