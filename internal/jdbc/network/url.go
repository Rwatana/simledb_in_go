package network

import (
	"fmt"
	"net/url"
	"strings"
)

const networkURLPrefix = "jdbc:simpledb://"

// ParseNetworkURL extracts the host from a JDBC-compatible network URL.
// The Java driver uses the registry's default port, so URL port overrides are rejected.
func ParseNetworkURL(dataSourceName string) (string, error) {
	if !strings.HasPrefix(dataSourceName, networkURLPrefix) {
		return "", fmt.Errorf("parse network database URL: expected prefix %q", networkURLPrefix)
	}
	rawHost := strings.TrimPrefix(dataSourceName, networkURLPrefix)
	if rawHost == "" {
		return "", fmt.Errorf("parse network database URL: host is empty")
	}
	parsed, err := url.Parse("tcp://" + rawHost)
	if err != nil {
		return "", fmt.Errorf("parse network database URL host: %w", err)
	}
	host := parsed.Hostname()
	if host == "" {
		return "", fmt.Errorf("parse network database URL: host is empty")
	}
	if strings.Contains(rawHost, "#") || parsed.User != nil || parsed.Path != "" || parsed.RawPath != "" || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" || parsed.RawFragment != "" {
		return "", fmt.Errorf("parse network database URL: host must not include user information, path, query, or fragment")
	}
	if parsed.Port() != "" || hasExplicitPort(rawHost) {
		return "", fmt.Errorf("parse network database URL: explicit ports are not supported")
	}
	return host, nil
}

func hasExplicitPort(host string) bool {
	if strings.HasPrefix(host, "[") {
		return strings.IndexByte(host, ']') != len(host)-1
	}
	return strings.Contains(host, ":")
}
