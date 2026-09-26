package storage

import (
	"bytes"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
)

func (s *LocalStore) SaveCharacterImage(data []byte) (string, error) {
	if len(data) == 0 || len(data) > 10<<20 {
		return "", errors.New("角色图需为 10MB 以内的 PNG 或 JPG")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "png" && format != "jpeg") {
		return "", errors.New("请选择有效的 PNG 或 JPG 角色图")
	}
	if cfg.Width < 384 || cfg.Height < 384 || cfg.Width > 3072 || cfg.Height > 3072 {
		return "", errors.New("角色图宽高需在 384–3072 像素之间")
	}
	if _, _, err = image.Decode(bytes.NewReader(data)); err != nil {
		return "", errors.New("角色图数据不完整，请重新上传")
	}
	dir := filepath.Join(s.root, "characters")
	if err = os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, "character-*."+format)
	if err != nil {
		return "", err
	}
	path := f.Name()
	_, err = f.Write(data)
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		_ = os.Remove(path)
		return "", errors.New("角色图保存失败")
	}
	return path, nil
}
