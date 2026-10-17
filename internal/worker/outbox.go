package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/sunshow/linkgear/internal/models"
	"gorm.io/gorm"
)

// Envelope is the JSON payload POSTed to member webhooks.
type Envelope struct {
	LinkerID string          `json:"linker_id"`
	From     EnvelopeFrom    `json:"from"`
	Message  json.RawMessage `json:"message"`
	Text     string          `json:"text,omitempty"`
	TS       time.Time       `json:"ts"`
	MessageID string         `json:"message_id"`
}

type EnvelopeFrom struct {
	MemberID string `json:"member_id"`
	Nickname string `json:"nickname"`
	BotType  string `json:"bot_type"`
}

type Outbox struct {
	DB       *gorm.DB
	Workers  int
	Poll     time.Duration
	HTTP     *http.Client
	MaxTries int
}

func NewOutbox(db *gorm.DB, workers int, poll time.Duration) *Outbox {
	if workers < 1 {
		workers = 4
	}
	if poll <= 0 {
		poll = 2 * time.Second
	}
	return &Outbox{
		DB:       db,
		Workers:  workers,
		Poll:     poll,
		HTTP:     &http.Client{Timeout: 15 * time.Second},
		MaxTries: 5,
	}
}

func (o *Outbox) Run(ctx context.Context) {
	sem := make(chan struct{}, o.Workers)
	var wg sync.WaitGroup
	ticker := time.NewTicker(o.Poll)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return
		case <-ticker.C:
			o.drain(ctx, sem, &wg)
		}
	}
}

func (o *Outbox) drain(ctx context.Context, sem chan struct{}, wg *sync.WaitGroup) {
	var pending []models.Delivery
	if err := o.DB.Where("status = ?", models.DeliveryPending).Order("created_at asc").Limit(50).Find(&pending).Error; err != nil {
		log.Printf("outbox list: %v", err)
		return
	}
	for _, d := range pending {
		d := d
		select {
		case <-ctx.Done():
			return
		case sem <- struct{}{}:
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				o.deliverOne(ctx, d)
			}()
		}
	}
}

func (o *Outbox) deliverOne(ctx context.Context, d models.Delivery) {
	var msg models.Message
	if err := o.DB.First(&msg, "id = ?", d.MessageID).Error; err != nil {
		o.fail(d, fmt.Sprintf("load message: %v", err), false)
		return
	}
	var to models.Member
	if err := o.DB.First(&to, "id = ?", d.ToMemberID).Error; err != nil {
		o.fail(d, fmt.Sprintf("load member: %v", err), false)
		return
	}
	var from models.Member
	_ = o.DB.First(&from, "id = ?", msg.FromMemberID).Error

	raw := json.RawMessage(msg.Body)
	if !json.Valid([]byte(msg.Body)) {
		b, _ := json.Marshal(msg.Body)
		raw = b
	}
	env := Envelope{
		LinkerID:  msg.LinkerID,
		MessageID: msg.ID,
		From: EnvelopeFrom{
			MemberID: from.ID,
			Nickname: from.Nickname,
			BotType:  from.BotType,
		},
		Message: raw,
		TS:      msg.CreatedAt.UTC(),
	}
	if !json.Valid([]byte(msg.Body)) {
		env.Text = msg.Body
	}
	payload, err := json.Marshal(env)
	if err != nil {
		o.fail(d, err.Error(), false)
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, to.WebhookURL, bytes.NewReader(payload))
	if err != nil {
		o.fail(d, err.Error(), true)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "linkgear/1.0")
	req.Header.Set("X-Linkgear-Linker-Id", msg.LinkerID)
	req.Header.Set("X-Linkgear-Message-Id", msg.ID)
	if to.WebhookAuthHeader != "" {
		// Expect full header value e.g. "Bearer xxx" or "Key xxx"
		req.Header.Set("Authorization", to.WebhookAuthHeader)
	}

	resp, err := o.HTTP.Do(req)
	if err != nil {
		o.fail(d, err.Error(), true)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		o.fail(d, fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)), true)
		return
	}

	now := time.Now().UTC()
	_ = o.DB.Model(&d).Updates(map[string]any{
		"status":       models.DeliveryOK,
		"attempts":     gorm.Expr("attempts + 1"),
		"last_error":   "",
		"delivered_at": now,
	}).Error
	_ = o.DB.Model(&models.Member{}).Where("id = ?", to.ID).Updates(map[string]any{
		"last_deliver_ok_at": now,
		"last_deliver_error": "",
	}).Error
}

func (o *Outbox) fail(d models.Delivery, errMsg string, retryable bool) {
	now := time.Now().UTC()
	attempts := d.Attempts + 1
	status := models.DeliveryPending
	if !retryable || attempts >= o.MaxTries {
		status = models.DeliveryFail
	}
	_ = o.DB.Model(&d).Updates(map[string]any{
		"status":     status,
		"attempts":   attempts,
		"last_error": errMsg,
	}).Error
	_ = o.DB.Model(&models.Member{}).Where("id = ?", d.ToMemberID).Updates(map[string]any{
		"last_deliver_err_at": now,
		"last_deliver_error":  errMsg,
	}).Error
	if status == models.DeliveryFail {
		log.Printf("delivery %s failed permanently: %s", d.ID, errMsg)
	}
}

// KickOnce processes pending deliveries once (for tests).
func (o *Outbox) KickOnce(ctx context.Context) {
	sem := make(chan struct{}, o.Workers)
	var wg sync.WaitGroup
	o.drain(ctx, sem, &wg)
	wg.Wait()
}
