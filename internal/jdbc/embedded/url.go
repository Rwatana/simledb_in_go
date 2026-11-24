package embedded

import (
	"fmt"
	"strings"
)

const embeddedURLPrefix = "jdbc:simpledb:"

// ParseEmbeddedURL extracts the database name from a JDBC-compatible embedded URL.
func ParseEmbeddedURL(dataSourceName string) (string, error) {
	if !strings.HasPrefix(dataSourceName, embeddedURLPrefix) {
		return "", fmt.Errorf("parse embedded database URL: expected prefix %q", embeddedURLPrefix)
	}
	name := strings.TrimPrefix(dataSourceName, embeddedURLPrefix)
	if name == "" {
		return "", fmt.Errorf("parse embedded database URL: database name is empty")
	}
	return name, nil
}
