# Contributing

[中文](#中文) | [English](#contributing)

Thanks for helping improve `agent-in-chat-feishu`.

This repository is a Feishu/Lark-only distribution that keeps cc-connect's agent runtime and chat processing surface. Please keep changes aligned with that shape unless the project direction changes explicitly.

## Issues

Please include:

- `agentchat --version`
- OS and install method
- agent type, for example `codex` or `claudecode`
- Feishu/Lark setup path, for example QR setup or existing app binding
- config snippet with secrets redacted
- logs with tokens, app secrets, and user identifiers redacted where needed
- reproduction steps

## Pull Requests

Before opening a PR:

```bash
go test ./...
```

Please update docs when a config field, command, permission, or setup flow changes.

## Scope

Good contributions:

- Feishu/Lark reliability
- group history context and identity mapping
- agent runtime compatibility
- slash commands and session behavior
- provider/model management
- daemon, cron, relay, webhook, management API
- tests and docs

Out of scope for this distribution:

- adding bundled adapters for other chat platforms
- restoring old platform-specific docs

## 中文

感谢你帮助改进 `agent-in-chat-feishu`。

这个仓库是 Feishu/Lark 专用发行版，同时保留 cc-connect 的 Agent 运行时和聊天处理能力。除非项目方向明确改变，请保持这个边界。

提交 issue 时建议包含：

- `agentchat --version`
- 操作系统和安装方式
- Agent 类型，例如 `codex` 或 `claudecode`
- 飞书/Lark 配置方式，例如扫码 setup 或绑定已有应用
- 配置片段，注意隐藏 secret
- 日志，注意隐藏 token、app secret 和必要的用户标识
- 复现步骤

提交 PR 前请至少运行：

```bash
go test ./...
```

如果改动影响配置项、命令、权限或 setup 流程，请同步更新文档。
