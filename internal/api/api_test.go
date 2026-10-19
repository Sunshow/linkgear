package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sunshow/linkgear/internal/api"
	"github.com/sunshow/linkgear/internal/db"
	"github.com/sunshow/linkgear/internal/service"
)

func testMux(t *testing.T) (*http.ServeMux, *service.Service) {
	t.Helper()
	dsn := "sqlite://file:linkgear_api_" + t.Name() + "?mode=memory&cache=shared"
	gdb, _, err := db.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.MigrateDSN(dsn); err != nil {
		t.Fatal(err)
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	svc := service.New(gdb, "http://example.test")
	apiSrv := &api.Server{Svc: svc}
	ui, err := api.NewUI(svc, "http://example.test")
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	api.Mount(mux, apiSrv, ui)
	return mux, svc
}

func TestAPIJoinAndSend(t *testing.T) {
	mux, _ := testMux(t)

	// create linker
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/linkers", bytes.NewBufferString(`{"name":"room1"}`))
	mux.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("create linker: %d %s", w.Code, w.Body.String())
	}
	var created struct {
		Linker struct {
			ID string `json:"id"`
		} `json:"linker"`
		AdminToken string `json:"admin_token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	// join link
	w = httptest.NewRecorder()
	body := `{"bot_type":"grokbot","nickname":"IT","expires_in":"1h"}`
	req = httptest.NewRequest(http.MethodPost, "/api/linkers/"+created.Linker.ID+"/join-links", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+created.AdminToken)
	mux.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("join-links: %d %s", w.Code, w.Body.String())
	}
	var jl struct {
		JoinToken struct {
			Token string `json:"token"`
		} `json:"join_token"`
		JoinURL    string `json:"join_url"`
		InviteText string `json:"invite_text"`
		DocsURL    string `json:"docs_url"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &jl); err != nil {
		t.Fatal(err)
	}
	if jl.InviteText == "" || !bytes.Contains([]byte(jl.InviteText), []byte(jl.JoinURL)) {
		t.Fatalf("invite_text missing join url: %q", jl.InviteText)
	}
	if !bytes.Contains([]byte(jl.InviteText), []byte("LinkGear Group room1")) {
		t.Fatalf("invite_text missing LinkGear Group routine name: %q", jl.InviteText)
	}
	if jl.DocsURL != "http://example.test/docs/bot-integration.md" {
		t.Fatalf("docs_url: %q", jl.DocsURL)
	}

	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer hook.Close()

	w = httptest.NewRecorder()
	joinBody, _ := json.Marshal(map[string]string{"webhook_url": hook.URL})
	req = httptest.NewRequest(http.MethodPost, "/api/join/"+jl.JoinToken.Token, bytes.NewReader(joinBody))
	mux.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("join: %d %s", w.Code, w.Body.String())
	}
	var joined struct {
		MemberToken string `json:"member_token"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &joined)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/linkers/"+created.Linker.ID+"/messages", bytes.NewBufferString(`{"text":"ping"}`))
	req.Header.Set("Authorization", "Bearer "+joined.MemberToken)
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, req)
	if w.Code != 202 {
		t.Fatalf("send: %d %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/healthz", nil)
	mux.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("health: %d", w.Code)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/docs/bot-integration.md", nil)
	mux.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("docs: %d %s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("Grok Bot")) {
		t.Fatalf("docs body unexpected")
	}
}

func TestRenameMemberAdminAndSelf(t *testing.T) {
	mux, _ := testMux(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/linkers", bytes.NewBufferString(`{"name":"rename-room"}`))
	mux.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var created struct {
		Linker     struct{ ID string `json:"id"` } `json:"linker"`
		AdminToken string                         `json:"admin_token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	w = httptest.NewRecorder()
	body := `{"bot_type":"grokbot","nickname":"Alice","expires_in":"1h"}`
	req = httptest.NewRequest(http.MethodPost, "/api/linkers/"+created.Linker.ID+"/join-links", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+created.AdminToken)
	mux.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("join-links: %d %s", w.Code, w.Body.String())
	}
	var jl struct {
		JoinToken struct {
			Token string `json:"token"`
		} `json:"join_token"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &jl)

	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer hook.Close()

	w = httptest.NewRecorder()
	joinBody, _ := json.Marshal(map[string]string{"webhook_url": hook.URL})
	req = httptest.NewRequest(http.MethodPost, "/api/join/"+jl.JoinToken.Token, bytes.NewReader(joinBody))
	mux.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("join: %d %s", w.Code, w.Body.String())
	}
	var joined struct {
		MemberToken string `json:"member_token"`
		Member      struct {
			ID       string `json:"id"`
			Nickname string `json:"nickname"`
		} `json:"member"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &joined); err != nil {
		t.Fatal(err)
	}

	// admin rename
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPatch, "/api/members/"+joined.Member.ID+"/nickname",
		bytes.NewBufferString(`{"nickname":"AdminRenamed"}`))
	req.Header.Set("Authorization", "Bearer "+created.AdminToken)
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("admin rename: %d %s", w.Code, w.Body.String())
	}
	var renamed struct {
		Nickname string `json:"nickname"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &renamed)
	if renamed.Nickname != "AdminRenamed" {
		t.Fatalf("admin rename nickname=%q", renamed.Nickname)
	}

	// self rename
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPatch, "/api/members/"+joined.Member.ID+"/nickname",
		bytes.NewBufferString(`{"nickname":"SelfRenamed"}`))
	req.Header.Set("Authorization", "Bearer "+joined.MemberToken)
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("self rename: %d %s", w.Code, w.Body.String())
	}
	_ = json.Unmarshal(w.Body.Bytes(), &renamed)
	if renamed.Nickname != "SelfRenamed" {
		t.Fatalf("self rename nickname=%q", renamed.Nickname)
	}

	// wrong token
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPatch, "/api/members/"+joined.Member.ID+"/nickname",
		bytes.NewBufferString(`{"nickname":"Nope"}`))
	req.Header.Set("Authorization", "Bearer wrong-token")
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("want 401, got %d %s", w.Code, w.Body.String())
	}

	// empty nickname
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPatch, "/api/members/"+joined.Member.ID+"/nickname",
		bytes.NewBufferString(`{"nickname":"  "}`))
	req.Header.Set("Authorization", "Bearer "+joined.MemberToken)
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("want 400 empty, got %d %s", w.Code, w.Body.String())
	}

	// UI rename form
	w = httptest.NewRecorder()
	form := "admin_token=" + created.AdminToken + "&nickname=UIRenamed"
	req = httptest.NewRequest(http.MethodPost, "/ui/linkers/"+created.Linker.ID+"/members/"+joined.Member.ID+"/rename",
		bytes.NewBufferString(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	mux.ServeHTTP(w, req)
	if w.Code != 303 {
		t.Fatalf("ui rename: %d %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/linkers/"+created.Linker.ID, nil)
	req.Header.Set("Authorization", "Bearer "+created.AdminToken)
	mux.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("get linker: %d %s", w.Code, w.Body.String())
	}
	var detail struct {
		Members []struct {
			ID       string `json:"id"`
			Nickname string `json:"nickname"`
		} `json:"members"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &detail)
	if len(detail.Members) != 1 || detail.Members[0].Nickname != "UIRenamed" {
		t.Fatalf("after ui rename: %+v", detail.Members)
	}
}
