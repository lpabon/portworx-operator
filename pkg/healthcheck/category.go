package healthcheck

import "context"

type CategoryID string

// Category contains a suite of health checks
type Category struct {
	ID          CategoryID
	Enabled     bool
	Checkers    []*Checker
	HintBaseURL string

	// Must be set
	Context context.Context
}

// NewCategory returns a category with a default background context
func NewCategory(id CategoryID, checkers []*Checker, enabled bool, hintBaseURL string) *Category {
	return newCategory(id, checkers, enabled, hintBaseURL, context.Background())
}

// NewCategoryWithContext creates a new category of health checks
func NewCategoryWithContext(
	id CategoryID,
	checkers []*Checker,
	enabled bool,
	hintBaseURL string,
	ctx context.Context,
) *Category {
	return newCategory(id, checkers, enabled, hintBaseURL, ctx)
}

func newCategory(
	id CategoryID,
	checkers []*Checker,
	enabled bool,
	hintBaseURL string,
	ctx context.Context,
) *Category {
	return &Category{
		ID:          id,
		Checkers:    checkers,
		Enabled:     enabled,
		HintBaseURL: hintBaseURL,
		Context:     ctx,
	}
}

func (c *Category) WithContext(ctx context.Context) *Category {
	if c != nil {
		c.Context = ctx
	}
	return c
}
