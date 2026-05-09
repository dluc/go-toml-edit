package tomledit

import (
	"strings"
	"testing"
)

// Audit focus 3: SetComment round-trip with Bytes().
func TestAudit_SetComment_BytesIncludesComment(t *testing.T) {
	input := `[server]
host = "localhost"
port = 8080
`
	doc, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	err = doc.SetComment("server.host", "primary host")
	if err != nil {
		t.Fatalf("SetComment: %v", err)
	}

	out := string(doc.Bytes())
	if !strings.Contains(out, "# primary host") {
		t.Errorf("Bytes() output does not contain comment:\n%s", out)
	}

	// Verify the comment is on the same line as the value.
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "host") && strings.Contains(line, "localhost") {
			if !strings.Contains(line, "# primary host") {
				t.Errorf("comment not on same line as value:\nline: %s\nfull output:\n%s", line, out)
			}
		}
	}
}

// Audit focus 3b: SetComment round-trip with Format().
func TestAudit_SetComment_FormatIncludesComment(t *testing.T) {
	input := `[server]
host = "localhost"
port = 8080
`
	doc, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	err = doc.SetComment("server.host", "primary host")
	if err != nil {
		t.Fatalf("SetComment: %v", err)
	}

	out := string(doc.Format())
	if !strings.Contains(out, "# primary host") {
		t.Errorf("Format() output does not contain comment:\n%s", out)
	}
}

// Audit focus 3c: SetLeadingComments round-trip with Bytes().
func TestAudit_SetLeadingComments_BytesRoundTrip(t *testing.T) {
	input := `[server]
host = "localhost"
`
	doc, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	err = doc.SetLeadingComments("server.host", []string{"The primary host", "Used for connections"})
	if err != nil {
		t.Fatalf("SetLeadingComments: %v", err)
	}

	out := string(doc.Bytes())
	if !strings.Contains(out, "# The primary host") {
		t.Errorf("Bytes() missing first leading comment:\n%s", out)
	}
	if !strings.Contains(out, "# Used for connections") {
		t.Errorf("Bytes() missing second leading comment:\n%s", out)
	}
}

// Audit focus 3d: SetLeadingComments round-trip with Format().
func TestAudit_SetLeadingComments_FormatRoundTrip(t *testing.T) {
	input := `[server]
host = "localhost"
`
	doc, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	err = doc.SetLeadingComments("server.host", []string{"The primary host"})
	if err != nil {
		t.Fatalf("SetLeadingComments: %v", err)
	}

	out := string(doc.Format())
	if !strings.Contains(out, "# The primary host") {
		t.Errorf("Format() missing leading comment:\n%s", out)
	}
}

// Audit focus 3e: SetComment followed by re-parse preserves comment.
func TestAudit_SetComment_ReparsePreservesComment(t *testing.T) {
	input := `name = "test"
`
	doc, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	err = doc.SetComment("name", "project name")
	if err != nil {
		t.Fatalf("SetComment: %v", err)
	}

	out := doc.Bytes()
	doc2, err := Parse(out)
	if err != nil {
		t.Fatalf("re-parse: %v\noutput was:\n%s", err, string(out))
	}

	// Value should survive.
	val, ok := doc2.GetString("name")
	if !ok || val != "test" {
		t.Errorf("expected name='test' after re-parse, got %q (ok=%v)", val, ok)
	}

	// Comment should survive in serialized output.
	out2 := string(doc2.Bytes())
	if !strings.Contains(out2, "# project name") {
		t.Errorf("comment lost after re-parse:\n%s", out2)
	}
}

// Audit focus 4: SetComment on inline table key.
// config = {x = 1} -> SetComment("config.x", "inline comment")
func TestAudit_SetComment_InlineTableKey(t *testing.T) {
	input := `config = {x = 1, y = 2}
`
	doc, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	err = doc.SetComment("config.x", "inline comment")
	if err != nil {
		t.Fatalf("SetComment on inline table key: %v", err)
	}

	out := string(doc.Bytes())
	t.Logf("Bytes() output:\n%s", out)

	// The comment should appear somewhere in the output.
	if !strings.Contains(out, "# inline comment") {
		t.Errorf("Bytes() does not contain inline comment on inline table key:\n%s", out)
	}

	// Verify the output is still valid TOML.
	doc2, err := Parse([]byte(out))
	if err != nil {
		// Inline tables in TOML don't support comments -- this might fail.
		// If so, that's an important finding: SetComment on inline table keys
		// produces invalid TOML.
		t.Logf("WARNING: SetComment on inline table key produced invalid TOML: %v", err)
		t.Logf("This is a potential bug: inline tables cannot have comments per TOML spec")
	} else {
		// If it parses, verify the value is intact.
		val, ok := doc2.GetInt("config.x")
		if !ok || val != 1 {
			t.Errorf("expected config.x=1, got %d (ok=%v)", val, ok)
		}
	}
}

// Audit focus 4b: SetComment on inline table key round-trip with Format().
func TestAudit_SetComment_InlineTableKey_Format(t *testing.T) {
	input := `config = {x = 1, y = 2}
`
	doc, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	err = doc.SetComment("config.x", "x value")
	if err != nil {
		t.Fatalf("SetComment: %v", err)
	}

	out := string(doc.Format())
	t.Logf("Format() output:\n%s", out)

	// Format() re-renders from semantic values -- does it include the comment?
	// Note: inline tables are single-line in TOML, comments inside them are
	// not valid TOML. This test documents the actual behavior.
	if !strings.Contains(out, "# x value") {
		t.Logf("Format() does not include comment on inline table key (expected: inline table comments are not standard TOML)")
	}
}

// Audit: SetComment on table header with Format().
func TestAudit_SetComment_TableHeader_Format(t *testing.T) {
	input := `[database]
name = "mydb"
`
	doc, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	err = doc.SetComment("database", "DB config")
	if err != nil {
		t.Fatalf("SetComment: %v", err)
	}

	out := string(doc.Format())
	if !strings.Contains(out, "# DB config") {
		t.Errorf("Format() does not contain table header comment:\n%s", out)
	}

	// Check it's on the header line.
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "[database]") {
			if !strings.Contains(line, "# DB config") {
				t.Errorf("comment not on table header line:\nline: %s\nfull:\n%s", line, out)
			}
		}
	}
}

// Audit: SetLeadingComments on table header.
func TestAudit_SetLeadingComments_TableHeader(t *testing.T) {
	input := `[database]
name = "mydb"
`
	doc, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	err = doc.SetLeadingComments("database", []string{"Database configuration"})
	if err != nil {
		t.Fatalf("SetLeadingComments: %v", err)
	}

	out := string(doc.Bytes())
	if !strings.Contains(out, "# Database configuration") {
		t.Errorf("Bytes() does not contain leading comment on table header:\n%s", out)
	}

	// The comment should appear before [database].
	idx1 := strings.Index(out, "# Database configuration")
	idx2 := strings.Index(out, "[database]")
	if idx1 < 0 || idx2 < 0 || idx1 >= idx2 {
		t.Errorf("leading comment should appear before table header:\n%s", out)
	}
}

// Audit: SetComment on array-of-tables header.
func TestAudit_SetComment_ArrayTableHeader(t *testing.T) {
	input := `[[products]]
name = "Widget"

[[products]]
name = "Gadget"
`
	doc, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// Set comment on the first products entry.
	// We need to access it via products[0], but SetComment works on path strings.
	// Try setting on the array-table header itself.
	err = doc.SetComment("products[0]", "first product")
	if err != nil {
		t.Logf("SetComment on products[0] returned error: %v", err)
		t.Logf("This means SetComment doesn't support array-table entry paths")
	} else {
		out := string(doc.Bytes())
		if !strings.Contains(out, "# first product") {
			t.Errorf("Bytes() does not contain comment on array table entry:\n%s", out)
		}
	}
}
