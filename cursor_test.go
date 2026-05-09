package tomledit

import (
	"testing"
)

func TestCursor_ServerHost(t *testing.T) {
	doc := parseTestDoc(t)
	val, ok := doc.Key("server").Key("host").String()
	if !ok {
		t.Fatal("cursor server.host String() returned false")
	}
	if val != "localhost" {
		t.Errorf("expected \"localhost\", got %q", val)
	}
}

func TestCursor_ServerPort(t *testing.T) {
	doc := parseTestDoc(t)
	val, ok := doc.Key("server").Key("port").Int()
	if !ok {
		t.Fatal("cursor server.port Int() returned false")
	}
	if val != 8080 {
		t.Errorf("expected 8080, got %d", val)
	}
}

func TestCursor_Products0Name(t *testing.T) {
	doc := parseTestDoc(t)
	val, ok := doc.Key("products").At(0).Key("name").String()
	if !ok {
		t.Fatal("cursor products[0].name String() returned false")
	}
	if val != "Widget" {
		t.Errorf("expected \"Widget\", got %q", val)
	}
}

func TestCursor_ProductsNeg1Name(t *testing.T) {
	doc := parseTestDoc(t)
	val, ok := doc.Key("products").At(-1).Key("name").String()
	if !ok {
		t.Fatal("cursor products[-1].name String() returned false")
	}
	if val != "Gadget" {
		t.Errorf("expected \"Gadget\", got %q", val)
	}
}

func TestCursor_NestedInlineX(t *testing.T) {
	doc := parseTestDoc(t)
	val, ok := doc.Key("nested").Key("inline").Key("x").Int()
	if !ok {
		t.Fatal("cursor nested.inline.x Int() returned false")
	}
	if val != 1 {
		t.Errorf("expected 1, got %d", val)
	}
}

func TestCursor_NestedArray2(t *testing.T) {
	doc := parseTestDoc(t)
	val, ok := doc.Key("nested").Key("array").At(2).Int()
	if !ok {
		t.Fatal("cursor nested.array[2] Int() returned false")
	}
	if val != 3 {
		t.Errorf("expected 3, got %d", val)
	}
}

func TestCursor_MissingKey(t *testing.T) {
	doc := parseTestDoc(t)
	cursor := doc.Key("nonexistent")
	val, ok := cursor.Key("x").String()
	if ok {
		t.Errorf("expected false for missing key, got (%q, true)", val)
	}
	if cursor.Err() == nil {
		t.Error("expected non-nil error for missing key")
	}
}

func TestCursor_ChainAfterError(t *testing.T) {
	doc := parseTestDoc(t)
	val, ok := doc.Key("nonexistent").Key("x").Key("y").String()
	if ok {
		t.Errorf("expected false after chained error, got (%q, true)", val)
	}
	// Should not panic
}

func TestCursor_CrossTableResolution(t *testing.T) {
	doc := parseTestDoc(t)
	val, ok := doc.Key("server").Key("database").Key("name").String()
	if !ok {
		t.Fatal("cursor server.database.name String() returned false")
	}
	if val != "mydb" {
		t.Errorf("expected \"mydb\", got %q", val)
	}
}

func TestCursor_Node(t *testing.T) {
	doc := parseTestDoc(t)
	cursor := doc.Key("server").Key("host")
	node := cursor.Node()
	if node == nil {
		t.Fatal("Node() returned nil")
	}
	s, ok := node.(*StringNode)
	if !ok {
		t.Fatalf("expected *StringNode, got %T", node)
	}
	if s.Val != "localhost" {
		t.Errorf("expected \"localhost\", got %q", s.Val)
	}
}

func TestCursor_ErrorMessage(t *testing.T) {
	doc := parseTestDoc(t)
	cursor := doc.Key("nonexistent")
	err := cursor.Err()
	if err == nil {
		t.Fatal("expected error for missing key")
	}
	errMsg := err.Error()
	if errMsg == "" {
		t.Error("error message should not be empty")
	}
}
