package creatibi

import (
	"strings"
	"testing"
)

func TestToCreatiBIDocMapsStoryboardFields(t *testing.T) {
	doc := toCreatiBIDoc("测试脚本", []storyboard{
		{
			TimeRange:    "00:00-00:03",
			Visual:       "左侧产品入画，右侧出现卖点贴片。",
			Action:       "镜头向前推进，产品旋转展示。",
			Voiceover:    "先解决最难的问题。",
			Subtitle:     "3 秒看懂核心卖点",
			ShotSize:     "近景",
			CameraIntent: "建立注意力焦点。",
			PropsScene:   "产品、白色桌面、卖点贴片",
			Audio:        "轻快 BGM + 点击音效",
			Purpose:      "hook",
		},
	})

	content := doc["content"].([]map[string]any)
	frame := content[1]
	items := frame["content"].([]map[string]any)
	item := items[0]
	attrs := item["attrs"].(map[string]any)
	property := attrs["property"].(map[string]any)
	textProperty := property["text"].(map[string]any)

	if property["Movement"] != "镜头向前推进，产品旋转展示。" {
		t.Fatalf("Movement was not mapped from action: %#v", property["Movement"])
	}
	if property["Prop"] != "产品、白色桌面、卖点贴片" {
		t.Fatalf("Prop was not mapped from props_scene: %#v", property["Prop"])
	}
	if property["ShotSize"] != "近景" {
		t.Fatalf("ShotSize was not mapped from shot_size: %#v", property["ShotSize"])
	}
	if textProperty["SoundEffec"] != "轻快 BGM + 点击音效" {
		t.Fatalf("SoundEffec was not mapped from audio: %#v", textProperty["SoundEffec"])
	}

	fields := item["content"].([]map[string]any)
	assertFieldContains(t, fields[0], "Copy", "旁白/对话：先解决最难的问题。")
	assertFieldContains(t, fields[0], "Copy", "字幕：3 秒看懂核心卖点")
	assertFieldContains(t, fields[1], "Note", "镜头动机：建立注意力焦点。")
	assertFieldContains(t, fields[1], "Note", "叙事目的：hook")
	assertFieldContains(t, fields[2], "Description", "画面描述：左侧产品入画，右侧出现卖点贴片。")
	assertFieldContains(t, fields[2], "Description", "动作描述：镜头向前推进，产品旋转展示。")
}

func assertFieldContains(t *testing.T, field map[string]any, label string, expected string) {
	t.Helper()
	attrs := field["attrs"].(map[string]any)
	if attrs["label"] != label {
		t.Fatalf("expected field label %q, got %#v", label, attrs["label"])
	}
	paragraphs := field["content"].([]map[string]any)
	content := paragraphs[0]["content"].([]map[string]any)
	text := content[0]["text"].(string)
	if !strings.Contains(text, expected) {
		t.Fatalf("field %s did not contain %q: %s", label, expected, text)
	}
}
