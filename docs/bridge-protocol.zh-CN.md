# Bridge 协议

Bridge Protocol 是可选的 WebSocket 接口，用于让外部运行时工具向 `agentchat` 发送消息并接收回复。它作为 cc-connect 的核心集成面被保留下来。

这个 Feishu 发行版不内置其他聊天软件适配器。如果启用 bridge，请把它当成通用本地集成 API。

## 开启

```toml
[bridge]
enabled = true
port = 9810
path = "/bridge/ws"
token = "${AGENTCHAT_BRIDGE_TOKEN}"
```

Endpoint：

```text
ws://localhost:9810/bridge/ws
```

认证方式：

- `Authorization: Bearer <token>`
- `X-Bridge-Token: <token>`
- `?token=<token>`

## 注册

连接后的第一条消息必须注册 adapter：

```json
{
  "type": "register",
  "platform": "local-tool",
  "capabilities": ["text", "image", "file", "card", "buttons"],
  "metadata": {
    "version": "1.0.0"
  }
}
```

## 发送消息

```json
{
  "type": "message",
  "msg_id": "msg-001",
  "session_key": "local-tool:project:alex",
  "user_id": "alex",
  "user_name": "Alex",
  "content": "Review the current branch.",
  "reply_ctx": "conv-001",
  "images": [],
  "files": []
}
```

`session_key` 由 adapter 定义。它应当在同一个会话中保持稳定。

## 卡片动作

```json
{
  "type": "card_action",
  "session_key": "local-tool:project:alex",
  "action": "cmd:/new",
  "reply_ctx": "conv-001"
}
```

## 回复

Bridge 会把 Engine 输出发回 adapter：

```json
{
  "type": "reply",
  "session_key": "local-tool:project:alex",
  "reply_ctx": "conv-001",
  "content": "I found two issues in the branch.",
  "final": false
}
```

## Keepalive

Adapter 应定期发送：

```json
{ "type": "ping" }
```

服务端返回：

```json
{ "type": "pong" }
```

## 安全建议

- 不需要时保持 bridge 关闭。
- 使用强随机 token。
- 只监听本机或可信内网。
- 不要把 bridge 端点直接暴露到公网。
