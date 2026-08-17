package tomledit

import (
	"strings"
	"testing"
)

// --- Bug B: hand-built table/array-table nodes must not serialize to empty ---

// A hand-built *ArrayTableNode (dirty=false, raw=nil) must still render its
// header from KeyPath -- it must not vanish from the output.
func TestSerialize_HandBuiltArrayTableNode(t *testing.T) {
	doc, err := Parse([]byte("a = 1\n"))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	doc.Children = append(doc.Children, &ArrayTableNode{KeyPath: []string{"z"}})

	out := string(doc.Bytes())
	if !strings.Contains(out, "[[z]]") {
		t.Fatalf("expected output to contain [[z]] header, got:\n%q", out)
	}

	doc2 := roundTrip(t, doc)
	if got := countArrayTableEntries(doc2, "z"); got != 1 {
		t.Errorf("expected 1 [[z]] entry after re-parse, got %d", got)
	}
}

// A hand-built *TableNode (dirty=false, raw=nil) must still render its
// header from KeyPath -- it must not vanish from the output.
func TestSerialize_HandBuiltTableNode(t *testing.T) {
	doc, err := Parse([]byte("a = 1\n"))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	doc.Children = append(doc.Children, &TableNode{KeyPath: []string{"w"}})

	out := string(doc.Bytes())
	if !strings.Contains(out, "[w]") {
		t.Fatalf("expected output to contain [w] header, got:\n%q", out)
	}

	doc2 := roundTrip(t, doc)
	if val := doc2.Get("w"); val == nil {
		t.Errorf("expected table [w] to be resolvable after re-parse")
	}
}

// A hand-built *TableNode with a hand-built child *KeyValueNode: both the
// header and the child key-value must render.
func TestSerialize_HandBuiltTableNodeWithChild(t *testing.T) {
	doc, err := Parse([]byte("a = 1\n"))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	val := &IntegerNode{Val: 5}
	val.markDirty()
	kv := &KeyValueNode{
		Key: &KeyNode{Parts: []string{"n"}},
		Val: val,
	}
	kv.markDirty()
	kv.Key.markDirty()
	tbl := &TableNode{KeyPath: []string{"w"}, Children: []Node{kv}}
	doc.Children = append(doc.Children, tbl)

	out := string(doc.Bytes())
	if !strings.Contains(out, "[w]") {
		t.Fatalf("expected output to contain [w] header, got:\n%q", out)
	}
	if !strings.Contains(out, "n = 5") {
		t.Fatalf("expected output to contain child key-value 'n = 5', got:\n%q", out)
	}

	doc2 := roundTrip(t, doc)
	if got, ok := doc2.GetInt("w.n"); !ok || got != 5 {
		t.Errorf("expected w.n == 5 after re-parse, got %d (ok=%v)", got, ok)
	}
}

// Regression guard: a normally parsed, unmodified document must still
// round-trip byte-exact. The len(Raw())>0 guard on the clean-node branch
// must not change output for parsed nodes, which always carry raw bytes.
func TestSerialize_ParsedDocumentStillRoundTripsExact(t *testing.T) {
	src := "[[x]]\na = 1\n[x.sub]\ns = 9\n\n[y]\nb = 2\n\n# trailing\n"
	roundTripExact(t, src)
}
