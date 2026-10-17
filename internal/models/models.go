package models

import "time"

const (
	BotTypeGrokBot = "grokbot"
	BotTypeDots    = "dots"
	BotTypeOther   = "other"

	MemberOnline  = "online"
	MemberOffline = "offline"

	DeliveryPending = "pending"
	DeliveryOK      = "ok"
	DeliveryFail    = "fail"
)

type Linker struct {
	ID         string    `gorm:"column:id;primaryKey" json:"id"`
	Name       string    `gorm:"column:name;not null" json:"name"`
	AdminToken string    `gorm:"column:admin_token;uniqueIndex;not null" json:"-"`
	CreatedAt  time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (Linker) TableName() string { return "linkers" }

type JoinToken struct {
	ID             string     `gorm:"column:id;primaryKey" json:"id"`
	LinkerID       string     `gorm:"column:linker_id;index;not null" json:"linker_id"`
	Token          string     `gorm:"column:token;uniqueIndex;not null" json:"token"`
	BotType        string     `gorm:"column:bot_type;not null" json:"bot_type"`
	Nickname       string     `gorm:"column:nickname;not null" json:"nickname"`
	ExpiresAt      time.Time  `gorm:"column:expires_at;not null" json:"expires_at"`
	UsedAt         *time.Time `gorm:"column:used_at" json:"used_at,omitempty"`
	UsedByMemberID *string    `gorm:"column:used_by_member_id" json:"used_by_member_id,omitempty"`
	CreatedAt      time.Time  `gorm:"column:created_at;not null" json:"created_at"`
}

func (JoinToken) TableName() string { return "join_tokens" }

type Member struct {
	ID                 string     `gorm:"column:id;primaryKey" json:"id"`
	LinkerID           string     `gorm:"column:linker_id;index;not null" json:"linker_id"`
	BotType            string     `gorm:"column:bot_type;not null" json:"bot_type"`
	Nickname           string     `gorm:"column:nickname;not null" json:"nickname"`
	WebhookURL         string     `gorm:"column:webhook_url;not null" json:"webhook_url"`
	WebhookAuthHeader  string     `gorm:"column:webhook_auth_header;not null" json:"-"`
	MemberToken        string     `gorm:"column:member_token;uniqueIndex;not null" json:"-"`
	Status             string     `gorm:"column:status;not null" json:"status"`
	LastSentAt         *time.Time `gorm:"column:last_sent_at" json:"last_sent_at,omitempty"`
	LastDeliverOKAt    *time.Time `gorm:"column:last_deliver_ok_at" json:"last_deliver_ok_at,omitempty"`
	LastDeliverErrAt   *time.Time `gorm:"column:last_deliver_err_at" json:"last_deliver_err_at,omitempty"`
	LastDeliverError   string     `gorm:"column:last_deliver_error;not null" json:"last_deliver_error,omitempty"`
	LastHeartbeatAt    *time.Time `gorm:"column:last_heartbeat_at" json:"last_heartbeat_at,omitempty"`
	JoinedAt           time.Time  `gorm:"column:joined_at;not null" json:"joined_at"`
}

func (Member) TableName() string { return "members" }

type Message struct {
	ID           string    `gorm:"column:id;primaryKey" json:"id"`
	LinkerID     string    `gorm:"column:linker_id;index;not null" json:"linker_id"`
	FromMemberID string    `gorm:"column:from_member_id;not null" json:"from_member_id"`
	Body         string    `gorm:"column:body;not null" json:"body"`
	CreatedAt    time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (Message) TableName() string { return "messages" }

type Delivery struct {
	ID          string     `gorm:"column:id;primaryKey" json:"id"`
	MessageID   string     `gorm:"column:message_id;index;not null" json:"message_id"`
	ToMemberID  string     `gorm:"column:to_member_id;not null" json:"to_member_id"`
	Status      string     `gorm:"column:status;index;not null" json:"status"`
	Attempts    int        `gorm:"column:attempts;not null" json:"attempts"`
	LastError   string     `gorm:"column:last_error;not null" json:"last_error,omitempty"`
	DeliveredAt *time.Time `gorm:"column:delivered_at" json:"delivered_at,omitempty"`
	CreatedAt   time.Time  `gorm:"column:created_at;not null" json:"created_at"`
}

func (Delivery) TableName() string { return "deliveries" }

func ValidBotType(t string) bool {
	switch t {
	case BotTypeGrokBot, BotTypeDots, BotTypeOther:
		return true
	default:
		return false
	}
}
