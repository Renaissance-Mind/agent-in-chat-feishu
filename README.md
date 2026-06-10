# agent-in-chat-feishu

[English](README.md) | [中文](README.zh-CN.md)

> ⚠️ **Personal-first defaults.** This project is designed primarily for personal or trusted small-team use. The default Codex agent mode is intentionally permissive so it can read local files, call local tools, and act like the agent you would run from your own terminal. For shared, production, or untrusted groups, review `mode`, `admin_from`, chat allowlists, and disabled commands before running it.

<p align="center">
  <img src="docs/images/banner.svg" alt="Agent in Chat Feishu" width="720">
</p>

Put Codex, Claude Code, and other coding agents into the Feishu chat loop your team already uses.

[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)](go.mod)
[![Platform](https://img.shields.io/badge/chat-Feishu%20%2F%20Lark-00BFA5)](docs/feishu.md)

`agent-in-chat-feishu` is a Feishu/Lark-only distribution derived from cc-connect. It keeps the mature agent runtime, sessions, slash commands, providers, progress cards, attachments, cron jobs, relay, management API, and multi-agent support, while removing the concrete adapters for other chat apps and the unused browser admin UI.

The point is not to make your group chat feel like a bot room. The agent joins the ordinary conversation loop: people talk normally, mention the bot when work should happen, and Codex receives the missing group context before it starts.

## Features

- 💬 **Feishu/Lark first** — bot setup, message receive, reply, cards, reactions, attachments, group history context.
- 🧠 **Agent runtime preserved** — `/model`, `/stop`, `/new`, `/list`, `/switch`, `/history`, `/provider`, `/cron`, `/dir`, `/mode`, `/usage`, `/commands`, `/alias`, `/delete`, `/bind`, `/workspace`.
- 🤝 **Many agents** — Codex, Claude Code, OpenCode, Gemini, Kimi, Qoder, iFlow, Cursor, ACP, Pi.
- 🧩 **Real chat context** — on mention, recent Feishu group history can be fetched, filtered, cached, and injected as background context.
- 🪪 **Readable identities** — Feishu user/app/chat names are cached on disk under `~/.agentchat` so Codex sees names instead of long IDs whenever possible.
- 📌 **Less noise by default** — progress cards are ignored when building group context; readable final reply cards still count.
- 🛠️ **Operational surface kept** — daemon mode, management API, webhook, cron/heartbeat, relay, session store, provider switching, and attachment send-back.

## How It Feels

Feishu group:

```text
Mina: The deploy failed again after the config change.
Alex: I think the env file is not loaded in the worker.
River: The log says "missing OPENAI_API_KEY", but local dev is fine.
Alex: @agentchat check the recent config and tell us what to fix.
```

What Codex receives:

```text
[Feishu group context]
Mina: The deploy failed again after the config change.
Alex: I think the env file is not loaded in the worker.
River: The log says "missing OPENAI_API_KEY", but local dev is fine.
[/Feishu group context]

Alex: check the recent config and tell us what to fix.
```

Progress cards from this or other bots are skipped. Sender names come from the local identity cache when available; new IDs trigger a Feishu lookup and then get persisted.

## Installation

```bash
npm install -g @renaissancemind/agent-in-chat-feishu
agentchat --help
```

Build from source:

```bash
git clone https://github.com/Renaissance-Mind/agent-in-chat-feishu.git
cd agent-in-chat-feishu
make build
./agentchat --help
```

## Quick Start

Create or connect a Feishu/Lark bot and write the project config:

```bash
agentchat feishu setup
```

Connect an existing app:

```bash
agentchat feishu setup --app cli_xxx:sec_xxx
```

Then run the bridge:

```bash
agentchat
```

`setup` is the default path. Without `--project`, it creates the local bot profile `feishu` and sets its initial work directory to `~/.agentchat/feishu/` next to the config file. That directory is only the starting workspace; you can switch to the real code repository later from chat with `/dir` or `/workspace`. The command writes the platform config and prints direct permission/event links for the app. QR onboarding usually creates the bot app with core capabilities; when binding an existing app, open the printed permission auth link, verify long-connection events, then publish a new app version if Feishu asks for one. You can reprint the links later with `agentchat feishu permissions`, or request tenant approval through the official API with `agentchat feishu permissions --apply`.

New projects default to chat binding. If `admin_from` is set, the first valid trigger from an admin automatically binds that group or DM and persists its `chat_id`; without an admin match, the bot replies with the `chat_id` to add to `allow_group_chats` or `allow_private_chats`.

For background service mode:

```bash
agentchat daemon install --work-dir ~/.agentchat
```

Daemon install captures the current `PATH`, matching cc-connect behavior. If you
install from a non-interactive shell or use a custom path manager for the agent
CLI, Node.js, or `lark-cli`, pass the service PATH explicitly:

```bash
agentchat daemon install --work-dir ~/.agentchat --env-path "$PATH"
```

## Configuration

Minimal config shape:

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
admin_from = ""
show_context_indicator = false

[projects.agent]
type = "codex"

[projects.agent.options]
work_dir = "/absolute/path/to/my-project"
mode = "yolo"
reasoning_effort = "medium"
model = "gpt-5.5"

[[projects.platforms]]
type = "feishu"

[projects.platforms.options]
app_id = "${FEISHU_APP_ID}"
app_secret = "${FEISHU_APP_SECRET}"
allow_private_chats = ""
allow_group_chats = ""
auto_bind_chats = true
group_context_buffer = true
context_buffer_max_messages = 100
context_buffer_max_age_mins = 0
share_session_in_channel = true
progress_style = "card"
reaction_emoji = "OnIt"
```

The default config and runtime data directory is `~/.agentchat`. See [config.example.toml](config.example.toml) for a fuller Feishu-only example.

## Feishu Permissions

For a full bot that behaves like the current runtime, enable robot capability, long-connection event delivery, and these permissions/events:

| Capability | Feishu permission or event |
|---|---|
| Receive group mentions | `im.message.receive_v1` with `im:message.group_at_msg:readonly` |
| Receive direct messages | `im.message.receive_v1` with `im:message.p2p_msg:readonly` |
| Detect direct-chat entry | `im.chat.access_event.bot_p2p_chat_entered_v1` with `im:chat.access_event.bot_p2p_chat:read` |
| Fetch recent group history and quoted messages | `im:message`, `im:message:readonly`, `im:message.group_msg` |
| Send and reply | `im:message` or `im:message:send_as_bot` |
| Update progress/card messages | `im:message` |
| Recall transient preview messages | `im:message` |
| Add/remove reactions | `im:message.reactions:write_only` |
| Upload/download image/file attachments | `im:resource` |
| Resolve names from group members | `im:chat.members:read` or broader group info scopes |
| Resolve user names | `contact:user.base:readonly` |
| Use interactive cards | card callback event `card.action.trigger` |
| Use bot custom menu callbacks | bot menu event `application.bot.menu_v6` |

The setup command prints a Feishu/Lark permission auth URL with the recommended runtime scopes preselected: `im:message`, `im:message:readonly`, `im:message:send_as_bot`, `im:message.group_at_msg:readonly`, `im:message.group_msg`, `im:message.p2p_msg:readonly`, `im:message.reactions:write_only`, `im:resource`, `im:chat.access_event.bot_p2p_chat:read`, `im:chat:read`, `im:chat.members:bot_access`, `im:chat.members:read`, and `contact:user.base:readonly`. If your terminal config contains `app_secret`, `agentchat feishu permissions --apply` can request tenant permission approval through Feishu's official `application/v6/scopes/apply` API.

Official references: [send messages](https://open.feishu.cn/document/server-docs/im-v1/message/create), [reply](https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/reference/im-v1/message/reply), [receive event](https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/reference/im-v1/message/events/receive), [history](https://open.feishu.cn/document/server-docs/im-v1/message/list), [reactions](https://open.feishu.cn/document/server-docs/im-v1/message-reaction/create?lang=zh-CN), [group members](https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/reference/im-v1/chat-members/get), [image upload](https://open.feishu.cn/document/server-docs/im-v1/image/create).

## Commands

Examples you can send in Feishu:

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

The CLI is `agentchat`:

```bash
agentchat sessions list
agentchat send --session <session-id> --message "ship a short status update"
agentchat daemon start
```

## Documentation

- [Feishu setup guide](docs/feishu.md)
- [Install guide](INSTALL.md)
- [Usage guide](docs/usage.md)
- [Management API](docs/management-api.md)
- [Bridge protocol](docs/bridge-protocol.md)

## Contributing

Contributions are welcome. Keep the distribution Feishu/Lark-only unless the project direction changes, and keep core agent/runtime behavior compatible with cc-connect where possible.

## License

[MIT](LICENSE)

## Acknowledgements

This project is derived from and deeply indebted to [cc-connect](https://github.com/chenhg5/cc-connect). Thanks to the cc-connect authors and contributors for the original agent runtime, chat command model, and Feishu platform foundation.
