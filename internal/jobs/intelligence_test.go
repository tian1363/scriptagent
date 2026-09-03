package jobs

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestIntelligenceDemoPromotesOnlyConfirmedSignalToContext(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	user, err := store.CreateUser(CreateUserInput{Email: "demo@example.com", Name: "Demo", Role: "owner", Status: "active", PasswordHash: "x"})
	if err != nil {
		t.Fatal(err)
	}
	space, err := store.CreateSpace(CreateSpaceInput{Title: "防晒实验"})
	if err != nil {
		t.Fatal(err)
	}
	if err = store.ClaimResource(user.ID, "space", space.ID); err != nil {
		t.Fatal(err)
	}
	if err = store.SeedIntelligenceDemo(user.ID, space.ID); err != nil {
		t.Fatal(err)
	}
	dashboard, err := store.IntelligenceDashboard(user.ID, space.ID)
	if err != nil || len(dashboard.Signals) < 4 {
		t.Fatalf("dashboard=%+v err=%v", dashboard, err)
	}
	if got := store.CreativeMemoryContext(user.ID, space.ID, 6); got != "" {
		t.Fatalf("unconfirmed signals leaked into context: %s", got)
	}
	memory, err := store.PromoteSignalToMemory(user.ID, dashboard.Signals[0].ID, space.ID)
	if err != nil {
		t.Fatal(err)
	}
	context := store.CreativeMemoryContext(user.ID, space.ID, 6)
	if !strings.Contains(context, memory.Title) || !strings.Contains(context, "置信度") {
		t.Fatalf("missing confirmed evidence: %s", context)
	}
	updated, err := store.UpdateCreativeMemory(user.ID, memory.ID, UpdateCreativeMemoryInput{Title: "更新后的结论", Finding: "用新证据校正后的创意判断。", Confidence: .66})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != "更新后的结论" || updated.Confidence != .66 {
		t.Fatalf("unexpected updated memory: %+v", updated)
	}
	context = store.CreativeMemoryContext(user.ID, space.ID, 6)
	if !strings.Contains(context, "更新后的结论") || strings.Contains(context, memory.Title) {
		t.Fatalf("memory context was not updated: %s", context)
	}
	other, err := store.CreateUser(CreateUserInput{Email: "other@example.com", Name: "Other", Role: "member", Status: "active", PasswordHash: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteCreativeMemory(other.ID, memory.ID); err == nil {
		t.Fatal("another user deleted the memory")
	}
	if err := store.DeleteCreativeMemory(user.ID, memory.ID); err != nil {
		t.Fatal(err)
	}
	if got := store.CreativeMemoryContext(user.ID, space.ID, 6); got != "" {
		t.Fatalf("deleted memory remains in context: %s", got)
	}
}
