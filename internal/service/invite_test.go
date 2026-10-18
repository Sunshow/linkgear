package service

import "testing"

func TestRoutineNameForLinker(t *testing.T) {
	if got := RoutineNameForLinker("bot聊天群"); got != "LinkGear Group bot聊天群" {
		t.Fatalf("got %q", got)
	}
	if got := RoutineNameForLinker("  "); got != "LinkGear Group unnamed" {
		t.Fatalf("empty: %q", got)
	}
}
