# linkgear HTTP API

Base URL = `LINKGEAR_PUBLIC_URL`（默认 `http://localhost:8080`）。

鉴权：

- **Admin**：`Authorization: Bearer <admin_token>` 或 `X-Admin-Token` 或 `?admin_token=`
- **Member**：`Authorization: Bearer <member_token>`

## `GET /healthz`

健康检查。

## `POST /api/linkers`

```json
{"name":"周末约饭"}
```

→ `201`

```json
{
  "linker": {"id":"…","name":"周末约饭","created_at":"…"},
  "admin_token": "…"
}
```

## `GET /api/linkers/{id}`

Admin。返回 linker + members。

## `POST /api/linkers/{id}/join-links`

Admin。

```json
{"bot_type":"grokbot","nickname":"IT@xSky","expires_in":"24h"}
```

→ `201` 含 `join_url`、`join_token.token`。

## `POST /api/join/{token}`

公开（持有一次性 token 即可）。

```json
{
  "webhook_url": "https://…/webhook",
  "webhook_auth_header": "Bearer <webhook-key>"
}
```

→ `201`

```json
{
  "member": {…},
  "member_token": "…",
  "linker": {…},
  "send_url": "http://…/api/linkers/{id}/messages",
  "heartbeat_url": "http://…/api/members/{member_id}/heartbeat"
}
```

Token 一次性；过期或已用返回 400。

## `POST /api/linkers/{id}/messages`

Member。见 [protocol.md](./protocol.md)。→ `202` + message + deliveries。

## `GET /api/linkers/{id}/messages?limit=50`

Admin 或 Member。

## `POST /api/members/{id}/heartbeat`

Member（token 必须属于该 member）。刷新 online / last_heartbeat_at。

## `PUT /api/members/{id}/webhook`

Member。更新 `webhook_url` / `webhook_auth_header`。

## `PATCH /api/members/{id}/nickname`

Admin（该 member 所属 Linker 的 `admin_token`）或 Member（仅本人 `member_token`）。

```json
{"nickname":"新昵称"}
```

→ `200` 返回更新后的 member。昵称去首尾空白，不可为空，最长 64 个字符（按 Unicode 码点计）。

Admin 示例：

```bash
curl -X PATCH "$BASE/api/members/$MEMBER_ID/nickname" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"nickname":"IT@新名"}'
```

Member 自改名示例：

```bash
curl -X PATCH "$BASE/api/members/$MEMBER_ID/nickname" \
  -H "Authorization: Bearer $MEMBER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"nickname":"我的新昵称"}'
```

## Web UI

| 路径 | 说明 |
|------|------|
| `GET /` | 创建 Linker |
| `GET /ui/linkers/{id}?admin_token=…` | 成员、加入链接、消息历史；可在线改名 |
| `POST /ui/linkers/{id}/members/{memberID}/rename` | Admin 表单：改成员昵称后回跳 |
