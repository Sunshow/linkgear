package api

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
	"time"

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

	static, err := fs.Sub(webFS, "static")
	if err == nil {
		mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(static))))
	}

	mux.HandleFunc("GET /", ui.Home)
	mux.HandleFunc("POST /ui/linkers", ui.CreateLinkerForm)
	mux.HandleFunc("GET /ui/linkers/{id}", ui.LinkerPage)
	mux.HandleFunc("POST /ui/linkers/{id}/join-links", ui.CreateJoinLinkForm)
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

	type joinView struct {
		models.JoinToken
		JoinURL string
		Unused  bool
	}
	jv := make([]joinView, 0, len(joins))
	for _, j := range joins {
		jv = append(jv, joinView{
			JoinToken: j,
			JoinURL:   u.PublicURL + "/api/join/" + j.Token,
			Unused:    j.UsedAt == nil,
		})
	}

	_ = u.tmpl.ExecuteTemplate(w, "linker.html", map[string]any{
		"Linker":     l,
		"AdminToken": token,
		"Members":    members,
		"JoinTokens": jv,
		"Messages":   msgs,
		"PublicURL":  u.PublicURL,
		"BotTypes":   []string{models.BotTypeGrokBot, models.BotTypeDots, models.BotTypeOther},
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
	_, err := u.Svc.CreateJoinLink(id, botType, nickname, exp)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	http.Redirect(w, r, "/ui/linkers/"+id+"?admin_token="+token, http.StatusSeeOther)
}
