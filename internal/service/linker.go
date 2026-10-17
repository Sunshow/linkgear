package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sunshow/linkgear/internal/models"
	"gorm.io/gorm"
)

var (
	ErrNotFound       = errors.New("not found")
	ErrUnauthorized   = errors.New("unauthorized")
	ErrBadRequest     = errors.New("bad request")
	ErrTokenUsed      = errors.New("join token already used")
	ErrTokenExpired   = errors.New("join token expired")
	ErrInvalidBotType = errors.New("invalid bot_type")
)

type Service struct {
	DB        *gorm.DB
	PublicURL string
}

func New(db *gorm.DB, publicURL string) *Service {
	return &Service{DB: db, PublicURL: strings.TrimRight(publicURL, "/")}
}

type CreateLinkerResult struct {
	Linker     models.Linker `json:"linker"`
	AdminToken string        `json:"admin_token"`
}

func (s *Service) CreateLinker(name string) (*CreateLinkerResult, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("%w: name required", ErrBadRequest)
	}
	now := time.Now().UTC()
	l := models.Linker{
		ID:         newID(),
		Name:       name,
		AdminToken: newSecret(),
		CreatedAt:  now,
	}
	if err := s.DB.Create(&l).Error; err != nil {
		return nil, err
	}
	return &CreateLinkerResult{Linker: l, AdminToken: l.AdminToken}, nil
}

func (s *Service) GetLinker(id string) (*models.Linker, error) {
	var l models.Linker
	if err := s.DB.First(&l, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &l, nil
}

func (s *Service) RequireAdmin(linkerID, adminToken string) (*models.Linker, error) {
	l, err := s.GetLinker(linkerID)
	if err != nil {
		return nil, err
	}
	if adminToken == "" || adminToken != l.AdminToken {
		return nil, ErrUnauthorized
	}
	return l, nil
}

func (s *Service) ListMembers(linkerID string) ([]models.Member, error) {
	var ms []models.Member
	if err := s.DB.Where("linker_id = ?", linkerID).Order("joined_at asc").Find(&ms).Error; err != nil {
		return nil, err
	}
	return ms, nil
}

type JoinLinkResult struct {
	JoinToken models.JoinToken `json:"join_token"`
	JoinURL   string           `json:"join_url"`
	JoinPath  string           `json:"join_path"`
}

func (s *Service) CreateJoinLink(linkerID, botType, nickname string, expiresIn time.Duration) (*JoinLinkResult, error) {
	botType = strings.ToLower(strings.TrimSpace(botType))
	nickname = strings.TrimSpace(nickname)
	if !models.ValidBotType(botType) {
		return nil, ErrInvalidBotType
	}
	if nickname == "" {
		return nil, fmt.Errorf("%w: nickname required", ErrBadRequest)
	}
	if expiresIn <= 0 {
		expiresIn = 24 * time.Hour
	}
	if _, err := s.GetLinker(linkerID); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	jt := models.JoinToken{
		ID:        newID(),
		LinkerID:  linkerID,
		Token:     newSecret(),
		BotType:   botType,
		Nickname:  nickname,
		ExpiresAt: now.Add(expiresIn),
		CreatedAt: now,
	}
	if err := s.DB.Create(&jt).Error; err != nil {
		return nil, err
	}
	path := "/api/join/" + jt.Token
	return &JoinLinkResult{
		JoinToken: jt,
		JoinURL:   s.PublicURL + path,
		JoinPath:  path,
	}, nil
}

type JoinResult struct {
	Member      models.Member `json:"member"`
	MemberToken string        `json:"member_token"`
	Linker      models.Linker `json:"linker"`
	SendURL     string        `json:"send_url"`
	HeartbeatURL string       `json:"heartbeat_url"`
}

func (s *Service) RedeemJoin(token, webhookURL, webhookAuthHeader string) (*JoinResult, error) {
	webhookURL = strings.TrimSpace(webhookURL)
	if webhookURL == "" {
		return nil, fmt.Errorf("%w: webhook_url required", ErrBadRequest)
	}
	var jt models.JoinToken
	if err := s.DB.Where("token = ?", token).First(&jt).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	now := time.Now().UTC()
	if jt.UsedAt != nil {
		return nil, ErrTokenUsed
	}
	if now.After(jt.ExpiresAt) {
		return nil, ErrTokenExpired
	}
	linker, err := s.GetLinker(jt.LinkerID)
	if err != nil {
		return nil, err
	}

	member := models.Member{
		ID:                newID(),
		LinkerID:          jt.LinkerID,
		BotType:           jt.BotType,
		Nickname:          jt.Nickname,
		WebhookURL:        webhookURL,
		WebhookAuthHeader: strings.TrimSpace(webhookAuthHeader),
		MemberToken:       newSecret(),
		Status:            models.MemberOnline,
		JoinedAt:          now,
		LastHeartbeatAt:   &now,
	}

	err = s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&member).Error; err != nil {
			return err
		}
		used := now
		mid := member.ID
		jt.UsedAt = &used
		jt.UsedByMemberID = &mid
		return tx.Model(&jt).Updates(map[string]any{
			"used_at":            used,
			"used_by_member_id": mid,
		}).Error
	})
	if err != nil {
		return nil, err
	}

	return &JoinResult{
		Member:       member,
		MemberToken:  member.MemberToken,
		Linker:       *linker,
		SendURL:      s.PublicURL + "/api/linkers/" + linker.ID + "/messages",
		HeartbeatURL: s.PublicURL + "/api/members/" + member.ID + "/heartbeat",
	}, nil
}

func (s *Service) MemberByToken(token string) (*models.Member, error) {
	var m models.Member
	if err := s.DB.Where("member_token = ?", token).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUnauthorized
		}
		return nil, err
	}
	return &m, nil
}

func (s *Service) Heartbeat(memberID, memberToken string) (*models.Member, error) {
	m, err := s.MemberByToken(memberToken)
	if err != nil {
		return nil, err
	}
	if m.ID != memberID {
		return nil, ErrUnauthorized
	}
	now := time.Now().UTC()
	if err := s.DB.Model(m).Updates(map[string]any{
		"status":            models.MemberOnline,
		"last_heartbeat_at": now,
	}).Error; err != nil {
		return nil, err
	}
	m.Status = models.MemberOnline
	m.LastHeartbeatAt = &now
	return m, nil
}

func (s *Service) UpdateWebhook(memberID, memberToken, webhookURL, webhookAuthHeader string) (*models.Member, error) {
	m, err := s.MemberByToken(memberToken)
	if err != nil {
		return nil, err
	}
	if m.ID != memberID {
		return nil, ErrUnauthorized
	}
	webhookURL = strings.TrimSpace(webhookURL)
	if webhookURL == "" {
		return nil, fmt.Errorf("%w: webhook_url required", ErrBadRequest)
	}
	upd := map[string]any{
		"webhook_url": webhookURL,
	}
	if webhookAuthHeader != "" || true {
		upd["webhook_auth_header"] = strings.TrimSpace(webhookAuthHeader)
	}
	if err := s.DB.Model(m).Updates(upd).Error; err != nil {
		return nil, err
	}
	m.WebhookURL = webhookURL
	m.WebhookAuthHeader = strings.TrimSpace(webhookAuthHeader)
	return m, nil
}

func (s *Service) ListJoinTokens(linkerID string) ([]models.JoinToken, error) {
	var list []models.JoinToken
	if err := s.DB.Where("linker_id = ?", linkerID).Order("created_at desc").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}
