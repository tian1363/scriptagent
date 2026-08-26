package agent

import (
	"strings"
	"testing"

	"github.com/tian1363/scriptagent/internal/jobs"
)

func TestFissionPromptOmitsUpstreamBulkContextAndUnusedRules(t *testing.T) {
	job := jobs.Job{
		FissionCount:      1,
		FissionDirections: "结构层-换CTA",
		Requirement:       "保留品牌语气",
	}
	prompt := fissionScriptPrompt(job, `{"replica_script":{"title":"base"}}`)
	if strings.Contains(prompt, "产品 Markdown") || strings.Contains(prompt, "视频理解结果") {
		t.Fatal("fission prompt should not repeat full product or analysis context")
	}
	if !strings.Contains(prompt, "结构层-换CTA") || !strings.Contains(prompt, "重点改写最后 1-2 个分镜") {
		t.Fatal("expected selected dimension and its rule")
	}
	if strings.Contains(prompt, "视听层-换BGM：") {
		t.Fatal("unselected dimension rules should not be injected")
	}
}

func TestValidateFissionDimensionsAllowsSingleSelectedElement(t *testing.T) {
	scripts := []scriptPayload{
		{Metadata: metadata{FissionDimension: "视听层-换BGM"}},
	}
	if err := validateFissionDimensions(scripts, "视听层-换BGM"); err != nil {
		t.Fatalf("expected selected single element to pass: %v", err)
	}
}

func TestValidateFissionDimensionsRejectsMixedElement(t *testing.T) {
	scripts := []scriptPayload{
		{Metadata: metadata{FissionDimension: "视听层-换BGM+结构层-换CTA"}},
	}
	if err := validateFissionDimensions(scripts, "视听层-换BGM\n结构层-换CTA"); err == nil {
		t.Fatal("expected mixed fission element to fail")
	}
}

func TestValidateFissionDimensionsRejectsOutOfOrderSelectedElement(t *testing.T) {
	scripts := []scriptPayload{
		{Metadata: metadata{FissionDimension: "结构层-换CTA"}},
		{Metadata: metadata{FissionDimension: "视听层-换BGM"}},
	}
	if err := validateFissionDimensions(scripts, "视听层-换BGM\n结构层-换CTA"); err == nil {
		t.Fatal("expected out-of-order fission element to fail")
	}
}

func TestValidateFissionDimensionsAllowsRepeatedSelectedElement(t *testing.T) {
	scripts := []scriptPayload{
		{Metadata: metadata{FissionDimension: "视听层-换BGM"}},
		{Metadata: metadata{FissionDimension: "视听层-换BGM"}},
	}
	if err := validateFissionDimensions(scripts, "视听层-换BGM\n视听层-换BGM"); err != nil {
		t.Fatalf("expected repeated selected element to pass: %v", err)
	}
}

func TestValidateFissionDimensionsUsesAllDirectionsWhenUnselected(t *testing.T) {
	scripts := []scriptPayload{
		{Metadata: metadata{FissionDimension: "元素层-字幕语言本地化"}},
	}
	if err := validateFissionDimensions(scripts, ""); err != nil {
		t.Fatalf("expected default all directions to pass: %v", err)
	}
}
