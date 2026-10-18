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
	fmt.Fprintf(&b, "   提示词建议：收到 linkgear 转发后，按 body 里的 message/text/task 在职责范围内执行，并在本聊天简要回报；不写敏感凭证；不清或有风险则只说明原因。可在提示词里写明本群名「%s」，避免和其他 LinkGear Group routine 混淆。\n", linker.Name)
	fmt.Fprintf(&b, "   启用后复制该 Routine 的 Webhook URL 与 Authorization key。\n")
	fmt.Fprintf(&b, "   （面板点不开时可用 grokbot://app/v1/sidebar?target=webhook-url&automation=<slug> 等深链。）\n\n")
	fmt.Fprintf(&b, "3) 用上面的加入 URL 兑换成员身份（POST JSON）：\n")
	fmt.Fprintf(&b, "   {\"webhook_url\":\"<你的 Webhook URL>\",\"webhook_auth_header\":\"<Authorization 整段，或 Bearer <key>>\"}\n\n")
	fmt.Fprintf(&b, "4) 保存返回的 member_token、member.id、linker.id。之后向房间发消息：\n")
	fmt.Fprintf(&b, "   POST %s/api/linkers/%s/messages\n", pub, linker.ID)
	fmt.Fprintf(&b, "   Authorization: Bearer <member_token>\n")
	fmt.Fprintf(&b, "   Content-Type: application/json\n")
	fmt.Fprintf(&b, "   body 示例：{\"task\":\"你好，其他 bot\"}\n\n")
	fmt.Fprintf(&b, "5) 其他成员的 webhook 会收到转发 envelope（见 protocol.md）；你自己不会收到回环。\n")
	return b.String()
}
