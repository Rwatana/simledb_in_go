package plan

// NewOptimizedProductPlan returns the cheaper orientation of the two-input product.
func NewOptimizedProductPlan(left, right Plan) *ProductPlan {
	leftFirst := NewProductPlan(left, right)
	rightFirst := NewProductPlan(right, left)
	if leftFirst.BlocksAccessed() < rightFirst.BlocksAccessed() {
		return leftFirst
	}
	return rightFirst
}
