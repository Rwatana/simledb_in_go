package embedded

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
)

func TestNewConnectorReturnsFreshDriverConnections(t *testing.T) {
	if connector, err := NewConnector(nil); connector != nil || err == nil || !strings.Contains(err.Error(), "manager") {
		t.Fatalf("NewConnector(nil) = (%v, %v), want manager validation error", connector, err)
	}

	connector, err := NewConnector(newEmbeddedManagers(t))
	if err != nil {
		t.Fatalf("NewConnector() error = %v", err)
	}
	if _, ok := any(connector).(driver.Connector); !ok {
		t.Fatalf("connector type %T does not implement driver.Connector", connector)
	}

	first, err := connector.Connect(context.Background())
	if err != nil {
		t.Fatalf("Connect() first connection error = %v", err)
	}
	second, err := connector.Connect(context.Background())
	if err != nil {
		t.Fatalf("Connect() second connection error = %v", err)
	}
	if first == second {
		t.Fatal("Connect() returned the same connection twice, want fresh connections")
	}
	if _, ok := first.(*Connection); !ok {
		t.Fatalf("Connect() first type = %T, want *Connection", first)
	}
	if err := first.Close(); err != nil {
		t.Errorf("first Close() error = %v", err)
	}
	if err := second.Close(); err != nil {
		t.Errorf("second Close() error = %v", err)
	}
}

func TestConnectorConnectValidatesReceiverContextAndCancellation(t *testing.T) {
	var nilConnector *Connector
	if connection, err := nilConnector.Connect(context.Background()); connection != nil || err == nil || !strings.Contains(err.Error(), "connector") {
		t.Errorf("Connect() on nil receiver = (%v, %v), want connector error", connection, err)
	}

	connector := &Connector{managers: newEmbeddedManagers(t)}
	if connection, err := connector.Connect(nil); connection != nil || err == nil || !strings.Contains(err.Error(), "context") {
		t.Errorf("Connect(nil) = (%v, %v), want context error", connection, err)
	}
	if connection, err := (&Connector{}).Connect(context.Background()); connection != nil || err == nil || !strings.Contains(err.Error(), "manager") {
		t.Errorf("Connect() without managers = (%v, %v), want manager error", connection, err)
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if connection, err := connector.Connect(canceled); connection != nil || !errors.Is(err, context.Canceled) {
		t.Errorf("Connect() with canceled context = (%v, %v), want context.Canceled", connection, err)
	}
}

func TestConnectorDriverOpenAndDatabaseSQLUseConfiguredManagers(t *testing.T) {
	connector, err := NewConnector(newEmbeddedManagers(t))
	if err != nil {
		t.Fatalf("NewConnector() error = %v", err)
	}
	driverValue := connector.Driver()
	if _, ok := driverValue.(driver.Driver); !ok {
		t.Fatalf("Driver() type %T does not implement driver.Driver", driverValue)
	}
	first, err := driverValue.Open("ignored-data-source-name")
	if err != nil {
		t.Fatalf("Driver().Open() first connection error = %v", err)
	}
	second, err := driverValue.Open("")
	if err != nil {
		t.Fatalf("Driver().Open() second connection error = %v", err)
	}
	if first == second {
		t.Fatal("Driver().Open() returned the same connection twice, want fresh connections")
	}
	if err := first.Close(); err != nil {
		t.Errorf("first Close() error = %v", err)
	}
	if err := second.Close(); err != nil {
		t.Errorf("second Close() error = %v", err)
	}

	database := sql.OpenDB(connector)
	database.SetMaxOpenConns(1)
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Errorf("database Close() error = %v", err)
		}
	})
	transaction, err := database.Begin()
	if err != nil {
		t.Fatalf("database Begin() error = %v", err)
	}
	if err := transaction.Commit(); err != nil {
		t.Fatalf("database Commit() error = %v", err)
	}
	transaction, err = database.Begin()
	if err != nil {
		t.Fatalf("database Begin() after commit error = %v", err)
	}
	if err := transaction.Rollback(); err != nil {
		t.Fatalf("database Rollback() error = %v", err)
	}
}

func TestConnectorDriverRejectsInvalidState(t *testing.T) {
	if connection, err := (*connectorDriver)(nil).Open(""); connection != nil || err == nil || !strings.Contains(err.Error(), "driver") {
		t.Errorf("Open() on nil driver = (%v, %v), want driver error", connection, err)
	}
	if connection, err := (&connectorDriver{}).Open(""); connection != nil || err == nil || !strings.Contains(err.Error(), "connector") {
		t.Errorf("Open() without connector = (%v, %v), want connector error", connection, err)
	}
	if connection, err := (&connectorDriver{connector: &Connector{}}).Open(""); connection != nil || err == nil || !strings.Contains(err.Error(), "open embedded driver connection") {
		t.Errorf("Open() with invalid connector state = (%v, %v), want contextual connection error", connection, err)
	}
	if connection, err := ((*Connector)(nil)).Driver().Open(""); connection != nil || err == nil || !strings.Contains(err.Error(), "connector") {
		t.Errorf("nil Connector.Driver().Open() = (%v, %v), want connector error", connection, err)
	}
}
