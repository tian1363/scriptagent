package reactagent

import "testing"

func TestParseActionFromJSON(t *testing.T) {
	action, err := parseAction(`{"type":"tool","reason":"需要查产品资料","tool":"list_products","input":{}}`)
	if err != nil {
		t.Fatal(err)
	}
	if action.Type != "tool" || action.Tool != "list_products" {
		t.Fatalf("unexpected action: %+v", action)
	}
}

func TestParseActionFromFencedJSON(t *testing.T) {
	action, err := parseAction("```json\n{\"type\":\"final\",\"answer\":\"完成\"}\n```")
	if err != nil {
		t.Fatal(err)
	}
	if action.Type != "final" || action.Answer != "完成" {
		t.Fatalf("unexpected action: %+v", action)
	}
}

func TestNormalizeRawJSON(t *testing.T) {
	raw := normalizeRawJSON([]byte(`{"b":2}`))
	if string(raw) != `{"b":2}` {
		t.Fatalf("unexpected normalized json: %s", raw)
	}
	empty := normalizeRawJSON(nil)
	if string(empty) != `{}` {
		t.Fatalf("unexpected empty json: %s", empty)
	}
}
