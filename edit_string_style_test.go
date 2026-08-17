package tomledit

import "testing"

// Bug #5c: Set replaces an existing string value with a fresh node that
// always defaults to StringBasic ("...") style, discarding the original
// quoting style (literal 'x', multi-line basic """x""", multi-line literal
// '''x'''). Fix: carry the OLD node's Style forward onto the replacement --
// but ONLY when the new value can be safely represented in that style.
// Otherwise fall back to StringBasic, which can always represent any string
// via escaping. The non-negotiable invariant: output must always be valid,
// re-parseable TOML.

// mustGetStringNode resolves path and asserts the resulting node is a
// *StringNode, returning it for direct Style inspection.
func mustGetStringNode(t *testing.T, doc *DocumentNode, path string) *StringNode {
	t.Helper()
	node := doc.Get(path)
	if node == nil {
		t.Fatalf("Get(%q) returned nil", path)
	}
	sn, ok := node.(*StringNode)
	if !ok {
		t.Fatalf("Get(%q) returned %T, not *StringNode", path, node)
	}
	return sn
}

// assertExactAndReparse checks doc.Bytes() equals want exactly, and that the
// output re-parses without error.
func assertExactAndReparse(t *testing.T, doc *DocumentNode, want string) {
	t.Helper()
	got := string(doc.Bytes())
	if got != want {
		t.Errorf("Bytes() mismatch:\n got:  %q\n want: %q", got, want)
	}
	if _, err := Parse([]byte(got)); err != nil {
		t.Fatalf("output does not re-parse as valid TOML: %v\noutput:\n%s", err, got)
	}
}

// 1. basic style is unaffected by Set (already correct; guards no regression).
func TestSet_StringStyle_BasicUnchanged(t *testing.T) {
	doc, err := Parse([]byte("k = \"x\"\n"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := doc.Set("k", "new"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	sn := mustGetStringNode(t, doc, "k")
	if sn.Style != StringBasic {
		t.Errorf("style = %v, want StringBasic", sn.Style)
	}
	assertExactAndReparse(t, doc, "k = \"new\"\n")
}

// 2. literal style must be preserved across Set.
func TestSet_StringStyle_LiteralPreserved(t *testing.T) {
	doc, err := Parse([]byte("k = 'x'\n"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := doc.Set("k", "new"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	sn := mustGetStringNode(t, doc, "k")
	if sn.Style != StringLiteral {
		t.Errorf("style = %v, want StringLiteral", sn.Style)
	}
	assertExactAndReparse(t, doc, "k = 'new'\n")
}

// 3. multi-line basic style must be preserved across Set.
func TestSet_StringStyle_MultiLineBasicPreserved(t *testing.T) {
	doc, err := Parse([]byte("k = \"\"\"x\"\"\"\n"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := doc.Set("k", "new"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	sn := mustGetStringNode(t, doc, "k")
	if sn.Style != StringMultiLineBasic {
		t.Errorf("style = %v, want StringMultiLineBasic", sn.Style)
	}
	if sn.Val != "new" {
		t.Errorf("value = %q, want %q", sn.Val, "new")
	}
	// renderMultiLineBasicString always emits a newline right after the
	// opening delimiter (TOML trims it from the semantic value), so the
	// rendered form is `"""` + "\n" + content + `"""` -- matching the
	// existing renderer behavior exercised in serializer_test.go.
	assertExactAndReparse(t, doc, "k = \"\"\"\nnew\"\"\"\n")
}

// 4. multi-line literal style must be preserved across Set.
func TestSet_StringStyle_MultiLineLiteralPreserved(t *testing.T) {
	doc, err := Parse([]byte("k = '''x'''\n"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := doc.Set("k", "new"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	sn := mustGetStringNode(t, doc, "k")
	if sn.Style != StringMultiLineLiteral {
		t.Errorf("style = %v, want StringMultiLineLiteral", sn.Style)
	}
	if sn.Val != "new" {
		t.Errorf("value = %q, want %q", sn.Val, "new")
	}
	assertExactAndReparse(t, doc, "k = '''\nnew'''\n")
}

// 5. SAFETY: a literal can't hold an apostrophe -- must fall back to basic.
func TestSet_StringStyle_LiteralFallback_Apostrophe(t *testing.T) {
	doc, err := Parse([]byte("k = 'x'\n"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := doc.Set("k", "a'b"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	sn := mustGetStringNode(t, doc, "k")
	if sn.Style != StringBasic {
		t.Errorf("style = %v, want StringBasic (fallback)", sn.Style)
	}
	assertExactAndReparse(t, doc, "k = \"a'b\"\n")
}

// 6. SAFETY: a literal can't hold a newline -- must fall back to basic.
func TestSet_StringStyle_LiteralFallback_Newline(t *testing.T) {
	doc, err := Parse([]byte("k = 'x'\n"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := doc.Set("k", "a\nb"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	sn := mustGetStringNode(t, doc, "k")
	if sn.Style != StringBasic {
		t.Errorf("style = %v, want StringBasic (fallback)", sn.Style)
	}
	assertExactAndReparse(t, doc, "k = \"a\\nb\"\n")
}

// 7. SAFETY: multi-line literal can't hold ''' -- must fall back to a valid style.
func TestSet_StringStyle_MultiLineLiteralFallback_TripleQuote(t *testing.T) {
	doc, err := Parse([]byte("k = '''x'''\n"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := doc.Set("k", "has ''' inside"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	sn := mustGetStringNode(t, doc, "k")
	if sn.Style == StringMultiLineLiteral {
		t.Errorf("style = StringMultiLineLiteral, must not preserve an unsafe style")
	}
	assertExactAndReparse(t, doc, "k = \"has ''' inside\"\n")
}

// 8. SAFETY: multi-line literal can't end with a trailing quote (would collide
// with the closing ''' delimiter) -- must fall back to a valid style.
func TestSet_StringStyle_MultiLineLiteralFallback_TrailingQuote(t *testing.T) {
	doc, err := Parse([]byte("k = '''x'''\n"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := doc.Set("k", "ends with quote'"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	sn := mustGetStringNode(t, doc, "k")
	if sn.Style == StringMultiLineLiteral {
		t.Errorf("style = StringMultiLineLiteral, must not preserve an unsafe style")
	}
	assertExactAndReparse(t, doc, "k = \"ends with quote'\"\n")
}

// 9. old value was a non-string type (int) -- new string value must use the
// default basic style, not inherit anything from the old node.
func TestSet_StringStyle_OldNonString(t *testing.T) {
	doc, err := Parse([]byte("k = 42\n"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := doc.Set("k", "hi"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	sn := mustGetStringNode(t, doc, "k")
	if sn.Style != StringBasic {
		t.Errorf("style = %v, want StringBasic", sn.Style)
	}
	assertExactAndReparse(t, doc, "k = \"hi\"\n")
}

// 10. SetCreate on a brand-new key (no old node to carry from) uses the
// default basic style.
func TestSet_StringStyle_SetCreateNewKey(t *testing.T) {
	doc, err := Parse([]byte(""))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := doc.SetCreate("newkey", "hi"); err != nil {
		t.Fatalf("SetCreate: %v", err)
	}
	sn := mustGetStringNode(t, doc, "newkey")
	if sn.Style != StringBasic {
		t.Errorf("style = %v, want StringBasic", sn.Style)
	}
	assertExactAndReparse(t, doc, "newkey = \"hi\"\n")
}

// 11. MULTIPLE STYLES IN ONE DOCUMENT -- the key permutation. A document with
// several keys, each using a different style, must have each key preserve
// its own original style independently after Set, and the whole document
// must still re-parse.
func TestSet_StringStyle_MultipleStylesInOneDocument(t *testing.T) {
	src := "b = \"x\"\n" +
		"l = 'y'\n" +
		"mb = \"\"\"z\"\"\"\n" +
		"ml = '''w'''\n"
	doc, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	for path, val := range map[string]string{
		"b":  "newb",
		"l":  "newl",
		"mb": "newmb",
		"ml": "newml",
	} {
		if err := doc.Set(path, val); err != nil {
			t.Fatalf("Set(%q): %v", path, err)
		}
	}

	tests := []struct {
		path  string
		style StringStyle
		val   string
	}{
		{"b", StringBasic, "newb"},
		{"l", StringLiteral, "newl"},
		{"mb", StringMultiLineBasic, "newmb"},
		{"ml", StringMultiLineLiteral, "newml"},
	}
	for _, tt := range tests {
		sn := mustGetStringNode(t, doc, tt.path)
		if sn.Style != tt.style {
			t.Errorf("path %q: style = %v, want %v", tt.path, sn.Style, tt.style)
		}
		if sn.Val != tt.val {
			t.Errorf("path %q: value = %q, want %q", tt.path, sn.Val, tt.val)
		}
	}

	want := "b = \"newb\"\n" +
		"l = 'newl'\n" +
		"mb = \"\"\"\nnewmb\"\"\"\n" +
		"ml = '''\nnewml'''\n"
	assertExactAndReparse(t, doc, want)
}

// 12. a literal value inside a [table] context must preserve its style too.
func TestSet_StringStyle_LiteralInTable(t *testing.T) {
	doc, err := Parse([]byte("[t]\nk = 'x'\n"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := doc.Set("t.k", "new"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	sn := mustGetStringNode(t, doc, "t.k")
	if sn.Style != StringLiteral {
		t.Errorf("style = %v, want StringLiteral", sn.Style)
	}
	assertExactAndReparse(t, doc, "[t]\nk = 'new'\n")
}
