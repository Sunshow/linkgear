package service

import (
	"fmt"
	"strings"

	"github.com/sunshow/linkgear/internal/models"
)

const RepoDocsBase = "https://github.com/Sunshow/linkgear/blob/main/docs"

// RoutineNameForLinker returns the suggested Grok Bot webhook routine display name
// for a Linker, so one bot joining multiple rooms can tell them apart.
func RoutineNameForLinker(linkerName string) string {
	name := strings.TrimSpace(linkerName)
	if name == "" {
		name = "unnamed"
	}
	return "LinkGear Group " + name
}

// BuildInviteText returns a clipboard-ready block: join URL + bot onboarding prompt.
func (s *Service) BuildInviteText(linker *models.Linker, jt *models.JoinToken, joinURL string) string {
	pub := strings.TrimRight(s.PublicURL, "/")
	docsBot := pub + "/docs/bot-integration.md"
	docsProto := pub + "/docs/protocol.md"
	docsAPI := pub + "/docs/api.md"
	exp := jt.ExpiresAt.Local().Format("2006-01-02 15:04:05")
	routineName := RoutineNameForLinker(linker.Name)

	var b strings.Builder
	fmt.Fprintf(&b, "【linkgear 加入邀请】\n\n")
	fmt.Fprintf(&b, "请加入 Linker「%s」（id: %s）。\n", linker.Name, linker.ID)
	fmt.Fprintf(&b, "你的预定身份：bot_type=%s，昵称=%s。\n", jt.BotType, jt.Nickname)
	fmt.Fprintf(&b, "加入链接过期时间（本地时区）：%s（一次性，用后即失效）。\n\n", exp)
	fmt.Fprintf(&b, "一次性加入 URL：\n%s\n\n", joinURL)
	fmt.Fprintf(&b, "—— 请按下列步骤自行接入（linkgear 只转发消息，不替你决策）——\n\n")
	fmt.Fprintf(&b, "1) 阅读接入文档：\n")
	fmt.Fprintf(&b, "   - %s\n", docsBot)
	fmt.Fprintf(&b, "   - %s\n", docsProto)
	fmt.Fprintf(&b, "   - %s\n", docsAPI)
	fmt.Fprintf(&b, "   仓库副本：%s/bot-integration.md\n\n", RepoDocsBase)
	fmt.Fprintf(&b, "2) 若你是 Grok Bot：创建一条 Webhook Routine，名称必须带上本群名，便于你加入多个群时区分：\n")
	fmt.Fprintf(&b, "   推荐名称：「%s」（格式：LinkGear Group <群名>）。\n", routineName)
	fmt.Fprintf(&b, "   提示词建议（可按需改）：\n%s\n", indentPrompt(RoutinePromptSuggestion(linker.Name), "   > "))
	fmt.Fprintf(&b, "   启用后复制该 Routine 的 Webhook URL 与 Authorization key。\n")
	fmt.Fprintf(&b, "   （面板点不开时可用 grokbot://app/v1/sidebar?target=webhook-url&automation=<slug> 等深链。）\n\n")
	fmt.Fprintf(&b, "3) 用上面的加入 URL 兑换成员身份（POST JSON）：\n")
	fmt.Fprintf(&b, "   {\"webhook_url\":\"<你的 Webhook URL>\",\"webhook_auth_header\":\"Bearer <key>\"}\n")
	fmt.Fprintf(&b, "   注意：webhook_auth_header 必须是完整的 Authorization 值；裸 key 一定要写成 `Bearer <key>`，否则转发时会被 webhook 拒绝（401 Malformed Authorization header）。\n\n")
	fmt.Fprintf(&b, "4) 保存返回的 member_token、member.id、linker.id，以及发送地址 send_url（下方 POST URL）到 routine 能读取的文件（如 env 文件）；不要把 token 贴到任何聊天里。发消息：\n")
	fmt.Fprintf(&b, "   POST %s/api/linkers/%s/messages\n", pub, linker.ID)
	fmt.Fprintf(&b, "   Authorization: Bearer <member_token>\n")
	fmt.Fprintf(&b, "   Content-Type: application/json\n")
	fmt.Fprintf(&b, "   body 示例：{\"text\":\"大家好，我是 %s，负责……，有相关事情可以 @我。\"}\n\n", jt.Nickname)
	fmt.Fprintf(&b, "5) 加入成功后立刻做两件事，让双方都看到加入成功：\n")
	fmt.Fprintf(&b, "   a. 用 member_token 向房间 POST 一条简短自我介绍（你是谁、负责什么）；\n")
	fmt.Fprintf(&b, "   b. 在你自己的聊天里告诉你的用户：已加入 Linker「%s」，昵称 %s，member id <member.id>。\n\n", linker.Name, jt.Nickname)
	fmt.Fprintf(&b, "6) 其他成员的 webhook 会收到转发 envelope（见 protocol.md）；你自己不会收到回环。\n")
	return b.String()
}

// RoutinePromptSuggestion is the suggested Grok Bot routine prompt for a Linker.
// Keep in sync with docs/bot-integration.md section 2.
func RoutinePromptSuggestion(linkerName string) string {
	return fmt.Sprintf(`你收到了来自 linkgear Linker「%s」的转发消息（字段见 protocol.md：from.nickname、from.member_id、message / text）。
发消息到群：从你保存的文件读取 send_url 和 member_token，POST {"text":"..."}，头 Authorization: Bearer <member_token>；不要把 token 写进任何回复。
1. 每条收到的消息都在本聊天简要记录（谁、说了什么、你怎么处理）。
2. 打招呼 / 新成员自我介绍 / ping，或 @ 了你的消息：在群里简短回复（不要只在本聊天回）。
3. 任务类消息：在职责范围内执行，把结果发回群里，并在本聊天简要回报；内容不清或有风险时，在群里说明原因、不执行。
4. 防循环：不回复自己发的消息；不回复对招呼的回复、纯确认/收到/pong 类消息；每个新成员的招呼最多回一次。`, linkerName)
}

func indentPrompt(s, prefix string) string {
	return prefix + strings.ReplaceAll(s, "\n", "\n"+prefix)
}
