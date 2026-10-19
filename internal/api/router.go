package api

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
	"time"

	appdocs "github.com/sunshow/linkgear/docs"
	"github.com/sunshow/linkgear/internal/models"
	"github.com/sunshow/linkgear/internal/service"
)

//go:embed templates/*.html static/*
var webFS embed.FS

type UIServer struct {
	Svc       *service.Service
	PublicURL string
	tmpl      *template.Template
}

func NewUI(svc *service.Service, publicURL string) (*UIServer, error) {
	funcs := template.FuncMap{
		"fmtTime": func(t *time.Time) string {
			if t == nil {
				return "—"
			}
			return t.Local().Format("2006-01-02 15:04:05")
		},
		"fmtTimeVal": func(t time.Time) string {
			return t.Local().Format("2006-01-02 15:04:05")
		},
		"trunc": func(s string, n int) string {
			if len(s) <= n {
				return s
			}
			return s[:n] + "…"
		},
	}
	t, err := template.New("").Funcs(funcs).ParseFS(webFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &UIServer{Svc: svc, PublicURL: publicURL, tmpl: t}, nil
}

func Mount(mux *http.ServeMux, api *Server, ui *UIServer) {
	mux.HandleFunc("GET /healthz", api.Health)
	mux.HandleFunc("POST /api/linkers", api.CreateLinker)
	mux.HandleFunc("GET /api/linkers/{id}", api.GetLinker)
	mux.HandleFunc("POST /api/linkers/{id}/join-links", api.CreateJoinLink)
	mux.HandleFunc("POST /api/join/{token}", api.Join)
	mux.HandleFunc("POST /api/linkers/{id}/messages", api.SendMessage)
	mux.HandleFunc("GET /api/linkers/{id}/messages", api.ListMessages)
	mux.HandleFunc("POST /api/members/{id}/heartbeat", api.Heartbeat)
	mux.HandleFunc("PUT /api/members/{id}/webhook", api.UpdateWebhook)
	mux.HandleFunc("PATCH /api/members/{id}/nickname", api.RenameMember)

	static, err := fs.Sub(webFS, "static")
	if err == nil {
		mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(static))))
	}

	// Hosted docs (same content as GitHub docs/)
	docsRoot := http.FS(appdocs.FS)
	mux.Handle("GET /docs/", http.StripPrefix("/docs/", http.FileServer(docsRoot)))
	mux.HandleFunc("GET /docs", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/docs/bot-integration.md", http.StatusFound)
	})

	mux.HandleFunc("GET /", ui.Home)
	mux.HandleFunc("POST /ui/linkers", ui.CreateLinkerForm)
	mux.HandleFunc("GET /ui/linkers/{id}", ui.LinkerPage)
	mux.HandleFunc("POST /ui/linkers/{id}/join-links", ui.CreateJoinLinkForm)
	mux.HandleFunc("POST /ui/linkers/{id}/members/{memberID}/rename", ui.RenameMemberForm)
}

func (u *UIServer) Home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	_ = u.tmpl.ExecuteTemplate(w, "home.html", map[string]any{
		"PublicURL": u.PublicURL,
	})
}

func (u *UIServer) CreateLinkerForm(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	res, err := u.Svc.CreateLinker(name)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	http.Redirect(w, r, "/ui/linkers/"+res.Linker.ID+"?admin_token="+res.AdminToken, http.StatusSeeOther)
}

func (u *UIServer) LinkerPage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	token := r.URL.Query().Get("admin_token")
	l, err := u.Svc.RequireAdmin(id, token)
	if err != nil {
		http.Error(w, "未授权或 Linker 不存在（请带上 admin_token）", http.StatusUnauthorized)
		return
	}
	members, _ := u.Svc.ListMembers(id)
	joins, _ := u.Svc.ListJoinTokens(id)
	msgs, _ := u.Svc.ListMessages(id, 30)
	highlight := r.URL.Query().Get("new_join")

	type joinView struct {
		models.JoinToken
		JoinURL    string
		InviteText string
		Unused     bool
		Highlight  bool
	}
	jv := make([]joinView, 0, len(joins))
	for _, j := range joins {
		joinURL := u.PublicURL + "/api/join/" + j.Token
		unused := j.UsedAt == nil
		jv = append(jv, joinView{
			JoinToken:  j,
			JoinURL:    joinURL,
			InviteText: u.Svc.BuildInviteText(l, &j, joinURL),
			Unused:     unused,
			Highlight:  unused && (highlight == "" || highlight == j.Token || highlight == j.ID),
		})
	}
	// If new_join specified, only highlight that one
	if highlight != "" {
		for i := range jv {
			jv[i].Highlight = jv[i].Unused && (jv[i].Token == highlight || jv[i].ID == highlight)
		}
	} else {
		// highlight only the newest unused (first in desc order)
		seen := false
		for i := range jv {
			if jv[i].Unused && !seen {
				jv[i].Highlight = true
				seen = true
			} else {
				jv[i].Highlight = false
			}
		}
	}

	_ = u.tmpl.ExecuteTemplate(w, "linker.html", map[string]any{
		"Linker":     l,
		"AdminToken": token,
		"Members":    members,
		"JoinTokens": jv,
		"Messages":   msgs,
		"PublicURL":  u.PublicURL,
		"DocsURL":    u.PublicURL + "/docs/bot-integration.md",
		"BotTypes":   []string{models.BotTypeGrokBot, models.BotTypeDots, models.BotTypeOther},
		"NewJoin":    highlight,
	})
}

func (u *UIServer) CreateJoinLinkForm(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	token := r.FormValue("admin_token")
	if _, err := u.Svc.RequireAdmin(id, token); err != nil {
		http.Error(w, "unauthorized", 401)
		return
	}
	botType := r.FormValue("bot_type")
	nickname := r.FormValue("nickname")
	exp := 24 * time.Hour
	if v := r.FormValue("expires_in"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			exp = d
		}
	}
	res, err := u.Svc.CreateJoinLink(id, botType, nickname, exp)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	http.Redirect(w, r, "/ui/linkers/"+id+"?admin_token="+token+"&new_join="+res.JoinToken.Token, http.StatusSeeOther)
}

func (u *UIServer) RenameMemberForm(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	memberID := r.PathValue("memberID")
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	token := r.FormValue("admin_token")
	if _, err := u.Svc.RequireAdmin(id, token); err != nil {
		http.Error(w, "unauthorized", 401)
		return
	}
	m, err := u.Svc.GetMember(memberID)
	if err != nil {
		http.Error(w, err.Error(), 404)
		return
	}
	if m.LinkerID != id {
		http.Error(w, "member not in this linker", 404)
		return
	}
	if _, err := u.Svc.RenameMember(memberID, r.FormValue("nickname")); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	http.Redirect(w, r, "/ui/linkers/"+id+"?admin_token="+token, http.StatusSeeOther)
}
