package parse

// CreateViewData holds the name and query definition of a CREATE VIEW statement.
type CreateViewData struct {
	viewName string
	query    QueryData
}

// NewCreateViewData constructs CREATE VIEW data from a view name and its definition.
func NewCreateViewData(viewName string, query QueryData) CreateViewData {
	return CreateViewData{viewName: viewName, query: query}
}

// ViewName returns the name of the view to create.
func (data CreateViewData) ViewName() string {
	return data.viewName
}

// ViewDefinition returns the rendered SELECT definition of the view.
func (data CreateViewData) ViewDefinition() string {
	return data.query.String()
}
