package network

import "testing"

func TestParseNetworkURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{name: "localhost", url: "jdbc:simpledb://localhost", want: "localhost"},
		{name: "dns name preserves case", url: "jdbc:simpledb://Db.Example.org", want: "Db.Example.org"},
		{name: "ipv4", url: "jdbc:simpledb://127.0.0.1", want: "127.0.0.1"},
		{name: "bracketed ipv6", url: "jdbc:simpledb://[2001:db8::1]", want: "2001:db8::1"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseNetworkURL(test.url)
			if err != nil {
				t.Fatalf("ParseNetworkURL(%q) error = %v", test.url, err)
			}
			if got != test.want {
				t.Errorf("ParseNetworkURL(%q) = %q, want %q", test.url, got, test.want)
			}
		})
	}
}

func TestParseNetworkURLRejectsMalformedDataSourceNames(t *testing.T) {
	invalid := []string{
		"",
		"localhost",
		"jdbc:simpledb://",
		"jdbc:simpledb://:",
		"jdbc:simpledb://localhost:1099",
		"jdbc:simpledb://localhost:",
		"jdbc:simpledb://[2001:db8::1]:",
		"jdbc:simpledb://localhost/database",
		"jdbc:simpledb://localhost?database=studentdb",
		"jdbc:simpledb://localhost#database",
		"jdbc:simpledb://localhost#",
		"jdbc:simpledb://user@localhost",
		"jdbc:simpledb://[2001:db8::1",
		"jdbc:simpledb:localhost",
		"jdbc:other://localhost",
	}
	for _, url := range invalid {
		if host, err := ParseNetworkURL(url); err == nil || host != "" {
			t.Errorf("ParseNetworkURL(%q) = (%q, %v), want an error and no host", url, host, err)
		}
	}
}
