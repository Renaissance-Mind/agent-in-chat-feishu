# Management API

Management API 是可选的本地 HTTP API，用于查看和控制正在运行的 `agentchat` 进程。它适合仪表盘、本地工具、托盘程序和自动化脚本使用。

## 开启

```toml
[management]
enabled = true
port = 9820
token = "${AGENTCHAT_MANAGEMENT_TOKEN}"
```

Base URL：

```text
http://localhost:9820/api/v1
```

## 认证

使用 bearer token：

```bash
curl -H "Authorization: Bearer $AGENTCHAT_MANAGEMENT_TOKEN" \
  http://localhost:9820/api/v1/status
```

## 常用接口

| 接口 | 方法 | 用途 |
|---|---|---|
| `/status` | `GET` | 进程状态 |
| `/projects` | `GET` | 项目列表 |
| `/sessions` | `GET` | 活跃会话 |
| `/sessions/{id}/messages` | `GET` | 查看最近消息 |
| `/send` | `POST` | 向会话发送消息 |
| `/restart` | `POST` | 重启进程 |
| `/config/reload` | `POST` | 从磁盘重新加载配置 |
| `/cron/jobs` | `GET` | 定时任务列表 |
| `/heartbeat/{project}` | `GET` | heartbeat 状态 |

成功响应：

```json
{
  "ok": true,
  "data": {}
}
```

错误响应：

```json
{
  "ok": false,
  "error": "message"
}
```

## 示例：查看状态

```bash
curl -H "Authorization: Bearer $AGENTCHAT_MANAGEMENT_TOKEN" \
  http://localhost:9820/api/v1/status
```

示例返回：

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

## 示例：向飞书会话发送消息

```bash
curl -X POST http://localhost:9820/api/v1/send \
  -H "Authorization: Bearer $AGENTCHAT_MANAGEMENT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "project": "my-project",
    "session_key": "feishu:oc_xxx",
    "message": "发一条简短状态更新。"
  }'
```

## 安全建议

- 默认只监听本机。
- 使用强随机 token。
- 避免把 token 放在 URL 中；优先使用 Header。
- 如果需要远程访问，请放在可信反向代理后面。
