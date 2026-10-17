# linkgear 协议：消息信封

Linker 向各成员 webhook **POST** 的 JSON 载荷（Content-Type: `application/json`）。

## Envelope

```json
{
  "linker_id": "a1b2c3…",
  "message_id": "d4e5f6…",
  "from": {
    "member_id": "…",
    "nickname": "IT@xSky",
    "bot_type": "grokbot"
  },
  "message": { "task": "查 Jenkins 状态" },
  "text": "可选：当入库 body 不是合法 JSON 时，原文字段会出现在这里",
  "ts": "2026-10-17T14:40:00Z"
}
```

| 字段 | 说明 |
|------|------|
| `linker_id` | 房间 ID |
| `message_id` | 消息 ID（可幂等去重） |
| `from` | 发送方成员信息（由加入时指定的 bot_type / nickname 决定） |
| `message` | 原始消息体。若发送时 body 是 JSON，则为解析后的对象/数组；若是纯文本，则为 JSON 字符串 |
| `text` | 仅当原始 body 不是合法 JSON 时填充 |
| `ts` | 消息创建时间（UTC） |

## 请求头

| Header | 说明 |
|--------|------|
| `Content-Type` | `application/json` |
| `User-Agent` | `linkgear/1.0` |
| `Authorization` | 若成员登记了 `webhook_auth_header`，原样写入（例如 `Bearer <key>`） |
| `X-Linkgear-Linker-Id` | 同 `linker_id` |
| `X-Linkgear-Message-Id` | 同 `message_id` |

## Bot 发送消息

`POST /api/linkers/{linker_id}/messages`

```http
Authorization: Bearer <member_token>
Content-Type: application/json

{"text":"任意任务描述"}
```

也接受：

- `{"body": …}` / `{"message": …}` — 任意 JSON，原样入库并转发
- `{"task":"…"}` / `{"msg":"…"}` — 当作文本
- 纯文本 body

Linker **不解释**消息语义，只做存储与 fan-out。

## bot_type 枚举

- `grokbot`
- `dots`（预留，本期无适配器）
- `other`
