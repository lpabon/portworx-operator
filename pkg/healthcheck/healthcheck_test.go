package healthcheck

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testResults struct {
	results  []*CheckResult
	resultFn CheckObserver
	t        *testing.T
}

func newTestResultsWithObserver(t *testing.T, r CheckObserver) *testResults {
	return &testResults{
		results:  make([]*CheckResult, 0),
		resultFn: r,
		t:        t,
	}
}

func newTestResults(t *testing.T) *testResults {
	return newTestResultsWithObserver(t, nil)
}

func (tr *testResults) result(r *CheckResult) {
	tr.results = append(tr.results, r)
	if tr.resultFn != nil {
		tr.resultFn(r)
	}
}

func TestNewHealthChecker(t *testing.T) {
	hc := NewHealthChecker(nil, nil)
	assert.Nil(t, hc)

	hc = NewHealthChecker(nil, &HealthCheckConfig{})
	assert.NotNil(t, hc)
}

func TestSingleChecker(t *testing.T) {

	// Set up
	called := false
	checkers := []*Checker{
		&Checker{
			Description: "Checker 1",
			HintAnchor:  "check123",
			Check: func(ctx context.Context, state HealthCheckState) error {
				called = true
				return nil
			},
		},
	}

	cat := NewCategoryDefaultCategory("test", checkers, true, "http://test.com/")
	assert.NotNil(t, cat)
	hc := NewHealthChecker([]*Category{cat}, &HealthCheckConfig{})
	assert.NotNil(t, hc)

	observer_called := false
	observer := func(r *CheckResult) {
		observer_called = true
	}

	// Test
	assert.False(t, called)
	assert.False(t, observer_called)

	tr := newTestResultsWithObserver(t, observer)
	hc.RunChecks(tr.result)

	assert.Len(t, tr.results, 1)
	result := tr.results[0]
	assert.True(t, called)
	assert.True(t, observer_called)
	assert.Equal(t, result.Category, CategoryID("test"))
	assert.Equal(t, result.Description, "Checker 1")
	assert.Equal(t, result.HintURL, "http://test.com/check123")
	assert.False(t, result.Retry)
	assert.False(t, result.Warning)
	assert.NoError(t, result.Err)
}

func TestPassingDataFromCheckToCheck(t *testing.T) {

	// Set up
	checkers := []*Checker{
		&Checker{
			Description: "Checker 123",
			HintAnchor:  "check123",
			Check: func(ctx context.Context, state HealthCheckState) error {
				state.Data["one"] = "two"
				return nil
			},
		},
		&Checker{
			Description: "Checker 234",
			HintAnchor:  "check234",
			Check: func(ctx context.Context, state HealthCheckState) error {
				if val, ok := state.Data["one"].(string); !ok {
					return fmt.Errorf("Not found or not a string")
				} else {
					if val != "two" {
						return fmt.Errorf("Not equal")
					}
				}
				return nil
			},
		},
	}

	cat := NewCategoryDefaultCategory("test", checkers, true, "http://test.com/")
	assert.NotNil(t, cat)
	hc := NewHealthChecker([]*Category{cat}, &HealthCheckConfig{})
	assert.NotNil(t, hc)

	// Test
	tr := newTestResults(t)
	hc.RunChecks(tr.result)

	assert.Len(t, tr.results, 2)

	result := tr.results[0]
	assert.Equal(t, result.Category, CategoryID("test"))
	assert.Equal(t, result.Description, "Checker 123")
	assert.Equal(t, result.HintURL, "http://test.com/check123")
	assert.False(t, result.Retry)
	assert.False(t, result.Warning)
	assert.NoError(t, result.Err)

	result = tr.results[1]
	assert.Equal(t, result.Category, CategoryID("test"))
	assert.Equal(t, result.Description, "Checker 234")
	assert.Equal(t, result.HintURL, "http://test.com/check234")
	assert.False(t, result.Retry)
	assert.False(t, result.Warning)
}
