# Usage Guide

[English README](../README.md) | [中文 README](../README.zh-CN.md)

This guide covers the retained cc-connect runtime features in `agent-in-chat-feishu`. The chat platform surface is Feishu/Lark only; the agent, session, provider, command, daemon, web, cron, relay, webhook, and management features remain.

## Feishu Setup

```bash
agentchat feishu setup --project my-project
agentchat feishu setup --project my-project --app cli_xxx:sec_xxx
```

Then start:

```bash
agentchat
```

See [Feishu setup guide](feishu.md) for permissions, events, and group history behavior.

## Daemon Mode

Install the background service after `setup` has created the config directory:

```bash
agentchat daemon install --work-dir ~/.agentchat
```

The service captures the installer process `PATH` and adds common macOS/Linux CLI
directories. For custom Node or agent managers, pass an explicit value:

```bash
agentchat daemon install --work-dir ~/.agentchat --env-path "$PATH"
```

## Supported Agents

Set the agent type in `config.toml`:

```toml
[projects.agent]
type = "codex"
```

Supported types:

- `codex`
- `claudecode`
- `opencode`
- `gemini`
- `kimi`
- `qoder`
- `iflow`
- `cursor`
- `acp`
- `pi`

## Chat Commands

Send these in Feishu:

| Command | Purpose |
|---|---|
| `/help` | Show commands |
| `/status` | Show project/session status |
| `/whoami` | Show your Feishu identity |
| `/new [name]` | Start a new session |
| `/list` | List sessions |
| `/switch <id>` | Switch active session |
| `/history [n]` | Show recent session history |
| `/stop` | Stop current agent execution |
| `/model` | List or switch models |
| `/provider` | Manage providers |
| `/mode` | Show or switch permission mode |
| `/dir` or `/cd` | Show or switch work directory |
| `/usage` | Show usage/quota when supported |
| `/cron` | Manage scheduled jobs |
| `/web` | Open web UI information |
| `/commands` | List custom commands |
| `/alias` | Manage command aliases |
| `/delete` | Delete sessions |
| `/workspace` | Manage workspace bindings |

## Permission Modes

Modes depend on the agent backend. Common values include:

```text
default
auto
auto-edit
plan
yolo
```

Use:

```text
/mode
/mode yolo
/mode default
```

## Providers And Models

Global provider example:

```toml
[[providers]]
name = "openai"
api_key = "${OPENAI_API_KEY}"
base_url = "https://api.openai.com/v1"
model = "gpt-5.5"
agent_types = ["codex"]

[[projects]]
name = "my-project"

[projects.agent]
type = "codex"
provider_refs = ["openai"]
```

CLI:

```bash
agentchat provider list --project my-project
agentchat provider add --project my-project --name relay --api-key sk_xxx --base-url https://example.com/v1
agentchat provider remove --project my-project --name relay
```

Chat:

```text
/provider
/provider list
/provider switch openai
/model
/model switch codex
```

## Group History Context

Enable:

```toml
[projects.platforms.options]
group_context_buffer = true
context_buffer_max_messages = 100
context_buffer_max_age_mins = 0
```

When the bot is mentioned in a Feishu group, recent group history is fetched, filtered, cached, and injected as background context. Interactive progress cards are skipped by default, while readable final reply cards are kept.

## Identity Cache

Feishu user/app/chat/member names are persisted under `~/.agentchat`. This keeps prompts readable and avoids repeatedly adding long IDs to agent input.

## Attachments

Send files or images back into the active Feishu session:

```bash
agentchat send --session <session-id> --message "Done"
agentchat send --session <session-id> --image /path/to/screenshot.png
agentchat send --session <session-id> --file /path/to/report.pdf
```

## Cron And Heartbeat

Cron jobs can trigger prompts or shell commands:

```toml
[[projects]]
name = "my-project"

[projects.heartbeat]
enabled = true
interval_mins = 30
only_when_idle = true
session_key = "feishu:oc_xxx"
prompt = "Check whether anything needs attention."
```

Use chat or CLI:

```text
/cron
```

```bash
agentchat cron list
```

## Webhook

Enable external HTTP triggers:

```toml
[webhook]
enabled = true
port = 9111
token = "${AGENTCHAT_WEBHOOK_TOKEN}"
path = "/hook"
```

Example:

```bash
curl -X POST 'http://localhost:9111/hook/prompt' \
  -H 'Authorization: Bearer your-token' \
  -H 'Content-Type: application/json' \
  -d '{"project":"my-project","session_key":"feishu:oc_xxx","prompt":"Review the latest commit"}'
```

## Daemon And Web UI

```bash
agentchat daemon start
agentchat daemon status
agentchat daemon logs
agentchat web
```

## Management API

Enable:

```toml
[management]
enabled = true
port = 9820
token = "${AGENTCHAT_MANAGEMENT_TOKEN}"
```

See [Management API](management-api.md).

## Bridge

The bridge server is retained for external runtime integrations and tooling. It is not a bundled chat adapter for another chat platform.

```toml
[bridge]
enabled = true
port = 9810
token = "${AGENTCHAT_BRIDGE_TOKEN}"
path = "/bridge/ws"
```

See [Bridge protocol](bridge-protocol.md).

## Multi-Workspace

A project can route sessions to different work directories:

```toml
[[projects]]
name = "workspace-router"
mode = "multi-workspace"
base_dir = "/Users/alex/work"
```

Use `/workspace`, `/dir`, or `/cd` in Feishu to inspect or switch workspaces.
