package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/sunshow/linkgear/internal/models"
	"gorm.io/gorm"
)

type SendMessageInput struct {
	// Body is stored as-is (JSON string or plain text). If Raw is set, it is marshaled.
	Text string
	Raw  json.RawMessage
}

type SendMessageResult struct {
	Message    models.Message    `json:"message"`
	Deliveries []models.Delivery `json:"deliveries"`
}

func (s *Service) SendMessage(linkerID, memberToken string, in SendMessageInput) (*SendMessageResult, error) {
	m, err := s.MemberByToken(memberToken)
	if err != nil {
		return nil, err
	}
	if m.LinkerID != linkerID {
		return nil, ErrUnauthorized
	}

	body := strings.TrimSpace(in.Text)
	if len(in.Raw) > 0 {
		body = string(in.Raw)
	}
	if body == "" {
		return nil, fmt.Errorf("%w: body or text required", ErrBadRequest)
	}

	now := time.Now().UTC()
	msg := models.Message{
		ID:           newID(),
		LinkerID:     linkerID,
		FromMemberID: m.ID,
		Body:         body,
		CreatedAt:    now,
	}

	var peers []models.Member
	var deliveries []models.Delivery

	err = s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&msg).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.Member{}).Where("id = ?", m.ID).Updates(map[string]any{
			"last_sent_at":      now,
			"last_heartbeat_at": now,
			"status":            models.MemberOnline,
		}).Error; err != nil {
			return err
		}
		if err := tx.Where("linker_id = ? AND id != ? AND webhook_url != ''", linkerID, m.ID).Find(&peers).Error; err != nil {
			return err
		}
		for _, p := range peers {
			d := models.Delivery{
				ID:         newID(),
				MessageID:  msg.ID,
				ToMemberID: p.ID,
				Status:     models.DeliveryPending,
				Attempts:   0,
				CreatedAt:  now,
			}
			if err := tx.Create(&d).Error; err != nil {
				return err
			}
			deliveries = append(deliveries, d)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &SendMessageResult{Message: msg, Deliveries: deliveries}, nil
}

type MessageView struct {
	models.Message
	FromNickname string `json:"from_nickname"`
	FromBotType  string `json:"from_bot_type"`
}

func (s *Service) ListMessages(linkerID string, limit int) ([]MessageView, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var msgs []models.Message
	if err := s.DB.Where("linker_id = ?", linkerID).Order("created_at desc").Limit(limit).Find(&msgs).Error; err != nil {
		return nil, err
	}
	out := make([]MessageView, 0, len(msgs))
	for _, msg := range msgs {
		var mem models.Member
		_ = s.DB.Select("nickname", "bot_type").First(&mem, "id = ?", msg.FromMemberID).Error
		out = append(out, MessageView{
			Message:      msg,
			FromNickname: mem.Nickname,
			FromBotType:  mem.BotType,
		})
	}
	return out, nil
}

func (s *Service) ListDeliveriesForMessage(messageID string) ([]models.Delivery, error) {
	var ds []models.Delivery
	if err := s.DB.Where("message_id = ?", messageID).Find(&ds).Error; err != nil {
		return nil, err
	}
	return ds, nil
}
