package service_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sunshow/linkgear/internal/db"
	"github.com/sunshow/linkgear/internal/models"
	"github.com/sunshow/linkgear/internal/service"
	"github.com/sunshow/linkgear/internal/worker"
)

func setup(t *testing.T) *service.Service {
	t.Helper()
	// unique memory db per test
	dsn := "sqlite://file:linkgear_test_" + t.Name() + "?mode=memory&cache=shared"
	gdb, _, err := db.Open(dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.MigrateDSN(dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		t.Fatalf("sqldb: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return service.New(gdb, "http://localhost:8080")
}

func TestCreateJoinSendFanout(t *testing.T) {
	svc := setup(t)

	created, err := svc.CreateLinker("test-room")
	if err != nil {
		t.Fatal(err)
	}

	jlA, err := svc.CreateJoinLink(created.Linker.ID, models.BotTypeGrokBot, "bot-a", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	jlB, err := svc.CreateJoinLink(created.Linker.ID, models.BotTypeGrokBot, "bot-b", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	var gotMu sync.Mutex
	var gotBodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotMu.Lock()
		gotBodies = append(gotBodies, string(b))
		gotMu.Unlock()
		if r.Header.Get("Authorization") != "Bearer webhook-secret-b" {
			t.Errorf("missing/wrong auth: %q", r.Header.Get("Authorization"))
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`ok`))
	}))
	defer srv.Close()

	a, err := svc.RedeemJoin(jlA.JoinToken.Token, "http://127.0.0.1:9/unused", "")
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.RedeemJoin(jlB.JoinToken.Token, srv.URL, "Bearer webhook-secret-b")
	if err != nil {
		t.Fatal(err)
	}

	// reuse token should fail
	if _, err := svc.RedeemJoin(jlA.JoinToken.Token, "http://x", ""); err == nil {
		t.Fatal("expected token used error")
	}

	res, err := svc.SendMessage(created.Linker.ID, a.MemberToken, service.SendMessageInput{
		Text: "hello from a",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Deliveries) != 1 {
		t.Fatalf("want 1 delivery, got %d", len(res.Deliveries))
	}

	outbox := worker.NewOutbox(svc.DB, 2, time.Millisecond)
	outbox.KickOnce(context.Background())

	deadline := time.Now().Add(2 * time.Second)
	for {
		gotMu.Lock()
		n := len(gotBodies)
		gotMu.Unlock()
		if n >= 1 || time.Now().After(deadline) {
			break
		}
		time.Sleep(20 * time.Millisecond)
		outbox.KickOnce(context.Background())
	}
	gotMu.Lock()
	defer gotMu.Unlock()
	if len(gotBodies) != 1 {
		t.Fatalf("want 1 webhook POST, got %d", len(gotBodies))
	}
	var env worker.Envelope
	if err := json.Unmarshal([]byte(gotBodies[0]), &env); err != nil {
		t.Fatal(err)
	}
	if env.From.Nickname != "bot-a" {
		t.Fatalf("from=%s", env.From.Nickname)
	}
	if env.LinkerID != created.Linker.ID {
		t.Fatalf("linker_id mismatch")
	}

	members, err := svc.ListMembers(created.Linker.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 2 {
		t.Fatalf("members=%d", len(members))
	}
	var bMember *models.Member
	for i := range members {
		if members[i].ID == b.Member.ID {
			bMember = &members[i]
		}
	}
	if bMember == nil || bMember.LastDeliverOKAt == nil {
		t.Fatalf("bot-b should have last_deliver_ok_at set: %+v", bMember)
	}
}

func TestInvalidBotType(t *testing.T) {
	svc := setup(t)
	c, err := svc.CreateLinker("x")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateJoinLink(c.Linker.ID, "chatgpt", "n", time.Hour); err == nil {
		t.Fatal("expected invalid bot type")
	}
}

func TestAdminAuth(t *testing.T) {
	svc := setup(t)
	c, err := svc.CreateLinker("x")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RequireAdmin(c.Linker.ID, "wrong"); err == nil {
		t.Fatal("expected unauthorized")
	}
	if _, err := svc.RequireAdmin(c.Linker.ID, c.AdminToken); err != nil {
		t.Fatal(err)
	}
}

func TestRenameMember(t *testing.T) {
	svc := setup(t)
	c, err := svc.CreateLinker("room")
	if err != nil {
		t.Fatal(err)
	}
	jl, err := svc.CreateJoinLink(c.Linker.ID, models.BotTypeGrokBot, "old-name", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	joined, err := svc.RedeemJoin(jl.JoinToken.Token, "http://127.0.0.1:9/hook", "")
	if err != nil {
		t.Fatal(err)
	}

	updated, err := svc.RenameMember(joined.Member.ID, "  new-name  ")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Nickname != "new-name" {
		t.Fatalf("nickname=%q", updated.Nickname)
	}
	got, err := svc.GetMember(joined.Member.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Nickname != "new-name" {
		t.Fatalf("persisted nickname=%q", got.Nickname)
	}

	if _, err := svc.RenameMember(joined.Member.ID, "   "); err == nil {
		t.Fatal("expected empty nickname error")
	}
	long := strings.Repeat("啊", service.MaxNicknameLen+1)
	if _, err := svc.RenameMember(joined.Member.ID, long); err == nil {
		t.Fatal("expected too-long nickname error")
	}
	if _, err := svc.RenameMember("missing-id", "x"); err == nil {
		t.Fatal("expected not found")
	}
}
