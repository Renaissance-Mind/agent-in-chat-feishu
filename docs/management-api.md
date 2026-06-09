# Management API

The Management API is an optional local HTTP API for inspecting and controlling a running `agentchat` process. It is useful for dashboards, local tooling, tray apps, and automation.

## Enable

```toml
[management]
enabled = true
port = 9820
token = "${AGENTCHAT_MANAGEMENT_TOKEN}"
```

Base URL:

```text
http://localhost:9820/api/v1
```

## Authentication

Use a bearer token:

```bash
curl -H "Authorization: Bearer $AGENTCHAT_MANAGEMENT_TOKEN" \
  http://localhost:9820/api/v1/status
```

## Common Endpoints

| Endpoint | Method | Purpose |
|---|---|---|
| `/status` | `GET` | Process status and summary |
| `/projects` | `GET` | List configured projects |
| `/sessions` | `GET` | List active sessions |
| `/sessions/{id}/messages` | `GET` | Inspect recent messages |
| `/send` | `POST` | Send a message into a session |
| `/restart` | `POST` | Restart the process |
| `/config/reload` | `POST` | Reload config from disk |
| `/cron/jobs` | `GET` | List cron jobs |
| `/heartbeat/{project}` | `GET` | Inspect heartbeat state |

Responses use:

```json
{
  "ok": true,
  "data": {}
}
```

Errors use:

```json
{
  "ok": false,
  "error": "message"
}
```

## Example: Status

```bash
curl -H "Authorization: Bearer $AGENTCHAT_MANAGEMENT_TOKEN" \
  http://localhost:9820/api/v1/status
```

Example response:

```json
{
  "ok": true,
  "data": {
    "version": "v1.3.2",
    "connected_platforms": ["feishu"],
    "projects_count": 1,
    "bridge_adapters": []
  }
}
```

## Example: Send To A Feishu Session

```bash
curl -X POST http://localhost:9820/api/v1/send \
  -H "Authorization: Bearer $AGENTCHAT_MANAGEMENT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "project": "my-project",
    "session_key": "feishu:oc_xxx",
    "message": "Post a short status update."
  }'
```

## Security

- Bind to localhost unless you intentionally expose it.
- Use a strong random token.
- Avoid putting tokens in URLs; headers are safer.
- Put the API behind a trusted reverse proxy if remote access is needed.
