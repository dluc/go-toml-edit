package tomledit

import (
	"testing"
)

func TestParseArrayOfInlineTables(t *testing.T) {
	input := `items = [{name = "a", value = 1}, {name = "b", value = 2}]` + "\n"
	doc, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	// Verify we can read values from the array of inline tables
	s, ok := doc.GetString("items[0].name")
	if !ok || s != "a" {
		t.Fatalf("expected 'a', got %q (ok=%v)", s, ok)
	}
	s, ok = doc.GetString("items[1].name")
	if !ok || s != "b" {
		t.Fatalf("expected 'b', got %q (ok=%v)", s, ok)
	}
	// Round-trip
	out := doc.Bytes()
	if string(out) != input {
		t.Fatalf("round-trip failed:\ngot:  %q\nwant: %q", out, input)
	}
}

func TestParseArrayCommentsPreserved(t *testing.T) {
	input := "arr = [\n    1,\n    # comment between elements\n    2,\n    3,\n]\n"
	doc, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	out := doc.Bytes()
	if string(out) != input {
		t.Fatalf("round-trip failed:\ngot:  %q\nwant: %q", out, input)
	}
}
