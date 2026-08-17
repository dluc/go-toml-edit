package tomledit

import "testing"

// Bug: a leading UTF-8 BOM (0xEF 0xBB 0xBF, U+FEFF) makes Parse reject
// otherwise-valid TOML. Since this library is a round-trip-preserving
// editor, the fix must ACCEPT and PRESERVE the BOM (not strip it), while
// still rejecting a BOM that appears anywhere other than the very start.

func TestBOM_LeadingAcceptedAndPreserved(t *testing.T) {
	roundTripExact(t, "\ufeffa = 1\n")
}

func TestBOM_OnlyAcceptedAndPreserved(t *testing.T) {
	src := "\ufeff"
	doc, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse returned error for BOM-only input: %v", err)
	}
	if len(doc.Children) != 0 {
		t.Fatalf("expected empty document for BOM-only input, got %d children", len(doc.Children))
	}
	roundTripExact(t, src)
}

func TestBOM_SemanticParseCorrect(t *testing.T) {
	doc, err := Parse([]byte("\ufeffa = 1\n"))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	val, ok := doc.GetInt("a")
	if !ok {
		t.Fatalf("GetInt(%q) returned ok=false", "a")
	}
	if val != 1 {
		t.Fatalf("expected a == 1, got %d", val)
	}
}

// Regression guard: a BOM that is NOT at the very start of the input must
// remain an error. This must not regress when leading-BOM support is added.
func TestBOM_MidFileStillErrors(t *testing.T) {
	_, err := Parse([]byte("a = 1\n\ufeffb = 2\n"))
	if err == nil {
		t.Fatal("expected ParseError for mid-file BOM, got nil")
	}
	if _, ok := err.(*ParseError); !ok {
		t.Fatalf("expected *ParseError, got %T: %v", err, err)
	}
}

// Regression guard: only ONE leading BOM is consumed. A second BOM
// immediately following the first is content, not a second marker, and
// must still error (matching spec/other parsers).
func TestBOM_DoubleLeadingStillErrors(t *testing.T) {
	_, err := Parse([]byte("\ufeff\ufeffa = 1\n"))
	if err == nil {
		t.Fatal("expected ParseError for double leading BOM, got nil")
	}
	if _, ok := err.(*ParseError); !ok {
		t.Fatalf("expected *ParseError, got %T: %v", err, err)
	}
}
