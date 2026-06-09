# Bridge Protocol

The Bridge Protocol is an optional WebSocket interface for external runtime tools that want to send messages into `agentchat` and receive replies. It is retained from cc-connect as a core integration surface.

This Feishu distribution does not bundle other chat app adapters. If you enable bridge, treat it as a generic local integration API.

## Enable

```toml
[bridge]
enabled = true
port = 9810
path = "/bridge/ws"
token = "${AGENTCHAT_BRIDGE_TOKEN}"
```

Endpoint:

```text
ws://localhost:9810/bridge/ws
```

Authenticate with one of:

- `Authorization: Bearer <token>`
- `X-Bridge-Token: <token>`
- `?token=<token>`

## Register

The first message must register the adapter:

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

## Send A Message

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

`session_key` is adapter-defined. It should be stable for the conversation you want to reuse.

## Card Action

```json
{
  "type": "card_action",
  "session_key": "local-tool:project:alex",
  "action": "cmd:/new",
  "reply_ctx": "conv-001"
}
```

## Replies

The bridge sends engine output back to the adapter:

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

Adapters should periodically send:

```json
{ "type": "ping" }
```

The server responds:

```json
{ "type": "pong" }
```

## Security

- Keep bridge disabled unless you need it.
- Use a strong token.
- Bind to localhost or a trusted private network.
- Do not expose the bridge endpoint directly to the public internet.
