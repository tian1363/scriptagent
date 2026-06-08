package agent

import "testing"

func TestValidateFissionDimensionsAllowsSingleSelectedElement(t *testing.T) {
	scripts := []scriptPayload{
		{Metadata: metadata{FissionDimension: "视听层-换BGM"}},
	}
	if err := validateFissionDimensions(scripts, "视听层-换BGM\n结构层-换CTA"); err != nil {
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

func TestValidateFissionDimensionsUsesAllDirectionsWhenUnselected(t *testing.T) {
	scripts := []scriptPayload{
		{Metadata: metadata{FissionDimension: "元素层-字幕语言本地化"}},
	}
	if err := validateFissionDimensions(scripts, ""); err != nil {
		t.Fatalf("expected default all directions to pass: %v", err)
	}
}
