# Repository Guide

`agent-in-chat-feishu` is a Feishu/Lark-only distribution of cc-connect with the core agent runtime preserved.

## Project Boundary

Keep:

- Feishu/Lark platform support in `platform/feishu`
- all agent backends in `agent/`
- core chat/session/runtime behavior in `core/`
- CLI command `agentchat` in `cmd/agentchat`
- daemon, cron, relay, webhook, management API, bridge

Do not add bundled adapters for other chat platforms unless the project direction changes explicitly.

## Important Paths

- `cmd/agentchat/` — CLI entrypoint and subcommands
- `platform/feishu/` — Feishu/Lark runtime, setup-facing behavior, identity cache, group history
- `core/` — engine, sessions, commands, providers, cron, relay, bridge, management
- `agent/` — Codex, Claude Code, OpenCode, Gemini, Kimi, Qoder, iFlow, Cursor, ACP, Pi
- `config/` — config model, setup helpers, config patching
- `docs/` — public documentation
- `npm/` — npm wrapper that installs the release binary

## Development

Use Go 1.25+.

Common checks:

```bash
go test ./cmd/agentchat ./platform/feishu ./config ./core
go test ./agent/...
```

Build:

```bash
make build
```

## Feishu Context Rules

- `group_context_buffer` means: when mentioned, fetch recent Feishu group history and inject it as background context.
- It is not the old silent side-channel mechanism.
- Interactive cards/progress cards should not be treated as normal group discussion context.
- User/app/chat/member names should be resolved and cached under `~/.agentchat` when possible.

## Documentation Rules

- Keep the README split by language: `README.md` for Chinese by default and `README.en.md` for English.
- Keep setup docs centered on `agentchat feishu setup`.
- Do not document removed chat platform adapters as supported features.
- Keep the cc-connect acknowledgement and MIT license.
