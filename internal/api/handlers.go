package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sunshow/linkgear/internal/service"
)

type Server struct {
	Svc *service.Service
}

func (s *Server) CreateLinker(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	res, err := s.Svc.CreateLinker(req.Name)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, res)
}

func (s *Server) GetLinker(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.Svc.RequireAdmin(id, adminToken(r)); err != nil {
		writeErr(w, err)
		return
	}
	l, err := s.Svc.GetLinker(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	members, err := s.Svc.ListMembers(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"linker":  l,
		"members": members,
	})
}

func (s *Server) CreateJoinLink(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.Svc.RequireAdmin(id, adminToken(r)); err != nil {
		writeErr(w, err)
		return
	}
	var req struct {
		BotType   string `json:"bot_type"`
		Nickname  string `json:"nickname"`
		ExpiresIn string `json:"expires_in"` // e.g. "24h"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	exp := 24 * time.Hour
	if req.ExpiresIn != "" {
		if d, err := time.ParseDuration(req.ExpiresIn); err == nil {
			exp = d
		}
	}
	res, err := s.Svc.CreateJoinLink(id, req.BotType, req.Nickname, exp)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, res)
}

func (s *Server) Join(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	var req struct {
		WebhookURL        string `json:"webhook_url"`
		WebhookAuthHeader string `json:"webhook_auth_header"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	res, err := s.Svc.RedeemJoin(token, req.WebhookURL, req.WebhookAuthHeader)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, res)
}

func (s *Server) SendMessage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	token := bearer(r)
	if token == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing bearer token"})
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "read body"})
		return
	}
	var in service.SendMessageInput
	ct := r.Header.Get("Content-Type")
	if strings.Contains(ct, "application/json") || (len(raw) > 0 && (raw[0] == '{' || raw[0] == '[')) {
		var envelope struct {
			Text    string          `json:"text"`
			Body    json.RawMessage `json:"body"`
			Message json.RawMessage `json:"message"`
			Task    string          `json:"task"`
			Msg     string          `json:"msg"`
		}
		if err := json.Unmarshal(raw, &envelope); err == nil {
			switch {
			case len(envelope.Body) > 0:
				in.Raw = envelope.Body
			case len(envelope.Message) > 0:
				in.Raw = envelope.Message
			case envelope.Text != "":
				in.Text = envelope.Text
			case envelope.Task != "":
				in.Text = envelope.Task
			case envelope.Msg != "":
				in.Text = envelope.Msg
			default:
				in.Raw = raw
			}
		} else {
			in.Text = string(raw)
		}
	} else {
		in.Text = strings.TrimSpace(string(raw))
	}

	res, err := s.Svc.SendMessage(id, token, in)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, res)
}

func (s *Server) ListMessages(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	// admin or member
	ok := false
	if _, err := s.Svc.RequireAdmin(id, adminToken(r)); err == nil {
		ok = true
	} else if tok := bearer(r); tok != "" {
		if m, err := s.Svc.MemberByToken(tok); err == nil && m.LinkerID == id {
			ok = true
		}
	}
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	msgs, err := s.Svc.ListMessages(id, limit)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": msgs})
}

func (s *Server) Heartbeat(w http.ResponseWriter, r *http.Request) {
	mid := r.PathValue("id")
	token := bearer(r)
	m, err := s.Svc.Heartbeat(mid, token)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) UpdateWebhook(w http.ResponseWriter, r *http.Request) {
	mid := r.PathValue("id")
	token := bearer(r)
	var req struct {
		WebhookURL        string `json:"webhook_url"`
		WebhookAuthHeader string `json:"webhook_auth_header"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	m, err := s.Svc.UpdateWebhook(mid, token, req.WebhookURL, req.WebhookAuthHeader)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":          m.ID,
		"webhook_url": m.WebhookURL,
		"status":      m.Status,
	})
}

func (s *Server) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "linkgear"})
}

func (s *Server) RenameMember(w http.ResponseWriter, r *http.Request) {
	mid := r.PathValue("id")
	var req struct {
		Nickname string `json:"nickname"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	m, err := s.Svc.GetMember(mid)
	if err != nil {
		writeErr(w, err)
		return
	}
	authorized := false
	if _, err := s.Svc.RequireAdmin(m.LinkerID, adminToken(r)); err == nil {
		authorized = true
	} else if tok := bearer(r); tok != "" {
		if self, err := s.Svc.MemberByToken(tok); err == nil && self.ID == mid {
			authorized = true
		}
	}
	if !authorized {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	updated, err := s.Svc.RenameMember(mid, req.Nickname)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}
