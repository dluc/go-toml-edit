package tomledit

import (
	"bytes"
	"fmt"
	"testing"
)

var benchInput = []byte(`# Application config
[server]
host = "localhost"
port = 8080
debug = false

[server.database]
host = "db.example.com"
port = 5432
name = "myapp"
max_connections = 100

[[products]]
name = "Widget"
price = 9.99
tags = ["sale", "featured"]

[[products]]
name = "Gadget"
price = 19.99
tags = ["new"]

[metadata]
created = 2024-01-15T10:30:00Z
version = "1.0.0"
`)

func BenchmarkParse(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Parse(benchInput)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBytes(b *testing.B) {
	doc, err := Parse(benchInput)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = doc.Bytes()
	}
}

func BenchmarkGet(b *testing.B) {
	doc, err := Parse(benchInput)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		doc.GetString("server.database.name")
	}
}

func BenchmarkSetExisting(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		doc, _ := Parse(benchInput)
		doc.Set("server.host", "newhost")
		_ = doc.Bytes()
	}
}

func BenchmarkFormat(b *testing.B) {
	doc, err := Parse(benchInput)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = doc.Format()
	}
}

func BenchmarkParseLarge(b *testing.B) {
	var buf bytes.Buffer
	for i := 0; i < 100; i++ {
		fmt.Fprintf(&buf, "[[items]]\nname = \"item_%d\"\nvalue = %d\nenabled = %v\n\n", i, i*100, i%2 == 0)
	}
	input := buf.Bytes()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Parse(input)
	}
}
