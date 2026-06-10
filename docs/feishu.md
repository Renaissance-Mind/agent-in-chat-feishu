# 飞书 / Lark 接入指南

[English README](../README.md) | [中文 README](../README.zh-CN.md)

本文档介绍如何把 `agentchat` 接入飞书或 Lark。这个项目只保留 Feishu/Lark 聊天适配，但保留 cc-connect 的 Agent 运行时、会话、命令、进度卡片、附件、定时任务、relay、management API 和多 Agent 支持。

## 快速开始

默认用 `setup`：

```bash
agentchat feishu setup --project my-project
```

关联已有应用：

```bash
agentchat feishu setup --project my-project --app cli_xxx:sec_xxx
```

启动：

```bash
agentchat
```

`setup` 会自动准备项目配置。如果 `--project` 不存在，会创建项目；如果项目里没有 Feishu/Lark 平台，会自动补一个。命令完成时还会打印权限申请、权限后台和事件订阅直达链接。

新项目默认使用聊天绑定，而不是 allow-all。如果已设置 `admin_from`，管理员把机器人拉进群或发起私聊后，第一次有效触发会自动绑定该会话并持久化 `chat_id`；如果不是管理员触发，机器人会返回需要加入 `allow_group_chats` 或 `allow_private_chats` 的 `chat_id`。

## 创建机器人

### 方式一：扫码新建

```bash
agentchat feishu setup --project my-project
```

终端会打印二维码和 URL。用飞书/Lark 手机 App 扫码后，注册流程通常会创建机器人应用，并预配核心能力。命令结束时会打印 `scope-apply` 权限申请链接，链接里已经预选推荐 scopes。

完成后建议按终端链接核验：

- 应用已经发布
- 机器人能力已启用
- 长连接事件订阅已启用
- 可用范围包含要使用的群和用户
- 权限状态不是待审批或未发布

### 方式二：关联已有应用

```bash
agentchat feishu setup --project my-project --app cli_xxx:sec_xxx
```

这会校验 `app_id/app_secret`，然后写入 `config.toml`。它不会直接修改已有应用的权限申请，但会打印带预选 scope 的权限申请链接，以及对应的权限后台和事件订阅页面。

之后可随时重新打印这些链接：

```bash
agentchat feishu permissions --project my-project
```

开放平台入口：

- 飞书：https://open.feishu.cn/app
- Lark：https://open.larksuite.com/app

## 配置示例

```toml
[[projects]]
name = "my-project"
admin_from = ""
reply_footer = false
inject_sender = true
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
group_reply_all = false
share_session_in_channel = true
group_context_buffer = true
context_buffer_max_messages = 100
context_buffer_max_age_mins = 0
progress_style = "card"
enable_feishu_card = true
reply_to_trigger = true
reaction_emoji = "OnIt"
```

常用字段：

| 字段 | 作用 |
|---|---|
| `allow_group_chats` | 允许访问的群聊 chat_id；默认空字符串表示未绑定任何群 |
| `allow_private_chats` | 允许访问的私聊 chat_id；默认空字符串表示未绑定任何私聊 |
| `auto_bind_chats` | `true` 时允许 `admin_from` 用户首次有效触发时自动绑定群聊或私聊 |
| `allow_from` | 允许访问的用户 open_id，`*` 表示所有人 |
| `group_reply_all` | `false` 时群里只有 @ 机器人才触发 |
| `share_session_in_channel` | 群内共享一个 Agent 会话 |
| `thread_isolation` | 按飞书 reply thread/root 隔离会话 |
| `group_context_buffer` | 被 @ 时拉取最近群历史并作为背景上下文 |
| `context_buffer_max_messages` | 每个群最多保留多少条上下文 |
| `context_buffer_max_age_mins` | 群上下文按时间过期，`0` 表示不按时间过期 |
| `progress_style` | `legacy`、`compact` 或 `card` |
| `reaction_emoji` | 收到消息后自动加的表情，`none` 表示关闭 |
| `done_emoji` | Agent 完成后自动加的表情，`none` 表示关闭 |
| `resolve_mentions` | 发送消息时按群成员名称解析 @ |

默认运行数据目录是 `~/.agentchat`。Feishu 用户名、群名、群成员名和应用名映射会落盘缓存到该目录下，避免每次都把长 user/app/chat ID 塞进 Agent 输入。

## 群历史上下文

当前机制是：用户 @ 机器人时，agentchat 调用飞书历史消息接口拉取最近群消息，过滤后缓存，并把这段历史作为背景上下文传给 Agent。

它不是旧的 silent 消息机制，也不要求每条未 @ 消息都先触发本地 Agent。

飞书群里的消息：

```text
Mina：这个 PR 昨晚的测试挂了。
Alex：我看失败点像是配置文件路径。
River：日志里还有一次权限错误。
Alex：@agentchat 看一下最近上下文，帮我们判断先修哪一个。
```

Agent 看到的输入：

```text
[Feishu group context]
Mina：这个 PR 昨晚的测试挂了。
Alex：我看失败点像是配置文件路径。
River：日志里还有一次权限错误。
[/Feishu group context]

Alex：看一下最近上下文，帮我们判断先修哪一个。
```

上下文构建会默认跳过 Feishu interactive/card 消息，因此其他机器人的进度卡片不会被当成聊天内容。普通文本回复仍会进入上下文。

## 权限清单

如果希望机器人拥有和当前运行版本一样的能力，建议启用机器人能力、长连接事件，并开通这些权限/事件：

| 能力 | 权限或事件 |
|---|---|
| 接收群里 @ 机器人消息 | `im.message.receive_v1`，并具备群 @ 消息接收权限 |
| 接收私聊消息 | `im.message.receive_v1`，并具备单聊消息接收权限 |
| 拉取群历史上下文和引用消息 | `im:message`、`im:message:readonly`、`im:message.group_msg` |
| 发送消息 | `im:message` 或 `im:message:send_as_bot` |
| 回复消息 | `im:message` 或 `im:message:send_as_bot` |
| 更新进度/状态卡片 | `im:message:update` |
| 撤回临时预览消息 | `im:message:recall` |
| 添加/删除表情回复 | `im:message.reactions:write_only` |
| 上传/下载图片和文件 | `im:resource` 和 `im:resource:upload` |
| 读取群成员名称 | `im:chat.members:read` 或更宽的群信息权限 |
| 读取用户名称 | `contact:user.base:readonly` |
| 交互卡片按钮 | 事件 `card.action.trigger` |
| 机器人自定义菜单回调 | 事件 `application.bot.menu_v6` |

`setup` 打印的权限申请链接会预选运行时推荐 scopes：`im:message`、`im:message:readonly`、`im:message:send_as_bot`、`im:message:update`、`im:message:recall`、`im:message.group_msg`、`im:message.reactions:write_only`、`im:resource`、`im:resource:upload`、`im:chat:readonly`、`im:chat.members:read` 和 `contact:user.base:readonly`。如果运行时仍缺权限，日志会包含缺失 scope 和对应的直达申请链接。

官方参考：

- [发送消息](https://open.feishu.cn/document/server-docs/im-v1/message/create)
- [回复消息](https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/reference/im-v1/message/reply)
- [接收消息事件](https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/reference/im-v1/message/events/receive)
- [获取会话历史消息](https://open.feishu.cn/document/server-docs/im-v1/message/list)
- [添加消息表情回复](https://open.feishu.cn/document/server-docs/im-v1/message-reaction/create?lang=zh-CN)
- [获取群成员列表](https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/reference/im-v1/chat-members/get)
- [上传图片](https://open.feishu.cn/document/server-docs/im-v1/image/create)

## 事件订阅

推荐使用长连接模式，不需要公网 IP、域名或反向代理。

需要订阅：

| 事件 | 用途 |
|---|---|
| `im.message.receive_v1` | 接收用户消息 |
| `card.action.trigger` | 处理权限确认、provider 切换等卡片按钮 |
| `application.bot.menu_v6` | 处理机器人自定义菜单的事件回调；如果菜单项配置成直接发送文字，则不是必需 |

如果暂时无法配置卡片回调，可以设置：

```toml
enable_feishu_card = false
```

这样交互会尽量回退到纯文本。

## 常用命令

在飞书中发送：

```text
/help
/status
/whoami
/model
/stop
/new
/list
/history
/provider
/mode
/cron
/usage
```

本地 CLI：

```bash
agentchat sessions list
agentchat send --session <session-id> --message "发一条状态更新"
agentchat daemon start
agentchat daemon logs
```

## 常见问题

### 群历史没有进入 Agent 上下文

检查：

- `group_context_buffer = true`
- 机器人在群里
- 已开通群消息历史权限，尤其是 `im:message.group_msg`
- 用户是 @ 机器人触发，而不是普通未 @ 消息

### 群成员名字仍然显示成 App 或 User

通常是缺少群成员/通讯录相关权限，或者第一次遇到该 ID 时拉取失败。补齐权限后重新触发一次，成功解析后会写入 `~/.agentchat` 下的身份缓存。

### 自动表情失败

检查机器人是否在消息所在会话内，并确认表情权限或 `im:message` 权限正常。也可以设置：

```toml
reaction_emoji = "none"
done_emoji = "none"
```

### 卡片按钮点击无响应

检查是否订阅 `card.action.trigger`。如果权限或事件暂时不可用，先关闭卡片：

```toml
enable_feishu_card = false
progress_style = "compact"
```
