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
	MaxDocumentBytes       = 20 * 1024 * 1024
	MaxAssetBytes          = 100 * 1024 * 1024
	MaxChatAttachmentBytes = 20 * 1024 * 1024
)

type LocalStore struct {
	root string
}

func NewLocalStore(root string) *LocalStore {
	return &LocalStore{root: root}
}

func (s *LocalStore) SaveGeneratedVideo(id string, input io.Reader) (string, error) {
	dir := filepath.Join(s.root, "generated")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, id+".mp4")
	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return "", err
	}
	defer out.Close()
	_, err = io.Copy(out, io.LimitReader(input, MaxVideoBytes+1))
	return path, err
}

func (s *LocalStore) SaveUpload(file multipart.File, header *multipart.FileHeader, kind string) (string, error) {
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if kind == "video" && ext != ".mp4" && ext != ".mov" && ext != ".webm" {
		return "", errors.New("unsupported video type")
	}
	if kind == "markdown" && ext != ".md" && ext != ".markdown" {
		return "", errors.New("unsupported markdown type")
	}
	if kind == "asset" && ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".webp" && ext != ".gif" && ext != ".mp4" && ext != ".mov" && ext != ".webm" {
		return "", errors.New("unsupported asset type")
	}
	if kind == "chat" && !isChatAttachmentExt(ext) {
		return "", errors.New("unsupported chat attachment type")
	}

	limit := int64(MaxVideoBytes)
	if kind == "markdown" {
		limit = MaxMarkdownBytes
	}
	if kind == "asset" {
		limit = MaxAssetBytes
	}
	if kind == "chat" {
		limit = MaxChatAttachmentBytes
	}
	if header.Size > limit {
		return "", errors.New("file is too large")
	}

	name := randomName() + ext
	dir := filepath.Join(s.root, kind)
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
