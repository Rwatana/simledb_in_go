package materialize

import (
	"errors"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

func TestMultibufferProductPlanOpenMaterializesRightAndOwnsOpenedLeftScan(t *testing.T) {
	leftSchema := &record.Schema{}
	leftSchema.AddIntegerField("left_id")
	leftSchema.AddVarcharField("left_name", 16)
	rightSchema := &record.Schema{}
	rightSchema.AddIntegerField("right_id")
	rightSchema.AddVarcharField("right_name", 16)
	left := &materializePlan{schema: leftSchema, rows: [][]query.Constant{{query.NewIntConstant(1), query.NewStringConstant("Ada")}, {query.NewIntConstant(2), query.NewStringConstant("Lin")}}}
	right := &materializePlan{schema: rightSchema, rows: [][]query.Constant{{query.NewIntConstant(10), query.NewStringConstant("Math")}, {query.NewIntConstant(20), query.NewStringConstant("History")}}}
	transaction := newMaterializePlanTransaction()

	scan, err := NewMultibufferProductPlan(transaction, left, right).Open()
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if scan.HasField("left_name") != true || scan.HasField("right_name") != true || scan.HasField("missing") {
		t.Errorf("HasField returned an incorrect product schema")
	}
	var got [][2]int32
	for {
		found, err := scan.Next()
		if err != nil {
			t.Fatalf("Next() error = %v", err)
		}
		if !found {
			break
		}
		leftValue, err := scan.GetInt("left_id")
		if err != nil {
			t.Fatalf("GetInt(left_id) error = %v", err)
		}
		rightValue, err := scan.GetInt("right_id")
		if err != nil {
			t.Fatalf("GetInt(right_id) error = %v", err)
		}
		if _, err := scan.GetString("left_name"); err != nil {
			t.Fatalf("GetString(left_name) error = %v", err)
		}
		if _, err := scan.GetString("right_name"); err != nil {
			t.Fatalf("GetString(right_name) error = %v", err)
		}
		if _, err := scan.GetVal("right_id"); err != nil {
			t.Fatalf("GetVal(right_id) error = %v", err)
		}
		got = append(got, [2]int32{leftValue, rightValue})
	}
	want := [][2]int32{{1, 10}, {1, 20}, {2, 10}, {2, 20}}
	if len(got) != len(want) {
		t.Fatalf("product rows = %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Errorf("product row %d = %v, want %v", index, got[index], want[index])
		}
	}
	if right.scan == nil || right.scan.closeCalls != 1 {
		t.Errorf("right source close calls = %#v, want 1", right.scan)
	}
	if err := scan.BeforeFirst(); err != nil {
		t.Fatalf("BeforeFirst() error = %v", err)
	}
	if found, err := scan.Next(); err != nil || !found {
		t.Fatalf("Next() after rewind = (%t, %v), want (true, nil)", found, err)
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if left.scan == nil || left.scan.closeCalls != 1 {
		t.Errorf("left scan close calls = %#v, want 1", left.scan)
	}
	if transaction.pins != transaction.unpins {
		t.Errorf("pins/unpins = %d/%d, want balanced ownership", transaction.pins, transaction.unpins)
	}
}

func TestMultibufferProductPlanOpenClosesLeftWhenRightMaterializationFails(t *testing.T) {
	schema := &record.Schema{}
	schema.AddIntegerField("id")
	left := &materializePlan{schema: schema}
	cause := errors.New("right plan open failed")
	right := &materializePlan{schema: schema, openErr: cause}

	_, err := NewMultibufferProductPlan(newMaterializePlanTransaction(), left, right).Open()
	if !errors.Is(err, cause) {
		t.Fatalf("Open() error = %v, want cause %v", err, cause)
	}
	if left.scan == nil || left.scan.closeCalls != 1 {
		t.Errorf("left scan close calls = %#v, want 1", left.scan)
	}
}

func TestMultibufferProductPlanOpenPreservesMaterializationAndCleanupErrors(t *testing.T) {
	schema := &record.Schema{}
	schema.AddIntegerField("id")
	primaryErr := errors.New("copy source failed")
	cleanupErr := errors.New("left cleanup failed")
	tests := []struct {
		name      string
		configure func(*materializePlanTransaction, *materializePlan, *materializePlan)
		causes    []error
	}{
		{
			name: "left plan open",
			configure: func(_ *materializePlanTransaction, left, _ *materializePlan) {
				left.openErr = primaryErr
			},
			causes: []error{primaryErr},
		},
		{
			name: "left plan nil scan",
			configure: func(_ *materializePlanTransaction, left, _ *materializePlan) {
				left.nilScan = true
			},
		},
		{
			name: "left partial open and cleanup",
			configure: func(_ *materializePlanTransaction, left, _ *materializePlan) {
				left.openErr, left.openScanOnError, left.closeErr = primaryErr, true, cleanupErr
			},
			causes: []error{primaryErr, cleanupErr},
		},
		{
			name: "right source next",
			configure: func(_ *materializePlanTransaction, _, right *materializePlan) {
				right.rows, right.nextErr, right.closeErr = [][]query.Constant{{query.NewIntConstant(1)}}, primaryErr, cleanupErr
			},
			causes: []error{primaryErr, cleanupErr},
		},
		{
			name: "right plan nil scan",
			configure: func(_ *materializePlanTransaction, _, right *materializePlan) {
				right.nilScan = true
			},
		},
		{
			name: "right source value",
			configure: func(_ *materializePlanTransaction, _, right *materializePlan) {
				right.rows, right.getValErr = [][]query.Constant{{query.NewIntConstant(1)}}, primaryErr
			},
			causes: []error{primaryErr},
		},
		{
			name: "right source close",
			configure: func(_ *materializePlanTransaction, _, right *materializePlan) {
				right.closeErr = primaryErr
			},
			causes: []error{primaryErr},
		},
		{
			name: "destination create",
			configure: func(tx *materializePlanTransaction, _, _ *materializePlan) {
				tx.sizeErr = primaryErr
			},
			causes: []error{primaryErr},
		},
		{
			name: "destination record page",
			configure: func(tx *materializePlanTransaction, _, _ *materializePlan) {
				tx.blockSizeErr = primaryErr
			},
			causes: []error{primaryErr},
		},
		{
			name: "destination row insert",
			configure: func(tx *materializePlanTransaction, _, right *materializePlan) {
				right.rows = [][]query.Constant{{query.NewIntConstant(1)}}
				tx.setIntFailAt = (tx.blockSize/record.NewLayout(schema).SlotSize())*2 + 1
				tx.setIntFailErr = primaryErr
			},
			causes: []error{primaryErr},
		},
		{
			name: "destination field write",
			configure: func(tx *materializePlanTransaction, _, right *materializePlan) {
				right.rows = [][]query.Constant{{query.NewIntConstant(1)}}
				tx.setIntFailAt = (tx.blockSize/record.NewLayout(schema).SlotSize())*2 + 2
				tx.setIntFailErr = primaryErr
			},
			causes: []error{primaryErr},
		},
		{
			name: "destination close",
			configure: func(tx *materializePlanTransaction, _, _ *materializePlan) {
				tx.unpinErr = primaryErr
			},
			causes: []error{primaryErr},
		},
		{
			name: "negative product buffers",
			configure: func(tx *materializePlanTransaction, _, _ *materializePlan) {
				tx.availableBuffers = -1
			},
			causes: []error{},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			transaction := newMaterializePlanTransaction()
			left := &materializePlan{schema: schema}
			right := &materializePlan{schema: schema}
			test.configure(transaction, left, right)
			scan, err := NewMultibufferProductPlan(transaction, left, right).Open()
			if err == nil {
				t.Fatal("Open() succeeded, want error")
			}
			for _, cause := range test.causes {
				if !errors.Is(err, cause) {
					t.Errorf("Open() error %v does not preserve %v", err, cause)
				}
			}
			if scan != nil {
				_ = scan.Close()
			}
		})
	}
}

func TestMultibufferProductPlanCopyRecordsFromRejectsMissingSchemaAndTransaction(t *testing.T) {
	schema := &record.Schema{}
	schema.AddIntegerField("id")
	plan := NewMultibufferProductPlan(newMaterializePlanTransaction(), nil, nil)
	if _, err := plan.copyRecordsFrom(&materializePlan{}); err == nil {
		t.Error("copyRecordsFrom() with nil schema succeeded")
	}
	plan.transaction = nil
	if _, err := plan.copyRecordsFrom(&materializePlan{schema: schema}); err == nil {
		t.Error("copyRecordsFrom() with nil transaction succeeded")
	}
}

func TestMultibufferProductPlanOpenRetainsPartialChunkCleanupForRetry(t *testing.T) {
	schema := &record.Schema{}
	schema.AddIntegerField("id")
	primaryErr := errors.New("left rewind failed")
	cleanupErr := errors.New("right chunk release failed")
	transaction := newMaterializePlanTransaction()
	transaction.unpinErr, transaction.unpinErrAt, transaction.unpinErrUntil = cleanupErr, 2, 3
	left := &materializePlan{schema: schema, beforeFirstErr: primaryErr, closeErr: cleanupErr}
	right := &materializePlan{schema: schema}

	scan, err := NewMultibufferProductPlan(transaction, left, right).Open()
	if !errors.Is(err, primaryErr) || !errors.Is(err, cleanupErr) {
		t.Fatalf("Open() error = %v, want both %v and %v", err, primaryErr, cleanupErr)
	}
	if scan == nil {
		t.Fatal("Open() scan = nil, want retryable partial scan")
	}
	transaction.unpinErr = nil
	left.scan.closeErr = nil
	if err := scan.Close(); err != nil {
		t.Fatalf("retry Close() error = %v", err)
	}
}

func TestOwnedMultibufferProductScanCloseRetriesFailedChunkRelease(t *testing.T) {
	schema := &record.Schema{}
	schema.AddIntegerField("id")
	left := &materializePlan{schema: schema}
	right := &materializePlan{schema: schema}
	transaction := newMaterializePlanTransaction()
	scan, err := NewMultibufferProductPlan(transaction, left, right).Open()
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	cause := errors.New("chunk unpin failed")
	transaction.unpinErr = cause
	if err := scan.Close(); !errors.Is(err, cause) {
		t.Fatalf("Close() error = %v, want cause %v", err, cause)
	}
	if left.scan == nil || left.scan.closeCalls != 1 {
		t.Errorf("left scan close calls = %#v, want 1 despite chunk release error", left.scan)
	}
	transaction.unpinErr = nil
	if err := scan.Close(); err != nil {
		t.Fatalf("retry Close() error = %v", err)
	}
}

func TestOwnedMultibufferProductScanRejectsUnavailableProduct(t *testing.T) {
	var scan *ownedMultibufferProductScan
	if err := scan.BeforeFirst(); err == nil {
		t.Error("nil BeforeFirst() succeeded")
	}
	if found, err := scan.Next(); err == nil || found {
		t.Errorf("nil Next() = (%t, %v), want unavailable error", found, err)
	}
	if value, err := scan.GetInt("id"); err == nil || value != 0 {
		t.Errorf("nil GetInt() = (%d, %v), want unavailable error", value, err)
	}
	if value, err := scan.GetString("name"); err == nil || value != "" {
		t.Errorf("nil GetString() = (%q, %v), want unavailable error", value, err)
	}
	if value, err := scan.GetVal("id"); err == nil || value != (query.Constant{}) {
		t.Errorf("nil GetVal() = (%v, %v), want unavailable error", value, err)
	}
	if scan.HasField("id") {
		t.Error("nil HasField() = true, want false")
	}
	if err := scan.Close(); err != nil {
		t.Errorf("nil Close() error = %v, want nil", err)
	}
	if err := (&ownedMultibufferProductScan{}).BeforeFirst(); err == nil {
		t.Error("scan without product BeforeFirst() succeeded")
	}
	if found, err := (&ownedMultibufferProductScan{}).Next(); err == nil || found {
		t.Errorf("scan without product Next() = (%t, %v), want unavailable error", found, err)
	}
}

func TestMultibufferProductPlanOpenRejectsMissingDependencies(t *testing.T) {
	transaction := newMaterializePlanTransaction()
	schema := &record.Schema{}
	schema.AddIntegerField("id")
	validPlan := &materializePlan{schema: schema}
	for _, test := range []struct {
		name string
		plan *MultibufferProductPlan
	}{
		{name: "nil receiver"},
		{name: "nil transaction", plan: NewMultibufferProductPlan(nil, validPlan, validPlan)},
		{name: "nil left", plan: NewMultibufferProductPlan(transaction, nil, validPlan)},
		{name: "nil right", plan: NewMultibufferProductPlan(transaction, validPlan, nil)},
		{name: "nil left schema", plan: NewMultibufferProductPlan(transaction, &materializePlan{}, validPlan)},
		{name: "nil right schema", plan: NewMultibufferProductPlan(transaction, validPlan, &materializePlan{})},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := test.plan.Open(); err == nil {
				t.Fatal("Open() succeeded, want dependency error")
			}
		})
	}
}
