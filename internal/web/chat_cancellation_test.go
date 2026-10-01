package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	chatpkg "github.com/tian1363/scriptagent/internal/chat"
	"github.com/tian1363/scriptagent/internal/jobs"
	"github.com/tian1363/scriptagent/internal/storage"
	"github.com/tian1363/scriptagent/internal/userctx"
)

type disconnectChatStub struct {
	ChatResponder
	store    *jobs.Store
	started  chan struct{}
	release  chan struct{}
	finished chan error
}

func (s *disconnectChatStub) SendWithAttachments(ctx context.Context, conversationID, content, productID string, _ []chatpkg.AttachmentInput) (*jobs.ChatThread, error) {
	close(s.started)
	<-s.release
	if err := ctx.Err(); err != nil {
		s.finished <- err
		return nil, err
	}
	_, err := s.store.AddChatMessage(conversationID, "assistant", "任务已完成")
	s.finished <- err
	if err != nil {
		return nil, err
	}
	return s.store.GetUserChatThread(userctx.UserID(ctx), conversationID)
}

func TestChatRunFinishesAfterClientDisconnect(t *testing.T) {
	dir := t.TempDir()
	store, err := jobs.OpenStore(filepath.Join(dir, "chat.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	stub := &disconnectChatStub{store: store, started: make(chan struct{}), release: make(chan struct{}), finished: make(chan error, 1)}
	h := NewHandler(Config{}, store, storage.NewLocalStore(filepath.Join(dir, "uploads")), nil, nil, stub, nil)
	server := httptest.NewServer(h.Routes())
	defer server.Close()
	client := clientWithJar(t)
	user := registerTestUser(t, client, server.URL, "disconnect@example.com", "")
	conversation, err := store.CreateChatConversation("test")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ClaimResource(user.ID, "chat", conversation.ID); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/api/chats/"+conversation.ID+"/messages", strings.NewReader(`{"content":"继续执行"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	clientDone := make(chan struct{})
	go func() {
		response, _ := client.Do(req)
		if response != nil {
			response.Body.Close()
		}
		close(clientDone)
	}()
	select {
	case <-stub.started:
	case <-time.After(3 * time.Second):
		t.Fatal("chat run did not start")
	}
	cancel()
	<-clientDone
	close(stub.release)
	select {
	case err := <-stub.finished:
		if err != nil {
			t.Fatalf("chat run stopped after disconnect: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("chat run did not finish")
	}
	thread, err := store.GetUserChatThread(user.ID, conversation.ID)
	if err != nil || len(thread.Messages) != 1 || thread.Messages[0].Content != "任务已完成" {
		t.Fatalf("answer was not persisted: thread=%+v err=%v", thread, err)
	}
}
