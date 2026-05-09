package tomledit

import (
	"strings"
	"testing"
)

func TestDottedKeyThenTableRedefine(t *testing.T) {
	// a.b.c = 1 makes a.b implicit
	// [a.b] promotes it to explicit
	// then c = 2 under [a.b] should conflict with the already-defined c
	_, err := Parse([]byte("a.b.c = 1\n[a.b]\nc = 2\n"))
	if err == nil {
		t.Fatal("expected duplicate key error for a.b.c")
	}
	if !strings.Contains(err.Error(), "duplicate") && !strings.Contains(err.Error(), "already defined") {
		t.Logf("error message: %v", err)
	}
}

func TestDottedKeyThenTableNewKey(t *testing.T) {
	// a.b.c = 1 defines a.b via dotted key.
	// [a.b] tries to reopen it with a table header, which is invalid
	// per TOML spec: "Since the table has already been specified using
	// dotted keys, it cannot be directly defined using a header."
	_, err := Parse([]byte("a.b.c = 1\n[a.b]\nd = 2\n"))
	if err == nil {
		t.Fatal("expected error: dotted-key-defined table cannot be reopened with [table] header")
	}
}
