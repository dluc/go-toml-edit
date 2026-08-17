package tomledit

import "testing"

// TestRawReturnsCopy verifies that Raw() hands back a copy: mutating it must
// not reach the document or the caller's source buffer. The internal raw field
// aliases the source buffer, so returning it directly would let a consumer
// corrupt both.
func TestRawReturnsCopy(t *testing.T) {
	const original = "a = [1, 2]\nb = \"hello\"\n"
	src := []byte(original)
	doc, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	for _, i := range []int{0, 1} {
		kv, ok := doc.Children[i].(*KeyValueNode)
		if !ok {
			t.Fatalf("doc.Children[%d] is %T, want *KeyValueNode", i, doc.Children[i])
		}
		raw := kv.Val.Raw()
		if len(raw) == 0 {
			t.Fatalf("Raw() for child %d is empty", i)
		}
		for j := range raw {
			raw[j] = 'X'
		}
	}

	if string(src) != original {
		t.Errorf("mutating Raw() corrupted the source buffer:\n got: %q\nwant: %q", string(src), original)
	}
	if got := string(doc.Bytes()); got != original {
		t.Errorf("mutating Raw() corrupted the document:\n got: %q\nwant: %q", got, original)
	}
}
