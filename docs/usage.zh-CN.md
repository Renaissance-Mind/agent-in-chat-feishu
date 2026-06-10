# 使用说明

[English README](../README.md) | [中文 README](../README.zh-CN.md)

本文档说明 `agent-in-chat-feishu` 保留下来的 cc-connect 运行时能力。聊天平台只保留 Feishu/Lark；Agent、会话、模型提供方、命令、daemon、cron、relay、webhook 和 management API 能力继续保留。

## 飞书配置

```bash
agentchat setup feishu
agentchat setup feishu --app cli_xxx:sec_xxx
agentchat feishu permissions
```

不传 `--project` 时，默认连接 Codex，使用本地机器人配置 `feishu`，初始工作目录为 `~/.agentchat/feishu/`。后续可以在聊天里用 `/dir` 或 `/workspace` 切换到真正要操作的代码仓库。

`setup` 成功后会默认安装并启动后台服务，尽量自动打开权限确认页面，并把权限确认直达链接作为最后一步打印出来。需要只写配置不启动时，使用 `agentchat setup feishu --no-start`。

权限、事件订阅和群历史上下文见 [飞书接入指南](feishu.md)。

## Daemon 后台服务

后台服务常用管理命令：

```bash
agentchat daemon status
agentchat daemon logs -f
agentchat daemon restart
```

setup 自动安装服务时会记录安装进程的 `PATH`，与 cc-connect 行为一致。如果你从非交互 shell 安装，或 Node、Agent CLI、`lark-cli` 来自自定义路径管理器，可以显式传入：

```bash
agentchat setup feishu --daemon-env-path "$PATH"
```

## 支持的 Agent

在 `config.toml` 中设置：

```toml
[projects.agent]
type = "codex"
```

支持：

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

## 聊天命令

在飞书中发送：

| 命令 | 作用 |
|---|---|
| `/help` | 查看命令 |
| `/status` | 查看项目/会话状态 |
| `/whoami` | 查看飞书身份 |
| `/new [name]` | 新建会话 |
| `/list` | 列出会话 |
| `/switch <id>` | 切换会话 |
| `/history [n]` | 查看最近会话历史 |
| `/stop` | 停止当前 Agent 执行 |
| `/model` | 查看或切换模型 |
| `/provider` | 管理模型提供方 |
| `/mode` | 查看或切换权限模式 |
| `/dir` 或 `/cd` | 查看或切换工作目录 |
| `/usage` | 查看用量/额度 |
| `/cron` | 管理定时任务 |
| `/commands` | 查看自定义命令 |
| `/alias` | 管理命令别名 |
| `/delete` | 删除会话 |
| `/workspace` | 管理 workspace 绑定 |

## 权限模式

不同 Agent 支持的模式略有差异，常见值：

```text
default
auto
auto-edit
plan
yolo
```

使用：

```text
/mode
/mode yolo
/mode default
```

## Provider 和模型

全局 provider 示例：

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

CLI：

```bash
agentchat provider list --project my-project
agentchat provider add --project my-project --name relay --api-key sk_xxx --base-url https://example.com/v1
agentchat provider remove --project my-project --name relay
```

聊天命令：

```text
/provider
/provider list
/provider switch openai
/model
/model switch codex
```

## 群历史上下文

开启：

```toml
[projects.platforms.options]
group_context_buffer = true
context_buffer_max_messages = 100
context_buffer_max_age_mins = 0
```

机器人被 @ 时，会拉取最近飞书群历史，过滤、缓存后作为背景上下文发送给 Agent。进度卡片默认不会进入这段上下文；可读的最终回复卡片会保留。

## 身份缓存

飞书用户名、应用名、群名和群成员名会落盘缓存在 `~/.agentchat` 下。这样 Agent 输入中会尽量显示人名，而不是很长的 ID。

## 附件回传

把文件或图片发回飞书会话：

```bash
agentchat send --session <session-id> --message "完成了"
agentchat send --session <session-id> --image /path/to/screenshot.png
agentchat send --session <session-id> --file /path/to/report.pdf
```

## Cron 和 Heartbeat

Heartbeat 示例：

```toml
[[projects]]
name = "my-project"

[projects.heartbeat]
enabled = true
interval_mins = 30
only_when_idle = true
session_key = "feishu:oc_xxx"
prompt = "检查项目是否有需要注意的地方。"
```

使用：

```text
/cron
```

```bash
agentchat cron list
```

## Webhook

开启外部 HTTP 触发：

```toml
[webhook]
enabled = true
port = 9111
token = "${AGENTCHAT_WEBHOOK_TOKEN}"
path = "/hook"
```

示例：

```bash
curl -X POST 'http://localhost:9111/hook/prompt' \
  -H 'Authorization: Bearer your-token' \
  -H 'Content-Type: application/json' \
  -d '{"project":"my-project","session_key":"feishu:oc_xxx","prompt":"Review the latest commit"}'
```

## Daemon

```bash
agentchat daemon start
agentchat daemon status
agentchat daemon logs
```

## Management API

```toml
[management]
enabled = true
port = 9820
token = "${AGENTCHAT_MANAGEMENT_TOKEN}"
```

见 [Management API](management-api.zh-CN.md)。

## Bridge

Bridge server 保留给外部运行时集成和工具接入。它不是这个发行版内置的其他聊天平台适配器。

```toml
[bridge]
enabled = true
port = 9810
token = "${AGENTCHAT_BRIDGE_TOKEN}"
path = "/bridge/ws"
```

见 [Bridge 协议](bridge-protocol.zh-CN.md)。

## Multi-Workspace

一个项目可以路由到不同工作目录：

```toml
[[projects]]
name = "workspace-router"
mode = "multi-workspace"
base_dir = "/Users/alex/work"
```

在飞书里用 `/workspace`、`/dir` 或 `/cd` 查看和切换工作区。
