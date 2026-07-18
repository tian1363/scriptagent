package storage

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
)

const (
	MaxVideoBytes          = 500 * 1024 * 1024
	MaxMarkdownBytes       = 2 * 1024 * 1024
	MaxChatAttachmentBytes = 20 * 1024 * 1024
)

type LocalStore struct {
	root string
}

func NewLocalStore(root string) *LocalStore {
	return &LocalStore{root: root}
}

func (s *LocalStore) SaveUpload(file multipart.File, header *multipart.FileHeader, kind string) (string, error) {
	return s.SaveUserUpload("", file, header, kind)
}

func (s *LocalStore) SaveUserUpload(userID string, file multipart.File, header *multipart.FileHeader, kind string) (string, error) {
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if kind == "video" && ext != ".mp4" && ext != ".mov" && ext != ".webm" {
		return "", errors.New("unsupported video type")
	}
	if kind == "chat" && !isChatAttachmentExt(ext) {
		return "", errors.New("unsupported chat attachment type")
	}
	if kind == "creative" && !isChatAttachmentExt(ext) {
		return "", errors.New("unsupported creative material type")
	}
	if kind == "markdown" && ext != ".md" && ext != ".markdown" {
		return "", errors.New("unsupported markdown type")
	}

	limit := int64(MaxVideoBytes)
	if kind == "markdown" {
		limit = MaxMarkdownBytes
	}
	if kind == "chat" || kind == "creative" {
		limit = MaxChatAttachmentBytes
	}
	if header.Size > limit {
		return "", errors.New("file is too large")
	}

	name := randomName() + ext
	dir := filepath.Join(s.root, kind)
	if strings.TrimSpace(userID) != "" {
		dir = filepath.Join(s.root, "users", safeSegment(userID), kind)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, name)
	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", err
	}
	defer out.Close()

	if _, err := io.Copy(out, io.LimitReader(file, limit+1)); err != nil {
		return "", err
	}
	return path, nil
}

func safeSegment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "anonymous"
	}
	var b strings.Builder
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "user"
	}
	return b.String()
}

func isChatAttachmentExt(ext string) bool {
	switch ext {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif", ".mp4", ".mov", ".webm":
		return true
	default:
		return false
	}
}

func randomName() string {
	var buf [12]byte
	if _, err := rand.Read(buf[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(buf[:])
}
