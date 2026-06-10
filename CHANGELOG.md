# Changelog

## Unreleased

- Forked into a Feishu/Lark-only distribution named `agent-in-chat-feishu`.
- Kept cc-connect core agent runtime, session management, chat commands, providers, daemon, cron, relay, webhook, bridge, and management API.
- Kept supported agent backends: Codex, Claude Code, OpenCode, Gemini, Kimi, Qoder, iFlow, Cursor, ACP, and Pi.
- Removed bundled non-Feishu chat platform adapters and user-facing docs for those adapters.
- Renamed the CLI command to `agentchat`.
- Changed the default runtime data directory to `~/.agentchat`.
- Added persistent Feishu identity caching for user, app, chat, and group member names.
- Kept Feishu group history context on mention while removing the old silent side-channel context mechanism.
- Rewrote README, install docs, usage docs, Feishu docs, and example config for this distribution.
