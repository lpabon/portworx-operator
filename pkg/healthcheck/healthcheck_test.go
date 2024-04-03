package healthcheck

import (
	"context"
	"fmt"
	"testing"
	"time"

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

func TestHealthCheckCategories(t *testing.T) {

	// Set up
	checkone := []*Checker{
		&Checker{
			Description: "Checker 1",
			HintAnchor:  "check1",
			Check: func(ctx context.Context, state *HealthCheckState) error {
				return nil
			},
		},
	}

	checktwo := []*Checker{
		&Checker{
			Description: "Checker 2",
			HintAnchor:  "check2",
			Check: func(ctx context.Context, state *HealthCheckState) error {
				return nil
			},
		},
	}

	cat1 := NewCategory("cat1", checkone, true, "http://test.com/")
	assert.NotNil(t, cat1)
	cat2 := NewCategory("cat2", checktwo, true, "http://test.com/")
	assert.NotNil(t, cat2)

	hc := NewHealthChecker([]*Category{cat1}, &HealthCheckConfig{})
	assert.NotNil(t, hc)
	hc.AppendCategories(cat2)

	cats := hc.GetCategories()
	assert.Len(t, cats, 2)

	tr := newTestResults(t)
	hc.RunChecks(tr.result)

	assert.Len(t, tr.results, 2)
	result := tr.results[0]
	assert.Equal(t, result.Category, CategoryID("cat1"))
	assert.Equal(t, result.Description, "Checker 1")
	assert.Equal(t, result.HintURL, "http://test.com/check1")
	assert.False(t, result.Retry)
	assert.False(t, result.Warning)
	assert.NoError(t, result.Err)

	result = tr.results[1]
	assert.Equal(t, result.Category, CategoryID("cat2"))
	assert.Equal(t, result.Description, "Checker 2")
	assert.Equal(t, result.HintURL, "http://test.com/check2")
	assert.False(t, result.Retry)
	assert.False(t, result.Warning)
	assert.NoError(t, result.Err)
}

func TestSingleChecker(t *testing.T) {

	// Set up
	called := false
	checkers := []*Checker{
		&Checker{
			Description: "Checker 1",
			HintAnchor:  "check123",
			Check: func(ctx context.Context, state *HealthCheckState) error {
				called = true
				return nil
			},
		},
	}

	cat := NewCategory("test", checkers, true, "http://test.com/")
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
			Check: func(ctx context.Context, state *HealthCheckState) error {
				state.Data["one"] = "two"
				return nil
			},
		},
		&Checker{
			Description: "Checker 234",
			HintAnchor:  "check234",
			Check: func(ctx context.Context, state *HealthCheckState) error {
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

	cat := NewCategory("test", checkers, true, "http://test.com/")
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

func TestHealthCheckerWarning(t *testing.T) {

	called := false
	// Set up
	checkers := []*Checker{
		&Checker{
			Description: "Checker 123",
			HintAnchor:  "check123",
			Check: func(ctx context.Context, state *HealthCheckState) error {
				return fmt.Errorf("ERROR")
			},
			Warning: true,
		},
		&Checker{
			Description: "Checker 234",
			HintAnchor:  "check234",
			Check: func(ctx context.Context, state *HealthCheckState) error {
				called = true
				return nil
			},
		},
	}

	cat := NewCategory("test", checkers, true, "http://test.com/")
	assert.NotNil(t, cat)
	hc := NewHealthChecker([]*Category{cat}, &HealthCheckConfig{})
	assert.NotNil(t, hc)

	// Test
	tr := newTestResults(t)
	hc.RunChecks(tr.result)

	assert.Len(t, tr.results, 2)
	assert.True(t, called)
}

func TestHealthCheckerFatal(t *testing.T) {

	called := false
	// Set up
	checkers := []*Checker{
		&Checker{
			Description: "Checker 123",
			HintAnchor:  "check123",
			Check: func(ctx context.Context, state *HealthCheckState) error {
				return fmt.Errorf("ERROR")
			},
			Fatal: true,
		},
		&Checker{
			Description: "Checker 234",
			HintAnchor:  "check234",
			Check: func(ctx context.Context, state *HealthCheckState) error {
				called = true
				return nil
			},
		},
	}

	cat := NewCategory("test", checkers, true, "http://test.com/")
	assert.NotNil(t, cat)
	hc := NewHealthChecker([]*Category{cat}, &HealthCheckConfig{})
	assert.NotNil(t, hc)

	// Test
	tr := newTestResults(t)
	hc.RunChecks(tr.result)

	assert.Len(t, tr.results, 1)
	assert.False(t, called)
}

func TestHealthCheckerRetry(t *testing.T) {

	// Set the retry window to a quick value for the unit tests
	saveDefaultRetryWindow := DefaultRetryWindow
	DefaultRetryWindow = time.Millisecond
	defer func() {
		DefaultRetryWindow = saveDefaultRetryWindow
	}()

	retryDeadline := time.Now().Add(time.Millisecond * 100)
	counter := 0

	// Set up
	checkers := []*Checker{
		&Checker{
			Description: "Checker 123",
			HintAnchor:  "check123",
			Check: func(ctx context.Context, state *HealthCheckState) error {
				counter++
				return fmt.Errorf("ERROR")
			},
			RetryDeadline: retryDeadline,
		},
		&Checker{
			Description: "Checker 234",
			HintAnchor:  "check234",
			Check: func(ctx context.Context, state *HealthCheckState) error {
				return nil
			},
		},
	}

	cat := NewCategory("test", checkers, true, "http://test.com/")
	assert.NotNil(t, cat)
	hc := NewHealthChecker([]*Category{cat}, &HealthCheckConfig{})
	assert.NotNil(t, hc)

	// Test
	tr := newTestResults(t)
	hc.RunChecks(tr.result)

	assert.Len(t, tr.results, counter+1)

	// picked 5 out of the air. Really all we want is a few times to retry
	// but it all depends on the scheduler. So on a non-busy system, it should
	// be rescheduled RetryDeadline / DefaultRetryWindo times
	assert.GreaterOrEqual(t, counter, 5)

	// Confirm that the last retry has failed
	assert.Error(t, tr.results[counter-1].Err)

	// Confirm that the last check worked
	assert.NoError(t, tr.results[counter].Err)
}

func TestHealthCheckerSkip(t *testing.T) {

	// Set up
	checkers := []*Checker{
		&Checker{
			Description: "Checker 123",
			HintAnchor:  "check123",
			Check: func(ctx context.Context, state *HealthCheckState) error {
				return SkipError{
					Reason: "Skip Test",
				}
			},

			// Make it fatal, but it will not stop since it should be skipped
			Fatal: true,
		},
		&Checker{
			Description: "Checker 234",
			HintAnchor:  "check234",
			Check: func(ctx context.Context, state *HealthCheckState) error {
				return nil
			},
		},
	}

	cat := NewCategory("test", checkers, true, "http://test.com/")
	assert.NotNil(t, cat)
	hc := NewHealthChecker([]*Category{cat}, &HealthCheckConfig{})
	assert.NotNil(t, hc)

	// Test
	tr := newTestResults(t)
	hc.RunChecks(tr.result)

	// Only has 1 becase it skipped the first one.
	assert.Len(t, tr.results, 1)

	// Assert that the only result is the non-skipped check
	result := tr.results[0]
	assert.Equal(t, result.Category, CategoryID("test"))
	assert.Equal(t, result.Description, "Checker 234")
	assert.Equal(t, result.HintURL, "http://test.com/check234")
}

func TestHealthCheckerVerboseSuccess(t *testing.T) {

	// Set up
	checkers := []*Checker{
		&Checker{
			Description: "Checker 123",
			HintAnchor:  "check123",
			Check: func(ctx context.Context, state *HealthCheckState) error {
				return VerboseSuccess{
					Message: "Hello",
				}
			},
		},
		&Checker{
			Description: "Checker 234",
			HintAnchor:  "check234",
			Check: func(ctx context.Context, state *HealthCheckState) error {
				return fmt.Errorf("ERROR")
			},
		},
	}

	cat := NewCategory("test", checkers, true, "http://test.com/")
	assert.NotNil(t, cat)
	hc := NewHealthChecker([]*Category{cat}, &HealthCheckConfig{})
	assert.NotNil(t, hc)

	// Test
	tr := newTestResults(t)
	hc.RunChecks(tr.result)

	assert.Len(t, tr.results, 2)

	result := tr.results[0]
	assert.Equal(t, result.Category, CategoryID("test"))
	assert.Equal(t, result.Description, "Checker 123\nHello")
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
	assert.Error(t, result.Err)
	assert.ErrorAs(t, result.Err, &CategoryError{})

}
