# Installation

[English](README.md) | [中文](README.zh-CN.md)

`agent-in-chat-feishu` installs the `agentchat` CLI. The package is Feishu/Lark-only, but keeps the cc-connect agent runtime and chat command surface.

## 1. Install

```bash
npm install -g agent-in-chat-feishu
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
mode = "full-auto"
reasoning_effort = "medium"
model = "gpt-5.5"
```

## 3. Create Or Connect A Feishu Bot

Recommended:

```bash
agentchat feishu setup --project my-project
```

Connect an existing Feishu/Lark app:

```bash
agentchat feishu setup --project my-project --app cli_xxx:sec_xxx
```

The `setup` command is the default path. It creates the project/platform config if needed and writes credentials into `config.toml`.

For QR onboarding, Feishu usually provisions the bot app, core permissions, and event subscription during the registration flow. For an existing app, run `setup --app ...`, then verify the app in the developer console.

## 4. Verify Feishu Capabilities

Enable robot capability and long-connection event delivery.

For full behavior, the app should be able to:

- receive direct messages and group mentions via `im.message.receive_v1`
- fetch recent group history for context
- send and reply to messages
- send and update interactive cards
- add/remove reactions
- upload images/files
- read group member names for identity mapping

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
agentchat daemon start
agentchat daemon status
agentchat daemon logs
```

Web UI:

```bash
agentchat web
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
/web
```

## 7. Runtime Data

By default, config, sessions, Feishu identity cache, and runtime state live under:

```text
~/.agentchat
```

The Feishu identity cache lets prompts use names instead of long user/app IDs where possible.
