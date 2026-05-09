package tomledit

import (
	"bytes"
	"testing"
)

func FuzzParse(f *testing.F) {
	// Seed corpus with various valid TOML inputs
	f.Add([]byte(`key = "value"` + "\n"))
	f.Add([]byte("[table]\nkey = 42\n"))
	f.Add([]byte(`[[array]]` + "\nname = \"item\"\n"))
	f.Add([]byte(`key = {a = 1, b = "two"}` + "\n"))
	f.Add([]byte("key = [1, 2, 3]\n"))
	f.Add([]byte("key = 2024-01-15T10:30:00Z\n"))
	f.Add([]byte(`key = """multi\nline"""` + "\n"))
	f.Add([]byte("# just a comment\n"))
	f.Add([]byte(""))
	f.Add([]byte("key = true\n"))
	f.Add([]byte("key = false\n"))
	f.Add([]byte("key = 3.14\n"))
	f.Add([]byte("key = 0xff\n"))
	f.Add([]byte("key = 0o77\n"))
	f.Add([]byte("key = 0b1010\n"))
	f.Add([]byte("key = 'literal string'\n"))
	f.Add([]byte("key = '''\nmulti-line\nliteral\n'''\n"))
	f.Add([]byte("key = 1979-05-27T07:32:00\n"))
	f.Add([]byte("key = 1979-05-27\n"))
	f.Add([]byte("key = 07:32:00\n"))
	f.Add([]byte(`"quoted key" = "value"` + "\n"))
	f.Add([]byte("a.b.c = \"dotted\"\n"))
	f.Add([]byte("[a.b.c]\nkey = \"deep\"\n"))
	f.Add([]byte("\r\nkey = \"crlf\"\r\n"))
	f.Add([]byte("  key  =  \"spaces\"  \n"))
	f.Add([]byte("key = +inf\n"))
	f.Add([]byte("key = -inf\n"))
	f.Add([]byte("key = nan\n"))
	f.Add([]byte("key = +nan\n"))
	f.Add([]byte("key = -0\n"))
	f.Add([]byte("key = +0\n"))
	f.Add([]byte(`"" = "empty key"` + "\n"))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Parse must not panic on any input
		Parse(data)
	})
}

func FuzzRoundTrip(f *testing.F) {
	// Seed corpus with various valid TOML inputs
	f.Add([]byte(`key = "value"` + "\n"))
	f.Add([]byte("[table]\nkey = 42\n"))
	f.Add([]byte(`[[array]]` + "\nname = \"item\"\n"))
	f.Add([]byte(`key = {a = 1, b = "two"}` + "\n"))
	f.Add([]byte("key = [1, 2, 3]\n"))
	f.Add([]byte("key = 2024-01-15T10:30:00Z\n"))
	f.Add([]byte(`key = """multi\nline"""` + "\n"))
	f.Add([]byte("# just a comment\n"))
	f.Add([]byte(""))
	f.Add([]byte("key = true\n"))
	f.Add([]byte("key = false\n"))
	f.Add([]byte("key = 3.14\n"))
	f.Add([]byte("key = 0xff\n"))
	f.Add([]byte("key = 0o77\n"))
	f.Add([]byte("key = 0b1010\n"))
	f.Add([]byte("key = 'literal string'\n"))
	f.Add([]byte("key = '''\nmulti-line\nliteral\n'''\n"))
	f.Add([]byte("key = 1979-05-27T07:32:00\n"))
	f.Add([]byte("key = 1979-05-27\n"))
	f.Add([]byte("key = 07:32:00\n"))
	f.Add([]byte(`"quoted key" = "value"` + "\n"))
	f.Add([]byte("a.b.c = \"dotted\"\n"))
	f.Add([]byte("[a.b.c]\nkey = \"deep\"\n"))
	f.Add([]byte("[server]\nhost = \"localhost\"\nport = 8080\n\n[[products]]\nname = \"Widget\"\nprice = 9.99\n"))
	f.Add([]byte(`"" = "empty key"` + "\n"))

	f.Fuzz(func(t *testing.T, data []byte) {
		doc, err := Parse(data)
		if err != nil {
			return // Invalid input, skip
		}

		// Bytes() must not panic
		out := doc.Bytes()

		// Output must be valid TOML that parses
		doc2, err := Parse(out)
		if err != nil {
			t.Fatalf("round-trip produced invalid TOML: %v\ninput:  %q\noutput: %q", err, data, out)
		}

		// Second round-trip must be stable (Bytes of Bytes must equal Bytes)
		out2 := doc2.Bytes()
		if !bytes.Equal(out, out2) {
			t.Fatalf("round-trip not stable:\nfirst:  %q\nsecond: %q", out, out2)
		}
	})
}
