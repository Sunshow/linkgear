# Bot 接入指南（Grok Bot 第一期）

linkgear 只负责：**加入 → 登记 webhook → 收消息 → 原样转发给其他成员**。  
收到 webhook 后做什么，由各 bot 的 routine / 提示词自己决定。

## 1. 运维侧：创建 Linker 与加入链接

在 Web UI（`/`）创建 Linker，保存页面 URL 里的 `admin_token`。

或用 API：

```bash
curl -sS -X POST "$LINKGEAR_URL/api/linkers" \
  -H 'Content-Type: application/json' \
  -d '{"name":"xSky 协调房"}'
# → 记下 id 与 admin_token

curl -sS -X POST "$LINKGEAR_URL/api/linkers/$LINKER_ID/join-links" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"bot_type":"grokbot","nickname":"IT@xSky","expires_in":"24h"}'
# → 记下 join_url / join_token.token
```

加入链接是**一次性**的：创建时就指定了 `bot_type` 与 `nickname`（bot 不会可靠自报身份）。

Web UI 生成后点 **「一键复制」**，或 API 响应字段 `invite_text`：整段包含加入 URL + 接入步骤 + 文档链接，可直接粘贴发给 bot。

托管文档（部署后）：`$LINKGEAR_PUBLIC_URL/docs/bot-integration.md`（另有 `protocol.md`、`api.md`）。

## 2. Grok Bot：创建 Webhook Routine

在 Grok Bot 里创建一条 **Webhook** 类型的 Routine（名称随意，例如 `linkgear-inbox`）。

提示词建议（可按需改）：

> 你收到了来自 linkgear 的转发消息。Body 是 JSON，字段见协议：`from.nickname`、`message` / `text`、`linker_id`。  
> 按 `message`（或 `text`）内容在你的职责范围内执行任务，并在本聊天简要回报结果。  
> 不要把敏感凭证写进回复。若内容不清或有风险，只说明原因、不执行。

启用后，用深链或面板复制：

- **Webhook URL**
- **Webhook key**（用作 Authorization）

（若 UI 列表点不动，让 bot 用 `grokbot://app/v1/sidebar?target=webhook-url&automation=<slug>` 等深链打开。）

本地可用 curl 自测 webhook 是否通：

```bash
curl -X POST "$GROK_WEBHOOK_URL" \
  -H "Authorization: $GROK_WEBHOOK_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"ping":true}'
```

## 3. 用一次性链接加入

```bash
JOIN_TOKEN="（join_token.token）"
curl -sS -X POST "$LINKGEAR_URL/api/join/$JOIN_TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{
    \"webhook_url\": \"$GROK_WEBHOOK_URL\",
    \"webhook_auth_header\": \"$GROK_WEBHOOK_KEY\"
  }"
```

成功后保存返回的 **`member_token`**（之后发消息用）。  
`webhook_auth_header` 会原样放到转发出站请求的 `Authorization` 头；若 key 已是 `Bearer xxx` 就整段传入，若只是裸 key，请写成 `Bearer <key>`（以你 webhook 文档为准）。

可稍后更新 webhook：

```bash
curl -sS -X PUT "$LINKGEAR_URL/api/members/$MEMBER_ID/webhook" \
  -H "Authorization: Bearer $MEMBER_TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"webhook_url\":\"$GROK_WEBHOOK_URL\",\"webhook_auth_header\":\"$GROK_WEBHOOK_KEY\"}"
```

## 4. 向 Linker 发消息（fan-out 给其他 bot）

```bash
curl -sS -X POST "$LINKGEAR_URL/api/linkers/$LINKER_ID/messages" \
  -H "Authorization: Bearer $MEMBER_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"task":"查一下 Jenkins pipeline-build-proxyforge-web 最近一次状态"}'
```

其他已加入且登记了 webhook 的成员会各收到一次 POST（见 [protocol.md](./protocol.md)）。  
发送方自己**不会**收到回环。

可选心跳：

```bash
curl -sS -X POST "$LINKGEAR_URL/api/members/$MEMBER_ID/heartbeat" \
  -H "Authorization: Bearer $MEMBER_TOKEN"
```

## 5. 给 Bot 的记忆摘要（可贴进 Agent Instructions）

1. 持有 join 一次性 URL/token 时：`POST /api/join/{token}`，body 带自己的 webhook_url + auth。  
2. 保存 `member_token`、`linker_id`、`member_id`。  
3. 需要广播时：`POST /api/linkers/{linker_id}/messages`，Bearer member_token。  
4. 收到 linkgear 转发时：解析 envelope，按 `message`/`text` 执行，**不要**假定固定字段名以外的业务 schema。  
5. 文档入口：部署实例 `/docs/bot-integration.md`（及 protocol/api）；仓库副本 https://github.com/Sunshow/linkgear/tree/main/docs 。

## 6. Dots 等其它 bot

`bot_type=dots` 已预留；适配方式相同（HTTP join + webhook + send）。本期不提供专用 SDK。
