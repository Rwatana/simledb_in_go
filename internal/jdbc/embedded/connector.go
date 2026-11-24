package embedded

import (
	"context"
	"database/sql/driver"
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/server"
)

// Connector creates embedded database connections backed by one server manager set.
type Connector struct {
	managers *server.Managers
}

var _ driver.Connector = (*Connector)(nil)

// NewConnector creates a connector bound to managers.
func NewConnector(managers *server.Managers) (*Connector, error) {
	if managers == nil {
		return nil, fmt.Errorf("create embedded connector: manager set is nil")
	}
	return &Connector{managers: managers}, nil
}

// Connect creates a fresh embedded connection unless the caller's context is canceled.
func (connector *Connector) Connect(ctx context.Context) (driver.Conn, error) {
	if connector == nil {
		return nil, fmt.Errorf("connect embedded database: connector is nil")
	}
	if ctx == nil {
		return nil, fmt.Errorf("connect embedded database: context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("connect embedded database: %w", err)
	}
	connection, err := NewConnection(connector.managers)
	if err != nil {
		return nil, fmt.Errorf("connect embedded database: %w", err)
	}
	return connection, nil
}

// Driver returns a driver whose Open method uses this connector's manager set.
func (connector *Connector) Driver() driver.Driver {
	return &connectorDriver{connector: connector}
}

type connectorDriver struct {
	connector *Connector
}

var _ driver.Driver = (*connectorDriver)(nil)

func (driver *connectorDriver) Open(_ string) (driver.Conn, error) {
	if driver == nil {
		return nil, fmt.Errorf("open embedded driver connection: driver is nil")
	}
	if driver.connector == nil {
		return nil, fmt.Errorf("open embedded driver connection: connector is nil")
	}
	connection, err := driver.connector.Connect(context.Background())
	if err != nil {
		return nil, fmt.Errorf("open embedded driver connection: %w", err)
	}
	return connection, nil
}
