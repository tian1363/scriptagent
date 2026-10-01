package web

import (
	"context"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tian1363/scriptagent/internal/jobs"
)

func (h *Handler) authorizedChatAttachment(w http.ResponseWriter, r *http.Request) *jobs.ChatAttachment {
	conversationID := chi.URLParam(r, "id")
	if _, err := h.store.GetUserChatThread(userIDFromRequest(r), conversationID); err != nil {
		writeError(w, http.StatusNotFound, err)
		return nil
	}
	attachment, err := h.store.GetChatAttachment(conversationID, chi.URLParam(r, "attachmentID"))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return nil
	}
	return attachment
}

func (h *Handler) getChatAttachmentFile(w http.ResponseWriter, r *http.Request) {
	attachment := h.authorizedChatAttachment(w, r)
	if attachment == nil {
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Type", attachment.MimeType)
	w.Header().Set("Content-Disposition", "inline")
	http.ServeFile(w, r, attachment.Path)
}

func (h *Handler) getChatAttachmentPoster(w http.ResponseWriter, r *http.Request) {
	attachment := h.authorizedChatAttachment(w, r)
	if attachment == nil {
		return
	}
	if attachment.Kind != "video" {
		http.NotFound(w, r)
		return
	}
	poster, err := chatVideoPoster(r.Context(), attachment.Path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Type", "image/jpeg")
	http.ServeFile(w, r, poster)
}

func chatVideoPoster(ctx context.Context, videoPath string) (string, error) {
	poster := videoPath + ".jpg"
	if info, err := os.Stat(poster); err == nil && info.Size() > 0 {
		return poster, nil
	}
	tmp, err := os.CreateTemp(filepath.Dir(videoPath), ".chat-poster-*.jpg")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)
	commandCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	command := exec.CommandContext(commandCtx, "ffmpeg", "-nostdin", "-v", "error", "-y", "-ss", "0.3", "-i", videoPath, "-frames:v", "1", "-vf", "scale=480:-2", "-q:v", "4", tmpPath)
	if err := command.Run(); err != nil {
		return "", err
	}
	info, err := os.Stat(tmpPath)
	if err != nil || info.Size() == 0 {
		return "", os.ErrInvalid
	}
	if err := os.Rename(tmpPath, poster); err != nil {
		return "", err
	}
	return poster, nil
}
