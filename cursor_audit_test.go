package tomledit

import (
	"testing"
)

// --- Cursor error propagation: 5+ calls after error ---

func TestAuditCursor_ErrorPropagation5Calls(t *testing.T) {
	doc, err := Parse([]byte(`[a]
x = 1
`))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// The first Key("nonexistent") fails. All subsequent calls should be no-ops.
	c := doc.Key("nonexistent").Key("b").Key("c").Key("d").Key("e").At(0)
	if c.Err() == nil {
		t.Fatal("expected error after chained calls on missing key")
	}
	if c.Node() != nil {
		t.Error("Node() should return nil after error")
	}

	// All typed extractors should return zero/false
	if s, ok := c.String(); ok || s != "" {
		t.Errorf("String() after error: (%q, %v)", s, ok)
	}
	if n, ok := c.Int(); ok || n != 0 {
		t.Errorf("Int() after error: (%d, %v)", n, ok)
	}
	if b, ok := c.Bool(); ok || b != false {
		t.Errorf("Bool() after error: (%v, %v)", b, ok)
	}
	if f, ok := c.Float(); ok || f != 0 {
		t.Errorf("Float() after error: (%f, %v)", f, ok)
	}
	if _, ok := c.Time(); ok {
		t.Error("Time() after error should return false")
	}
}

func TestAuditCursor_FirstErrorPreserved(t *testing.T) {
	doc, err := Parse([]byte(`[a]
x = 1
`))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	c := doc.Key("missing1").Key("missing2").Key("missing3")
	errMsg := c.Err().Error()
	// The error should reference "missing1" since that's the first failure
	if errMsg == "" {
		t.Fatal("error message should not be empty")
	}
	// The key that failed first should be in the error
	if !containsString(errMsg, "missing1") {
		t.Errorf("expected error to mention 'missing1', got: %s", errMsg)
	}
	// Should NOT contain "missing2" since we never got that far
	// (the error is captured at "missing1" and subsequent calls are no-ops)
}

func TestAuditCursor_ErrorThenKeyIsNoop(t *testing.T) {
	doc, err := Parse([]byte(`
[server]
host = "localhost"
`))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// Navigate to error, then try valid key
	c := doc.Key("nonexistent").Key("server").Key("host")
	if c.Err() == nil {
		t.Fatal("expected error")
	}
	if c.Node() != nil {
		t.Error("Node() should be nil")
	}
}

func TestAuditCursor_ErrorThenAtIsNoop(t *testing.T) {
	doc, err := Parse([]byte(`
[[items]]
name = "a"
`))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	c := doc.Key("nonexistent").At(0)
	if c.Err() == nil {
		t.Fatal("expected error")
	}
}

// --- Cursor never nil ---

func TestAuditCursor_NeverNil(t *testing.T) {
	doc, err := Parse([]byte(`key = "val"`))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// Every call must return non-nil cursor
	c1 := doc.Key("nonexistent")
	if c1 == nil {
		t.Fatal("Key() returned nil cursor")
	}

	c2 := c1.Key("anything")
	if c2 == nil {
		t.Fatal("Key() on error cursor returned nil")
	}

	c3 := c2.At(0)
	if c3 == nil {
		t.Fatal("At() on error cursor returned nil")
	}

	c4 := c3.At(-1)
	if c4 == nil {
		t.Fatal("At() on error cursor returned nil (2)")
	}
}

// --- Cursor cross-table resolution ---

func TestAuditCursor_DeepCrossTable(t *testing.T) {
	input := `
[a]
x = 1

[a.b]
y = 2

[a.b.c]
z = 3
`
	doc, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	val, ok := doc.Key("a").Key("b").Key("c").Key("z").Int()
	if !ok {
		t.Fatal("cursor a.b.c.z Int() returned false")
	}
	if val != 3 {
		t.Errorf("expected 3, got %d", val)
	}
}

// --- Cursor array-of-tables ---

func TestAuditCursor_ArrayOfTablesIndexing(t *testing.T) {
	input := `
[[products]]
name = "Widget"

[[products]]
name = "Gadget"

[[products]]
name = "Doohickey"
`
	doc, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// Positive index
	val, ok := doc.Key("products").At(2).Key("name").String()
	if !ok {
		t.Fatal("products[2].name returned false")
	}
	if val != "Doohickey" {
		t.Errorf("expected \"Doohickey\", got %q", val)
	}

	// Negative index
	val, ok = doc.Key("products").At(-1).Key("name").String()
	if !ok {
		t.Fatal("products[-1].name returned false")
	}
	if val != "Doohickey" {
		t.Errorf("expected \"Doohickey\", got %q", val)
	}

	// Out of bounds
	c := doc.Key("products").At(10)
	if c.Err() == nil {
		t.Error("At(10) should set error for out of bounds")
	}
}

// --- Cursor inline table ---

func TestAuditCursor_InlineTable(t *testing.T) {
	input := `
tbl = {a = 1, b = "hello", c = true}
`
	doc, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	val, ok := doc.Key("tbl").Key("a").Int()
	if !ok {
		t.Fatal("tbl.a returned false")
	}
	if val != 1 {
		t.Errorf("expected 1, got %d", val)
	}

	sval, ok := doc.Key("tbl").Key("b").String()
	if !ok {
		t.Fatal("tbl.b returned false")
	}
	if sval != "hello" {
		t.Errorf("expected \"hello\", got %q", sval)
	}

	bval, ok := doc.Key("tbl").Key("c").Bool()
	if !ok {
		t.Fatal("tbl.c returned false")
	}
	if bval != true {
		t.Errorf("expected true, got %v", bval)
	}
}

// --- Cursor type mismatch ---

func TestAuditCursor_TypeMismatch(t *testing.T) {
	input := `val = 42`
	doc, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	c := doc.Key("val")

	// Int on int should work
	if n, ok := c.Int(); !ok || n != 42 {
		t.Errorf("Int() on int: (%d, %v)", n, ok)
	}

	// String on int should fail
	if s, ok := c.String(); ok || s != "" {
		t.Errorf("String() on int: (%q, %v)", s, ok)
	}

	// Bool on int should fail
	if b, ok := c.Bool(); ok || b != false {
		t.Errorf("Bool() on int: (%v, %v)", b, ok)
	}

	// Float on int should fail
	if f, ok := c.Float(); ok || f != 0 {
		t.Errorf("Float() on int: (%f, %v)", f, ok)
	}

	// Time on int should fail
	if _, ok := c.Time(); ok {
		t.Error("Time() on int should return false")
	}
}

// --- Cursor Node() returns the right type ---

func TestAuditCursor_NodeReturnsCorrectType(t *testing.T) {
	input := `
[section]
val = "hello"
`
	doc, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// Node at table
	tblNode := doc.Key("section").Node()
	if tblNode == nil {
		t.Fatal("Key(\"section\").Node() returned nil")
	}
	if _, ok := tblNode.(*TableNode); !ok {
		t.Errorf("expected *TableNode, got %T", tblNode)
	}

	// Node at value
	valNode := doc.Key("section").Key("val").Node()
	if valNode == nil {
		t.Fatal("section.val Node() returned nil")
	}
	if _, ok := valNode.(*StringNode); !ok {
		t.Errorf("expected *StringNode, got %T", valNode)
	}
}

// --- Cursor with dotted keys ---

func TestAuditCursor_DottedKeys(t *testing.T) {
	input := `
[section]
a.b.c = "dotted"
`
	doc, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	val, ok := doc.Key("section").Key("a").Key("b").Key("c").String()
	if !ok {
		t.Fatal("section.a.b.c String() returned false")
	}
	if val != "dotted" {
		t.Errorf("expected \"dotted\", got %q", val)
	}
}

// --- Cursor array value indexing ---

func TestAuditCursor_ArrayValueIndexing(t *testing.T) {
	input := `arr = [10, 20, 30]`
	doc, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	val, ok := doc.Key("arr").At(1).Int()
	if !ok {
		t.Fatal("arr[1] Int() returned false")
	}
	if val != 20 {
		t.Errorf("expected 20, got %d", val)
	}

	// Negative index
	val, ok = doc.Key("arr").At(-1).Int()
	if !ok {
		t.Fatal("arr[-1] Int() returned false")
	}
	if val != 30 {
		t.Errorf("expected 30, got %d", val)
	}

	// Out of bounds
	c := doc.Key("arr").At(5)
	if c.Err() == nil {
		t.Error("At(5) should set error")
	}
}

// --- Cursor: Err() on valid cursor is nil ---

func TestAuditCursor_ErrOnValidCursor(t *testing.T) {
	doc, err := Parse([]byte(`key = "val"`))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	c := doc.Key("key")
	if c.Err() != nil {
		t.Errorf("Err() on valid cursor should be nil, got: %v", c.Err())
	}
}

// helper
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
