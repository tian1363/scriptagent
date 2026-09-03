package jobs

import (
	"path/filepath"
	"testing"
)

func TestProactiveSuggestionsDeduplicateDismissAndIsolateUsers(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "suggestions.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	user, err := store.CreateUser(CreateUserInput{Email: "one@example.com", PasswordHash: "hash"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := store.CreateUser(CreateUserInput{Email: "two@example.com", PasswordHash: "hash"})
	if err != nil {
		t.Fatal(err)
	}
	space, err := store.CreateSpace(CreateSpaceInput{Title: "UGC launch"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ClaimResource(user.ID, "space", space.ID); err != nil {
		t.Fatal(err)
	}

	items, err := store.RefreshProactiveSuggestions(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].TriggerType != "missing_product" {
		t.Fatalf("unexpected suggestions: %+v", items)
	}
	again, err := store.RefreshProactiveSuggestions(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 1 || again[0].ID != items[0].ID {
		t.Fatalf("refresh should deduplicate: %+v", again)
	}
	if _, err := store.UpdateProactiveSuggestionStatus(other.ID, items[0].ID, "dismissed"); err == nil {
		t.Fatal("another user must not update suggestion")
	}
	if _, err := store.UpdateProactiveSuggestionStatus(user.ID, items[0].ID, "dismissed"); err != nil {
		t.Fatal(err)
	}
	afterDismiss, err := store.RefreshProactiveSuggestions(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(afterDismiss) != 0 {
		t.Fatalf("dismissed suggestion returned: %+v", afterDismiss)
	}
	otherItems, err := store.RefreshProactiveSuggestions(other.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(otherItems) != 0 {
		t.Fatalf("resource leaked to other user: %+v", otherItems)
	}
}
