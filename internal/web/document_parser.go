package web

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func parseProductDocument(filename string, content []byte) (string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".md", ".markdown", ".txt":
		return requireDocumentText(string(content))
	case ".docx":
		return parseDOCX(content)
	case ".doc", ".pdf":
		return parseWithSystemDocumentTools(ext, content)
	default:
		return "", errors.New("仅支持 MD、PDF、DOC 或 DOCX 文件")
	}
}

func requireDocumentText(value string) (string, error) {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\x00", ""))
	if value == "" {
		return "", errors.New("文件中没有可提取的文字；扫描版 PDF 请先完成 OCR")
	}
	return value, nil
}

func parseDOCX(content []byte) (string, error) {
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return "", errors.New("DOCX 文件已损坏或格式不正确")
	}
	for _, file := range reader.File {
		if file.Name != "word/document.xml" {
			continue
		}
		stream, err := file.Open()
		if err != nil {
			return "", err
		}
		defer stream.Close()
		decoder := xml.NewDecoder(stream)
		var result strings.Builder
		inText := false
		for {
			token, err := decoder.Token()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return "", errors.New("无法读取 DOCX 正文")
			}
			switch item := token.(type) {
			case xml.StartElement:
				if item.Name.Local == "t" {
					inText = true
				}
				if item.Name.Local == "tab" {
					result.WriteByte('\t')
				}
			case xml.CharData:
				if inText {
					result.Write([]byte(item))
				}
			case xml.EndElement:
				if item.Name.Local == "t" {
					inText = false
				}
				if item.Name.Local == "p" {
					result.WriteByte('\n')
				}
			}
		}
		return requireDocumentText(result.String())
	}
	return "", errors.New("DOCX 中未找到正文")
}

func parseWithSystemDocumentTools(ext string, content []byte) (string, error) {
	temp, err := os.CreateTemp("", "scriptagent-document-*"+ext)
	if err != nil {
		return "", err
	}
	name := temp.Name()
	defer os.Remove(name)
	if _, err = temp.Write(content); err != nil {
		temp.Close()
		return "", err
	}
	if err = temp.Close(); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if ext == ".doc" {
		if runtime.GOOS != "darwin" {
			return "", errors.New("当前环境暂不支持旧版 DOC，请另存为 DOCX 后上传")
		}
		output, err := exec.CommandContext(ctx, "/usr/bin/textutil", "-convert", "txt", "-stdout", "--", name).Output()
		if err != nil {
			return "", errors.New("无法解析 DOC，请另存为 DOCX 后重试")
		}
		return requireDocumentText(string(output))
	}
	if path, err := exec.LookPath("pdftotext"); err == nil {
		output, commandErr := exec.CommandContext(ctx, path, name, "-").Output()
		if commandErr == nil {
			return requireDocumentText(string(output))
		}
	}
	if runtime.GOOS == "darwin" {
		_ = exec.CommandContext(ctx, "/usr/bin/mdimport", "-i", name).Run()
		output, err := exec.CommandContext(ctx, "/usr/bin/mdls", "-raw", "-name", "kMDItemTextContent", name).Output()
		if err == nil {
			raw := strings.TrimSpace(string(output))
			if raw != "(null)" {
				if decoded, unquoteErr := strconv.Unquote(raw); unquoteErr == nil {
					raw = decoded
				}
				if text, textErr := requireDocumentText(raw); textErr == nil {
					return text, nil
				}
			}
		}
	}
	return "", fmt.Errorf("无法提取 PDF 文字；扫描版 PDF 请先完成 OCR，或转换为 DOCX/MD 后上传")
}
