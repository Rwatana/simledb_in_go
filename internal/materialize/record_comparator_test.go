package materialize

import (
	"errors"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
)

type comparatorValueScan struct {
	values map[string]query.Constant
	errors map[string]error
	reads  []string
}

func (scan *comparatorValueScan) GetVal(fieldName string) (query.Constant, error) {
	scan.reads = append(scan.reads, fieldName)
	if err := scan.errors[fieldName]; err != nil {
		return query.Constant{}, err
	}
	return scan.values[fieldName], nil
}

func TestRecordComparatorOrdersIntegerAndStringValues(t *testing.T) {
	tests := []struct {
		name  string
		left  query.Constant
		right query.Constant
		want  int
	}{
		{name: "integer less", left: query.NewIntConstant(-3), right: query.NewIntConstant(2), want: -1},
		{name: "integer greater", left: query.NewIntConstant(9), right: query.NewIntConstant(2), want: 1},
		{name: "string less", left: query.NewStringConstant("alpha"), right: query.NewStringConstant("beta"), want: -1},
		{name: "string greater", left: query.NewStringConstant("zeta"), right: query.NewStringConstant("beta"), want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			left := &comparatorValueScan{values: map[string]query.Constant{"key": tt.left}}
			right := &comparatorValueScan{values: map[string]query.Constant{"key": tt.right}}
			got, err := NewRecordComparator([]string{"key"}).Compare(left, right)
			if err != nil || got != tt.want {
				t.Fatalf("Compare() = (%d, %v), want (%d, nil)", got, err, tt.want)
			}
		})
	}
}

func TestRecordComparatorUsesCopiedFieldOrderAndStopsAtFirstDifference(t *testing.T) {
	fields := []string{"group", "score", "unused"}
	comparator := NewRecordComparator(fields)
	fields[0] = "score"
	left := &comparatorValueScan{values: map[string]query.Constant{
		"group": query.NewIntConstant(1), "score": query.NewIntConstant(9),
	}}
	right := &comparatorValueScan{values: map[string]query.Constant{
		"group": query.NewIntConstant(2), "score": query.NewIntConstant(1),
	}}

	got, err := comparator.Compare(left, right)
	if err != nil || got != -1 {
		t.Fatalf("Compare() = (%d, %v), want (-1, nil) using the copied group-first order", got, err)
	}
	if strings.Join(left.reads, ",") != "group" || strings.Join(right.reads, ",") != "group" {
		t.Errorf("reads = left %v, right %v, want only the first differing field", left.reads, right.reads)
	}
}

func TestRecordComparatorReturnsEqualForEqualOrEmptyKeys(t *testing.T) {
	left := &comparatorValueScan{values: map[string]query.Constant{"group": query.NewStringConstant("same"), "id": query.NewIntConstant(4)}}
	right := &comparatorValueScan{values: map[string]query.Constant{"group": query.NewStringConstant("same"), "id": query.NewIntConstant(4)}}
	if got, err := NewRecordComparator([]string{"group", "id"}).Compare(left, right); err != nil || got != 0 {
		t.Fatalf("Compare(equal) = (%d, %v), want (0, nil)", got, err)
	}
	if got, err := NewRecordComparator(nil).Compare(nil, nil); err != nil || got != 0 {
		t.Fatalf("Compare(empty fields) = (%d, %v), want (0, nil)", got, err)
	}
}

func TestRecordComparatorRejectsNilState(t *testing.T) {
	scan := &comparatorValueScan{values: map[string]query.Constant{"key": query.NewIntConstant(1)}}
	if got, err := (*RecordComparator)(nil).Compare(scan, scan); got != 0 || err == nil || !strings.Contains(err.Error(), "comparator") {
		t.Fatalf("nil comparator Compare() = (%d, %v), want contextual error", got, err)
	}
	comparator := NewRecordComparator([]string{"key"})
	if got, err := comparator.Compare(nil, scan); got != 0 || err == nil || !strings.Contains(err.Error(), "left") {
		t.Fatalf("nil left scan Compare() = (%d, %v), want contextual error", got, err)
	}
	if got, err := comparator.Compare(scan, nil); got != 0 || err == nil || !strings.Contains(err.Error(), "right") {
		t.Fatalf("nil right scan Compare() = (%d, %v), want contextual error", got, err)
	}
}

func TestRecordComparatorWrapsScanReadFailures(t *testing.T) {
	readErr := errors.New("value read failed")
	for _, side := range []string{"left", "right"} {
		t.Run(side, func(t *testing.T) {
			left := &comparatorValueScan{values: map[string]query.Constant{"key": query.NewIntConstant(1)}}
			right := &comparatorValueScan{values: map[string]query.Constant{"key": query.NewIntConstant(2)}}
			if side == "left" {
				left.errors = map[string]error{"key": readErr}
			} else {
				right.errors = map[string]error{"key": readErr}
			}
			if got, err := NewRecordComparator([]string{"key"}).Compare(left, right); got != 0 || !errors.Is(err, readErr) || !strings.Contains(err.Error(), side) || !strings.Contains(err.Error(), "key") {
				t.Fatalf("Compare() = (%d, %v), want contextual %s read failure", got, err, side)
			}
			if side == "left" && len(right.reads) != 0 {
				t.Errorf("right reads = %v after left failure, want no read", right.reads)
			}
		})
	}
}

func TestRecordComparatorRejectsUnsetAndIncompatibleValues(t *testing.T) {
	tests := []struct {
		name  string
		left  query.Constant
		right query.Constant
	}{
		{name: "unset value", left: query.Constant{}, right: query.NewIntConstant(1)},
		{name: "incompatible kinds", left: query.NewIntConstant(1), right: query.NewStringConstant("1")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			left := &comparatorValueScan{values: map[string]query.Constant{"key": tt.left}}
			right := &comparatorValueScan{values: map[string]query.Constant{"key": tt.right}}
			if got, err := NewRecordComparator([]string{"key"}).Compare(left, right); got != 0 || err == nil || !strings.Contains(err.Error(), "key") {
				t.Fatalf("Compare() = (%d, %v), want a field-specific comparability error", got, err)
			}
		})
	}
}
