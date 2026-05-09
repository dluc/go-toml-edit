package tomledit

import "testing"

func TestParsePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		want    []pathSegment
		wantErr bool
	}{
		{
			name: "simple dotted key",
			path: "server.host",
			want: []pathSegment{
				{Type: keySegment, Key: "server"},
				{Type: keySegment, Key: "host"},
			},
		},
		{
			name: "key with index",
			path: "products[0].name",
			want: []pathSegment{
				{Type: keySegment, Key: "products"},
				{Type: indexSegment, Index: 0},
				{Type: keySegment, Key: "name"},
			},
		},
		{
			name: "negative index",
			path: "products[-1]",
			want: []pathSegment{
				{Type: keySegment, Key: "products"},
				{Type: indexSegment, Index: -1},
			},
		},
		{
			name: "adjacent brackets",
			path: "matrix[0][1]",
			want: []pathSegment{
				{Type: keySegment, Key: "matrix"},
				{Type: indexSegment, Index: 0},
				{Type: indexSegment, Index: 1},
			},
		},
		{
			name: "quoted key with dot",
			path: `server."host.name"`,
			want: []pathSegment{
				{Type: keySegment, Key: "server"},
				{Type: keySegment, Key: "host.name"},
			},
		},
		{
			name: "escaped dot",
			path: `server.host\.name`,
			want: []pathSegment{
				{Type: keySegment, Key: "server"},
				{Type: keySegment, Key: "host.name"},
			},
		},
		{
			name: "single key",
			path: "title",
			want: []pathSegment{
				{Type: keySegment, Key: "title"},
			},
		},
		{
			name: "deeply nested",
			path: "a.b.c.d",
			want: []pathSegment{
				{Type: keySegment, Key: "a"},
				{Type: keySegment, Key: "b"},
				{Type: keySegment, Key: "c"},
				{Type: keySegment, Key: "d"},
			},
		},
		{
			name: "index only",
			path: "[0]",
			want: []pathSegment{
				{Type: indexSegment, Index: 0},
			},
		},
		{
			name: "multiple indices after key",
			path: "a[0][1][2]",
			want: []pathSegment{
				{Type: keySegment, Key: "a"},
				{Type: indexSegment, Index: 0},
				{Type: indexSegment, Index: 1},
				{Type: indexSegment, Index: 2},
			},
		},
		{
			name: "index then key",
			path: "[0].name",
			want: []pathSegment{
				{Type: indexSegment, Index: 0},
				{Type: keySegment, Key: "name"},
			},
		},
		{
			name:    "empty path",
			path:    "",
			wantErr: true,
		},
		{
			name:    "unclosed bracket",
			path:    "a[0",
			wantErr: true,
		},
		{
			name:    "non-numeric index",
			path:    "a[foo]",
			wantErr: true,
		},
		{
			name:    "empty index",
			path:    "a[]",
			wantErr: true,
		},
		{
			name:    "unclosed quote",
			path:    `a."unclosed`,
			wantErr: true,
		},
		{
			name:    "trailing dot",
			path:    "a.b.",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePath(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (result: %v)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != len(tt.want) {
				t.Fatalf("got %d segments, want %d\n  got:  %+v\n  want: %+v", len(got), len(tt.want), got, tt.want)
			}

			for i, w := range tt.want {
				g := got[i]
				if g.Type != w.Type {
					t.Errorf("segment[%d].Type = %v, want %v", i, g.Type, w.Type)
				}
				if w.Type == keySegment && g.Key != w.Key {
					t.Errorf("segment[%d].Key = %q, want %q", i, g.Key, w.Key)
				}
				if w.Type == indexSegment && g.Index != w.Index {
					t.Errorf("segment[%d].Index = %d, want %d", i, g.Index, w.Index)
				}
			}
		})
	}
}
