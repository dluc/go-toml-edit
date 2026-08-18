package tomledit

// This file reproduces 13 confirmed bugs found in an adversarial review of
// go-toml-edit. Each TestAuditBug_* test asserts the CORRECT behavior for its
// finding, so it currently FAILS against the present code -- that failure IS
// the reproduction, documenting the bug for a future one-at-a-time fix pass.
//
// TEST-ONLY: no product .go file is modified by this file. Do not "fix" a
// failure here by weakening its assertion; the assertions describe the
// genuinely correct behavior per the audit findings below.

import (
	"math"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// --- shared subprocess harness (used by C-1 and C-4) ---

// auditRunGuardedSubprocess re-execs the current test binary, filtered to run
// only testName, with envKey=1 set in its environment (the test function
// itself checks envKey at the very top and, if set, performs the dangerous
// call directly before calling os.Exit(0) -- see each test body).
//
// It never blocks the parent test process indefinitely: if the child does
// not exit within timeout, it is killed and timedOut=true is returned. This
// guarantees the overall suite cannot hang or CPU-peg on a reproduced bug.
func auditRunGuardedSubprocess(t *testing.T, testName, envKey string, timeout time.Duration) (timedOut bool, waitErr error, output string) {
	t.Helper()

	cmd := exec.Command(os.Args[0], "-test.run=^"+testName+"$")
	cmd.Env = append(os.Environ(), envKey+"=1")
	var sb strings.Builder
	cmd.Stdout = &sb
	cmd.Stderr = &sb

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start subprocess: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		return false, err, sb.String()
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		<-done // reap the process so it doesn't become a zombie
		return true, nil, sb.String()
	}
}

// =====================================================================
// C-1 (Critical) -- parsePath infinite hang on a leading-dot path.
// path.go: parsePath's main loop (~line 42-73) only advances past a '.'
// separator "if i > 0 && path[i] == '.'" (line 44); at i==0 a leading '.'
// falls through to parseBareKey, which immediately breaks on '.' (line 138)
// without consuming any byte and returns newI == i. The outer loop then
// re-enters with the same i forever: `d.Get(".a")` never returns.
// Expected: Get returns promptly (nil, since the path is invalid/not
// found) -- never hangs. Actual: infinite loop (CPU-pegged, no return).
// =====================================================================

func TestAuditBug_C1_ParsePathLeadingDotHang(t *testing.T) {
	const envKey = "TOMLEDIT_AUDIT_C1_CHILD"
	if os.Getenv(envKey) == "1" {
		// Child: perform the hanging call directly. If parsePath is fixed,
		// this returns quickly and we exit 0.
		d, err := Parse([]byte("x = 1\n"))
		if err != nil {
			os.Exit(1)
		}
		_ = d.Get(".a")
		os.Exit(0)
	}

	timedOut, waitErr, output := auditRunGuardedSubprocess(t, "TestAuditBug_C1_ParsePathLeadingDotHang", envKey, 5*time.Second)
	if timedOut {
		t.Errorf("BUG C-1: parsePath hangs on leading-dot path %q -- Get(\".a\") did not return within 5s (child killed)\nchild output so far:\n%s", ".a", output)
		return
	}
	if waitErr != nil {
		t.Errorf("child process for C-1 exited with error: %v\noutput:\n%s", waitErr, output)
	}
}

// =====================================================================
// C-2 (Critical) -- Unmarshal reflection panics on map-typed struct fields
// whose element/key type doesn't match the map being assigned into.
// unmarshal.go: ensureSubMap (~line 1093-1111) calls
// `m.SetMapIndex(key, sub)` where `sub := reflect.MakeMap(m.Type())` --
// i.e. sub is created with m's OWN map type, not m's element type. When m
// is itself the element of an outer map (map[string]map[string]int), this
// mismatches and reflect panics. Separately, decodeDottedKVIntoMap
// (~line 330-348) calls `current.SetMapIndex(keyVal, reflect.ValueOf(val))`
// with a decoded interface{} value that is never checked for assignability
// against the map's key/value types, panicking whenever a string TOML key
// or value doesn't match a non-string map key/value type.
// Expected: Unmarshal never panics; it either succeeds with correct values
// or returns a descriptive error. Actual: reflect panics propagate out of
// Unmarshal.
// =====================================================================

func TestAuditBug_C2_UnmarshalPanics(t *testing.T) {
	t.Run("nested_map_of_map", func(t *testing.T) {
		// C-2a: map[string]map[string]int decoded from [a]\nb = 1\n.
		var m map[string]map[string]int
		var r any
		func() {
			defer func() { r = recover() }()
			_ = Unmarshal([]byte("[a]\nb = 1\n"), &m)
		}()
		if r != nil {
			t.Errorf("BUG C-2a: Unmarshal into map[string]map[string]int panicked: %v", r)
			return
		}
		if m == nil || m["a"]["b"] != 1 {
			t.Errorf("BUG C-2a: expected m[\"a\"][\"b\"] == 1 with no panic, got m = %#v", m)
		}
	})

	t.Run("dotted_key_into_map_int_string_field", func(t *testing.T) {
		// C-2b: struct field map[int]string; TOML supplies a string key
		// ("bar") via a dotted key, which cannot become an int map key.
		type cfg struct {
			Foo map[int]string
		}
		var c cfg
		var r any
		var err error
		func() {
			defer func() { r = recover() }()
			err = Unmarshal([]byte("foo.bar = \"hi\"\n"), &c)
		}()
		if r != nil {
			t.Errorf("BUG C-2b: Unmarshal into struct with map[int]string field panicked: %v", r)
			return
		}
		if err == nil {
			t.Errorf("BUG C-2b: expected a non-nil error decoding dotted string key into map[int]string, got nil (c = %#v)", c)
		}
	})

	t.Run("dotted_key_value_type_mismatch_into_map", func(t *testing.T) {
		// C-2b variant: struct field map[string]int; TOML value is a
		// string ("nope"), which cannot become an int map value.
		type cfg2 struct {
			Foo map[string]int
		}
		var c cfg2
		var r any
		var err error
		func() {
			defer func() { r = recover() }()
			err = Unmarshal([]byte("foo.bar = \"nope\"\n"), &c)
		}()
		if r != nil {
			t.Errorf("BUG C-2b (value mismatch): Unmarshal into struct with map[string]int field panicked: %v", r)
			return
		}
		if err == nil {
			t.Errorf("BUG C-2b (value mismatch): expected a non-nil error decoding string value into map[string]int, got nil (c = %#v)", c)
		}
	})
}

// =====================================================================
// C-3 (Critical) -- Merge drops array-of-tables data and flattens
// [[array-table]] entries to plain [table] headers.
// merge.go: mergeChildren (~line 74-210) processes source.Children in a
// single pass over `*TableNode` and `*ArrayTableNode` in document order,
// but table entries are applied immediately via mergeSubTable/SetCreate
// while array-table entries are deferred into `arrayTableGroups` and only
// applied at the end (~line 195-207), gated on
// `target.Get(group.subPrefix) != nil`. When a nested sub-table (e.g.
// [fruits.physical]) is processed BEFORE its owning array-table group,
// SetCreate auto-creates a plain intermediate TableNode at "fruits" as a
// side effect (edit.go createIntermediateTable). That intermediate table
// then makes `target.Get("fruits")` non-nil by the time the array-table
// group for "fruits" is processed, so the "already has entries: keep it
// (atomic)" branch (line 197-200) skips copying the actual [[fruits]]
// entry (with name = "apple") entirely, and the surviving artifact is a
// flattened [fruits]/[fruits.physical] pair instead of [[fruits]].
// Expected: after Merge, the array-of-tables entry and its data survive,
// and the output still re-parses. Actual: "apple" is silently dropped and
// [[fruits]] is flattened to [fruits].
// =====================================================================

func TestAuditBug_C3_MergeDropsArrayOfTables(t *testing.T) {
	target := mustParse(t, "title = \"empty\"\n")
	source := mustParse(t, "[[fruits]]\nname = \"apple\"\n\n[fruits.physical]\ncolor = \"red\"\n")

	if err := target.Merge(source); err != nil {
		t.Fatalf("Merge returned error: %v", err)
	}

	out := string(target.Bytes())

	if _, err := Parse([]byte(out)); err != nil {
		t.Fatalf("BUG C-3: merged output does not re-parse: %v\noutput:\n%s", err, out)
	}

	if !strings.Contains(out, "[[fruits]]") {
		t.Errorf("BUG C-3: merged output is missing \"[[fruits]]\" (array-of-tables was flattened to a plain table)\noutput:\n%s", out)
	}
	if !strings.Contains(out, `name = "apple"`) {
		t.Errorf("BUG C-3: merged output is missing `name = \"apple\"` (array-of-tables data was dropped)\noutput:\n%s", out)
	}
	if !strings.Contains(out, `color = "red"`) {
		t.Errorf("BUG C-3: merged output is missing `color = \"red\"`\noutput:\n%s", out)
	}
}

// =====================================================================
// C-4 (Critical) -- Marshal crashes (stack overflow) on a cyclic map.
// marshal.go: valueToNode (edit.go, case map[string]any -> line 793-794)
// and mapToInlineTableNode (edit.go ~line 824-843) recurse into nested
// map[string]any values with no cycle/depth detection whatsoever. A
// self-referential map (m["self"] = m) recurses forever until the Go
// runtime's stack guard triggers a fatal, unrecoverable "stack overflow"
// error that terminates the process (recover() cannot catch it).
// Expected: Marshal returns a (nil, error) pair -- no crash. Actual: the
// process crashes with a fatal stack-overflow error.
// =====================================================================

func TestAuditBug_C4_MarshalCyclicMapCrash(t *testing.T) {
	const envKey = "TOMLEDIT_AUDIT_C4_CHILD"
	if os.Getenv(envKey) == "1" {
		m := map[string]any{}
		m["self"] = m
		_, _ = Marshal(m)
		os.Exit(0)
	}

	timedOut, waitErr, output := auditRunGuardedSubprocess(t, "TestAuditBug_C4_MarshalCyclicMapCrash", envKey, 5*time.Second)
	if timedOut {
		t.Errorf("BUG C-4: Marshal(cyclic map) did not return within 5s (child killed) -- expected a fast crash or a fast error return\noutput so far:\n%s", output)
		return
	}
	if waitErr != nil || strings.Contains(output, "stack overflow") {
		t.Errorf("BUG C-4: Marshal crashes on a cyclic map (child exit error=%v)\nchild output:\n%s", waitErr, output)
	}
}

// =====================================================================
// H-1 (High) -- Set on a key name that already names a [table] mis-scopes.
// edit.go: setInternal (~line 30-63) resolves the parent for "server" as
// the DOCUMENT ROOT (empty parentSegs) rather than detecting that "server"
// already exists as a TableNode, then setKeyInParent/setKeyInChildren
// (~line 193-232) appends a brand-new top-level KV named "server" because
// no top-level *KeyValueNode* named "server" exists yet (the existing
// "server" is a *TableNode*, which setKeyInChildren's search doesn't
// consider a collision). The new KV ends up serialized directly after the
// existing [server] table with no intervening header, so on re-parse it is
// silently swallowed as a *nested* key inside the [server] table (i.e.
// path "server.server"), not a new top-level "server" key.
// Expected (fix-agnostic): either Set errors out (never silently
// mis-scopes), or the table is genuinely replaced -- i.e. after Set,
// GetString("server") == "oops" and GetString("server.host") is gone.
// Actual: Set returns nil, and the re-parsed document has neither.
// =====================================================================

func TestAuditBug_H1_SetOnTableNameMisscopes(t *testing.T) {
	doc := mustParse(t, "[server]\nhost = \"localhost\"\n")

	err := doc.Set("server", "oops")
	if err != nil {
		// Erroring out is an acceptable fix per the design-agnostic
		// invariant below (never silently mis-scope) -- nothing more to
		// check in that case.
		return
	}

	out := doc.Bytes()
	reparsed, perr := Parse(out)
	if perr != nil {
		t.Fatalf("BUG H-1: Set(\"server\", \"oops\") produced output that does not re-parse: %v\noutput:\n%s", perr, out)
	}

	val, ok := reparsed.GetString("server")
	hostVal, hostOK := reparsed.GetString("server.host")
	if !(ok && val == "oops" && !hostOK) {
		t.Errorf("BUG H-1: Set(\"server\", \"oops\") on a [table] named \"server\" mis-scopes:\n"+
			"  got  GetString(\"server\")      = (%q, %v)\n"+
			"  got  GetString(\"server.host\") = (%q, %v)\n"+
			"  want GetString(\"server\")      = (\"oops\", true)\n"+
			"  want GetString(\"server.host\") = (_, false)\n"+
			"output:\n%s", val, ok, hostVal, hostOK, out)
	}
}

// =====================================================================
// H-2 (High) -- NewTable / NewArrayTable / Rename lack cross-kind
// collision detection, silently producing invalid (unparseable) TOML with
// a nil error.
// edit.go: NewTable's existence check (~line 620-627) only scans for an
// existing *TableNode* with the same path; NewArrayTable (~line 647-694)
// performs no existence/collision check against *TableNode* or scalar KVs
// at all; Rename's duplicate check (~line 573-580) only scans sibling
// *KeyValueNode* entries, ignoring sibling *TableNode*/*ArrayTableNode*
// headers. In all three cases the resulting document round-trips through
// Bytes() into text that the package's OWN parser (parser.go's
// definitionTracker, ~line 1550-1802) rejects as a genuine TOML conflict
// (table/array-table/value redefinition) -- proving the edit API allowed
// the document to enter an invalid state without any error.
// Expected: either the edit call returns an error, or its output re-parses
// cleanly. Actual: the call returns nil and the output fails to re-parse.
// =====================================================================

func TestAuditBug_H2_CrossKindCollisionNoDetection(t *testing.T) {
	assertNoSilentCollision := func(t *testing.T, label string, err error, out []byte) {
		t.Helper()
		if err != nil {
			return // erroring out is a valid fix
		}
		if _, perr := Parse(out); perr != nil {
			t.Errorf("BUG H-2 (%s): call returned nil error but output does not re-parse: %v\noutput:\n%s", label, perr, out)
		}
	}

	t.Run("NewArrayTable_over_existing_table", func(t *testing.T) {
		doc := mustParse(t, "[server]\nhost = \"h\"\n")
		err := doc.NewArrayTable("server")
		assertNoSilentCollision(t, "NewArrayTable", err, doc.Bytes())
	})

	t.Run("NewTable_over_existing_scalar", func(t *testing.T) {
		doc := mustParse(t, "server = \"scalar\"\n")
		err := doc.NewTable("server")
		assertNoSilentCollision(t, "NewTable", err, doc.Bytes())
	})

	t.Run("Rename_over_existing_table", func(t *testing.T) {
		doc := mustParse(t, "name = \"x\"\n\n[server]\nhost = \"h\"\n")
		err := doc.Rename("name", "server")
		assertNoSilentCollision(t, "Rename", err, doc.Bytes())
	})
}

// =====================================================================
// H-3 (High) -- a hand-built Node value passed to Set renders as empty,
// producing invalid TOML.
// edit.go: valueToNode (~line 698-808) short-circuits with
// `if n, ok := v.(Node); ok { return n, nil }` (line 704-706) and returns
// the caller's node completely as-is -- it is never marked dirty and has
// no Raw() bytes (Raw() is nil on a freshly-constructed node). render.go's
// serializeNode default case (~line 79-82) treats "not dirty and no dirty
// descendants" as "safe to emit Raw() verbatim", so a hand-built leaf node
// serializes as an empty byte string. The same applies to hand-built Node
// elements inside a []any passed to Set (sliceToArrayNode, edit.go
// ~line 811-822, has the identical pass-through for elements that already
// implement Node).
// Expected: Set accepts a hand-built Node and renders its real value (or
// rejects it with an error) -- it must never silently emit an empty
// value. Actual: `x = 1` becomes `x = ` and `arr = [1, 2]` becomes
// `arr = [, ]`, both invalid TOML.
// =====================================================================

func TestAuditBug_H3_HandBuiltNodeRendersEmpty(t *testing.T) {
	t.Run("scalar_string_node", func(t *testing.T) {
		doc := mustParse(t, "x = 1\n")
		if err := doc.Set("x", &StringNode{Val: "hello", Style: StringBasic}); err != nil {
			t.Fatalf("Set returned error: %v", err)
		}
		out := doc.Bytes()
		reparsed, perr := Parse(out)
		if perr != nil {
			t.Errorf("BUG H-3 (scalar): hand-built StringNode rendered invalid TOML: %v\noutput: %q", perr, out)
			return
		}
		val, ok := reparsed.GetString("x")
		if !ok || val != "hello" {
			t.Errorf("BUG H-3 (scalar): got GetString(\"x\") = (%q, %v), want (\"hello\", true)\noutput: %q", val, ok, out)
		}
	})

	t.Run("array_of_string_nodes", func(t *testing.T) {
		doc := mustParse(t, "arr = [1, 2]\n")
		vals := []any{
			&StringNode{Val: "a", Style: StringBasic},
			&StringNode{Val: "b", Style: StringBasic},
		}
		if err := doc.Set("arr", vals); err != nil {
			t.Fatalf("Set returned error: %v", err)
		}
		out := doc.Bytes()
		if _, perr := Parse(out); perr != nil {
			t.Errorf("BUG H-3 (array): hand-built StringNode array elements rendered invalid TOML: %v\noutput: %q", perr, out)
		}
	})
}

// =====================================================================
// H-4 (High) -- embedded-field shadowing is backwards on decode.
// unmarshal.go: collectFields (~line 174-217) iterates struct fields in
// declaration order and, for an anonymous embedded struct field (line
// 189-199), immediately recurses and registers the embedded field's names
// in `exact`/`lower` via `if _, exists := exact[name]; !exists { ... }`
// (line 207-215) -- first writer wins. Because Go places "outer.Name"
// AFTER the embedded "inner" field in field-index order (embedded fields
// are processed in the same loop, at their own index, before later
// fields), the embedded inner.Name registers in `exact["Name"]` first,
// and outer's own directly-declared Name field is then skipped by the
// `!exists` guard. This is backwards from Go's own embedding/promotion
// rule: a shallower (less-embedded) field must shadow a promoted one.
// Expected: decoding "name" sets the OUTER struct's own Name field, and
// leaves the embedded inner.Name untouched. Actual: it sets inner.Name and
// leaves outer.Name untouched -- the opposite of Go's shadowing rule.
// =====================================================================

func TestAuditBug_H4_EmbeddedFieldShadowing(t *testing.T) {
	// The embedded type must itself be EXPORTED (capitalized). With an
	// unexported embedded type name (e.g. `inner`), reflect.StructField's
	// own Name is the type name itself, so IsExported() is false and
	// collectFields's `if !f.IsExported() { continue }` guard (line 177-179)
	// skips the embedded field entirely before ever reaching the promotion
	// logic -- that sidesteps this bug rather than reproducing it. Only an
	// exported embedded type name (`Inner`) reaches the shadowing logic.
	type Inner struct {
		Name string
	}
	type outer struct {
		Inner
		Name string
	}

	var o outer
	if err := Unmarshal([]byte("name = \"outer_value\"\n"), &o); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}

	if !(o.Name == "outer_value" && o.Inner.Name == "") {
		t.Errorf("BUG H-4: embedded-field shadowing precedence is reversed:\n"+
			"  got  o.Name = %q, o.Inner.Name = %q\n"+
			"  want o.Name = %q, o.Inner.Name = %q (shallower field must win)",
			o.Name, o.Inner.Name, "outer_value", "")
	}
}

// =====================================================================
// M-1 (Medium) -- Delete silently no-ops on a shared dotted-key prefix.
// document.go: when two sibling dotted keys share a prefix (a.b and a.c),
// resolveKeyInKVList (~line 446-467) returns a *dottedKeyGroup (not a
// *TableNode/*DocumentNode/etc.) as the resolved "parent" for path "a".
// edit.go's deleteKeyFromParent (~line 390-411) switches on the parent's
// concrete type but has no case for *dottedKeyGroup, so it falls into the
// `default:` branch (line 407-409), a silent no-op returning nil.
// Expected: Delete("a.b") removes only the "a.b" entry, leaving "a.c"
// intact, and returns nil. Actual: Delete returns nil but changes nothing
// -- "a.b" is still 1 afterward.
// =====================================================================

func TestAuditBug_M1_DeleteSilentNoopSharedPrefix(t *testing.T) {
	doc := mustParse(t, "a.b = 1\na.c = 2\n")

	if err := doc.Delete("a.b"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	_, bOK := doc.GetInt("a.b")
	cVal, cOK := doc.GetInt("a.c")

	if _, perr := Parse(doc.Bytes()); perr != nil {
		t.Fatalf("BUG M-1: output does not re-parse after Delete: %v", perr)
	}

	if !(!bOK && cOK && cVal == 2) {
		t.Errorf("BUG M-1: Delete(\"a.b\") on a shared dotted-key prefix silently no-ops:\n"+
			"  got  GetInt(\"a.b\") ok=%v (want ok=false)\n"+
			"  got  GetInt(\"a.c\") = (%d, %v) (want (2, true))", bOK, cVal, cOK)
	}
}

// =====================================================================
// M-2 (Medium) -- Set errors on an existing single 2-part dotted key.
// edit.go: setKeyInParent's *dottedKeyView case (~line 203-209) only
// handles the case where partIndex has already run past the end of the
// key's parts (delegating into the value, for a deeper dotted chain); for
// the common case where partIndex already points at the FINAL part (e.g.
// "a.b" resolved to a dottedKeyView with partIndex==1==len(Parts)-1), it
// falls through to `return fmt.Errorf("cannot set key %q: intermediate
// dotted key view", key)` (line 209) even though "b" IS the terminal
// segment, not an intermediate one.
// Expected: Set("a.b", 42) on `a.b = 1` succeeds and updates the value.
// Actual: Set returns a "cannot set key" error and nothing changes.
// =====================================================================

func TestAuditBug_M2_SetErrorsOnExistingDottedKey(t *testing.T) {
	doc := mustParse(t, "a.b = 1\n")

	err := doc.Set("a.b", 42)
	if err != nil {
		t.Errorf("BUG M-2: Set(\"a.b\", 42) on an existing single dotted key returned error: %v (want nil)", err)
		return
	}

	val, ok := doc.GetInt("a.b")
	if !ok || val != 42 {
		t.Errorf("BUG M-2: after Set(\"a.b\", 42), GetInt(\"a.b\") = (%d, %v), want (42, true)", val, ok)
	}
	if _, perr := Parse(doc.Bytes()); perr != nil {
		t.Errorf("BUG M-2: output does not re-parse after Set: %v", perr)
	}
}

// =====================================================================
// M-3 (Medium, DESIGN-DEPENDENT) -- Get returns a live container node;
// external mutation of it is silently dropped by Bytes().
// render.go: serializeNode's default leaf case (~line 79-82) and the
// ArrayNode branch of isSubtreeDirty (~line 89-108) only ever consult
// isDirty() flags set via markDirty(); direct field mutation on a node
// returned by Get() (e.g. `a.Elements = a.Elements[1:]`) never calls
// markDirty(), so Bytes() takes the "not dirty" fast path and re-emits the
// ORIGINAL Raw() bytes, silently discarding the caller's mutation.
//
// This finding is DESIGN-DEPENDENT: the eventual fix may be to make
// Get()'s returned containers dirty-tracking-aware (this test then stays
// as written), OR the project may decide the documented contract is
// "Get() returns a read-only view; mutate via Set/Delete instead" (in
// which case this test should be inverted/removed and the contract
// documented instead of changed). Do not "fix" this by picking a
// direction without that decision being made explicitly.
// =====================================================================

func TestAuditBug_M3_GetReturnsLiveContainerMutationDropped(t *testing.T) {
	doc := mustParse(t, "arr = [1, 2, 3]\n")

	node := doc.Get("arr")
	arr, ok := node.(*ArrayNode)
	if !ok {
		t.Fatalf("Get(\"arr\") returned %T, want *ArrayNode", node)
	}
	arr.Elements = arr.Elements[1:]

	out := doc.Bytes()
	reparsed, perr := Parse(out)
	if perr != nil {
		t.Fatalf("output does not re-parse after mutating the live container: %v\noutput: %q", perr, out)
	}

	node2 := reparsed.Get("arr")
	arr2, ok2 := node2.(*ArrayNode)
	if !ok2 {
		t.Fatalf("re-parsed Get(\"arr\") returned %T, want *ArrayNode", node2)
	}
	if len(arr2.Elements) != 2 {
		t.Errorf("BUG M-3: external mutation of the container returned by Get() is dropped by Bytes():\n"+
			"  got  %d elements after re-parse\n"+
			"  want 2 elements (the mutation should have survived)\n"+
			"output: %q", len(arr2.Elements), out)
	}
}

// =====================================================================
// M-4 (Medium) -- renderInteger emits a double negative at math.MinInt64
// for non-decimal bases.
// render.go: renderInteger (~line 271-291) handles negative values for
// IntegerHex/Octal/Binary via `"-0x" + strconv.FormatInt(-n.Val, 16)`
// (and the octal/binary equivalents). For n.Val == math.MinInt64, the
// negation `-n.Val` overflows int64 (there is no positive counterpart)
// and wraps back around to math.MinInt64 itself (still negative), so
// strconv.FormatInt renders ANOTHER leading '-', producing output like
// "-0x-8000000000000000" -- doubly negative and unparseable.
// Expected: rendering IntegerNode{Val: math.MinInt64} in hex/octal/binary
// produces valid, re-parseable TOML that decodes back to math.MinInt64.
// Actual: the output contains a double negative and fails to parse.
// =====================================================================

func TestAuditBug_M4_RenderIntegerDoubleNegativeMinInt64(t *testing.T) {
	cases := []struct {
		name string
		base IntegerBase
	}{
		{"hex", IntegerHex},
		{"octal", IntegerOctal},
		{"binary", IntegerBinary},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rendered := string(renderInteger(&IntegerNode{Val: math.MinInt64, Base: c.base}))
			src := "x = " + rendered + "\n"

			doc, perr := Parse([]byte(src))
			if perr != nil {
				t.Errorf("BUG M-4 (%s): renderInteger(MinInt64) produced invalid TOML %q: %v", c.name, rendered, perr)
				return
			}
			val, ok := doc.GetInt("x")
			if !ok || val != math.MinInt64 {
				t.Errorf("BUG M-4 (%s): rendered %q, decoded (%d, %v), want (%d, true)", c.name, rendered, val, ok, int64(math.MinInt64))
			}
		})
	}
}

// =====================================================================
// M-5 (Medium) -- ParseError.Offset is always 0 for parser-level errors.
// parser.go: errorfAt (~line 101-107) constructs every parser-level
// *ParseError with only Line/Column/Message populated; it never copies
// tok.Offset (Token.Offset exists, token.go ~line 74) into the
// ParseError's Offset field. Every one of the ~20 call sites that build a
// *ParseError directly (~line 1602-1769) has the same omission. Callers
// that want a byte-precise error location (rather than line/column) get 0
// no matter where the error actually occurred.
// Expected: for "a = ,\n" (the offending ',' is at 0-based byte offset 4),
// ParseError.Offset == 4. Actual: ParseError.Offset == 0.
// =====================================================================

func TestAuditBug_M5_ParseErrorOffsetAlwaysZero(t *testing.T) {
	pe := parseErrorFor(t, "a = ,\n")
	if pe.Offset != 4 {
		t.Errorf("BUG M-5: ParseError.Offset = %d, want 4 (the byte offset of the offending ',')", pe.Offset)
	}
}
