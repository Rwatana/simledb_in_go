package embedded

import "testing"

func TestParseEmbeddedURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{name: "relative database name", url: "jdbc:simpledb:studentdb", want: "studentdb"},
		{name: "relative database path", url: "jdbc:simpledb:databases/studentdb", want: "databases/studentdb"},
		{name: "absolute database path", url: "jdbc:simpledb:/var/lib/simpledb/studentdb", want: "/var/lib/simpledb/studentdb"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseEmbeddedURL(test.url)
			if err != nil {
				t.Fatalf("ParseEmbeddedURL(%q) error = %v", test.url, err)
			}
			if got != test.want {
				t.Errorf("ParseEmbeddedURL(%q) = %q, want %q", test.url, got, test.want)
			}
		})
	}
}

func TestParseEmbeddedURLRejectsUnsupportedAndEmptyNames(t *testing.T) {
	for _, url := range []string{"", "studentdb", "jdbc:other:studentdb", "jdbc:simpledb:"} {
		if name, err := ParseEmbeddedURL(url); err == nil || name != "" {
			t.Errorf("ParseEmbeddedURL(%q) = (%q, %v), want an error and no name", url, name, err)
		}
	}
}
