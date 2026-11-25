package plan

import "testing"

func TestNewOptimizedProductPlanChoosesLowerCostOrientation(t *testing.T) {
	tests := []struct {
		name     string
		left     *productPlanProbe
		right    *productPlanProbe
		reverse  bool
		wantCost int
	}{
		{
			name:     "keep input order when cheaper",
			left:     &productPlanProbe{blocks: 2, records: 2},
			right:    &productPlanProbe{blocks: 5, records: 10},
			wantCost: 12,
		},
		{
			name:     "reverse input order when cheaper",
			left:     &productPlanProbe{blocks: 5, records: 10},
			right:    &productPlanProbe{blocks: 2, records: 2},
			reverse:  true,
			wantCost: 12,
		},
		{
			name:     "reverse input order on equal cost",
			left:     &productPlanProbe{blocks: 2, records: 2},
			right:    &productPlanProbe{blocks: 2, records: 2},
			reverse:  true,
			wantCost: 6,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			product := NewOptimizedProductPlan(test.left, test.right)
			wantLeft, wantRight := test.left, test.right
			if test.reverse {
				wantLeft, wantRight = test.right, test.left
			}
			if product.left != wantLeft || product.right != wantRight {
				t.Errorf("selected order = (%p, %p), want (%p, %p)", product.left, product.right, wantLeft, wantRight)
			}
			if got := product.BlocksAccessed(); got != test.wantCost {
				t.Errorf("BlocksAccessed() = %d, want %d", got, test.wantCost)
			}
			if test.left.opens != 0 || test.right.opens != 0 {
				t.Errorf("plan selection opened children %d/%d times, want no opens", test.left.opens, test.right.opens)
			}
		})
	}
}
