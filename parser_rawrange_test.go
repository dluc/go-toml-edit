package tomledit

import "testing"

// TestRawFromTokenRangeBounds exercises the O(1) bounds handling directly.
// Parse never produces an out-of-range endIdx (p.pos is capped by advance),
// so these defensive branches are only reachable white-box -- but dropping
// them would reintroduce an index-out-of-range panic if that ever changed.
func TestRawFromTokenRangeBounds(t *testing.T) {
	src := []byte("a = [1]\n")
	tokens, err := lex(src)
	if err != nil {
		t.Fatalf("lex returned error: %v", err)
	}
	p := &parser{tokens: tokens, src: src}

	if got := p.rawFromTokenRange(2, 2); got != nil {
		t.Errorf("empty range: got %q, want nil", got)
	}
	if got := p.rawFromTokenRange(3, 1); got != nil {
		t.Errorf("inverted range: got %q, want nil", got)
	}
	if got := string(p.rawFromTokenRange(0, len(tokens)+5)); got != string(src) {
		t.Errorf("clamped range: got %q, want %q", got, string(src))
	}
	if got := p.rawFromTokenRange(len(tokens)+1, len(tokens)+5); got != nil {
		t.Errorf("out-of-range start: got %q, want nil", got)
	}
}
