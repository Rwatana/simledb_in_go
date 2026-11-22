package query

import "testing"

func TestEmptyPredicateIsTrueWithoutScanAccess(t *testing.T) {
	var predicate Predicate
	if got, err := predicate.IsSatisfied(nil); err != nil || !got {
		t.Fatalf("IsSatisfied(nil) = (%v, %v), want (true, nil)", got, err)
	}

	scan := &valueScanStub{}
	if got, err := predicate.IsSatisfied(scan); err != nil || !got {
		t.Fatalf("IsSatisfied(scan) = (%v, %v), want (true, nil)", got, err)
	}
	if scan.calls != 0 {
		t.Fatalf("GetVal calls = %d, want 0", scan.calls)
	}
}
