package healthcheck

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	ErrRetry = errors.New("waiting for check to complete")
)

// HealthCheckState will contain any setup needed
// by the cherkers and any information needed to be passed to other checks
type HealthCheckState struct {
	Data map[string]any
}

type Checker struct {
	// description is the short description that's printed to the command line
	// when the check is executed
	Description string

	// hintAnchor, when appended to `HintBaseURL`, provides a URL to more
	// information about the check
	HintAnchor string

	// fatal indicates that all remaining checks should be aborted if this check
	// fails; it should only be used if subsequent checks cannot possibly succeed
	// (default false)
	Fatal bool

	// warning indicates that if this check fails, it should be reported, but it
	// should not impact the overall outcome of the health check (default false)
	Warning bool

	// retryDeadline establishes a deadline before which this check should be
	// retried; if the deadline has passed, the check fails (default: no retries)
	RetryDeadline time.Time

	// surfaceErrorOnRetry indicates that the error message should be displayed
	// even if the check will be retried.  This is useful if the error message
	// contains the current status of the check.
	SurfaceErrorOnRetry bool

	// check is the function that's called to execute the check; if the function
	// returns an error, the check fails
	Check func(context.Context, *HealthCheckState) error
}

// CheckResult encapsulates a check's identifying information and output
type CheckResult struct {
	Category    CategoryID
	Description string
	HintURL     string
	Retry       bool
	Warning     bool
	Err         error
}

// CheckObserver receives the results of each check.
type CheckObserver func(*CheckResult)

// Runner is implemented by any health-checkers that can be triggered with RunChecks()
type Runner interface {
	RunChecks(observer CheckObserver) (bool, bool)
}

type HealthCheckConfig struct {
}

type HealthChecker struct {
	Categories []*Category
	Config     HealthCheckConfig
	state      *HealthCheckState
}

var (
	DefaultRetryWindow = 5 * time.Second
	DefaultTimeOut     = 30 * time.Second
)

// NewHealthCheck
func NewHealthChecker(categories []*Category, config *HealthCheckConfig) *HealthChecker {
	if config == nil {
		return nil
	}

	return &HealthChecker{
		Categories: categories,
		Config:     *config,
		state: &HealthCheckState{
			Data: make(map[string]any),
		},
	}
}

// AppendCategories returns a HealthChecker instance appending the provided Categories
func (hc *HealthChecker) AppendCategories(categories ...*Category) *HealthChecker {
	hc.Categories = append(hc.Categories, categories...)
	return hc
}

// GetCategories returns all the categories
func (hc *HealthChecker) GetCategories() []*Category {
	return hc.Categories
}

func (hc *HealthChecker) Run() Reporter {
	reporter := NewSimpleReporter()
	reporter.success, reporter.warning = hc.RunChecks(reporter.Observer)
	return reporter
}

// RunChecks runs all configured checkers, and passes the results of each
// check to the observer. If a check fails and is marked as fatal, then all
// remaining checks are skipped. If at least one check fails, RunChecks returns
// false; if all checks passed, RunChecks returns true.  Checks which are
// designated as warnings will not cause RunCheck to return false, however.
func (hc *HealthChecker) RunChecks(observer CheckObserver) (bool, bool) {
	success := true
	warning := false
	for _, c := range hc.Categories {
		if c.Enabled {
			for _, checker := range c.Checkers {
				if checker.Check != nil {
					if !hc.runCheck(c, checker, observer) {
						if !checker.Warning {
							success = false
						} else {
							warning = true
						}
						if checker.Fatal {
							return success, warning
						}
					}
				}
			}
		}
	}
	return success, warning
}

func (hc *HealthChecker) runCheck(category *Category, c *Checker, observer CheckObserver) bool {
	for {
		ctx, cancel := context.WithTimeout(category.Context, DefaultTimeOut)
		err := c.Check(ctx, hc.state)
		cancel()
		var se SkipError
		if errors.As(err, &se) {
			//log.Debugf("Skipping check: %s. Reason: %s", c.description, se.Reason)
			return true
		}

		checkResult := &CheckResult{
			Category:    category.ID,
			Description: c.Description,
			Warning:     c.Warning,
			HintURL:     fmt.Sprintf("%s%s", category.HintBaseURL, c.HintAnchor),
		}
		var vs VerboseSuccess
		if errors.As(err, &vs) {
			checkResult.Description = fmt.Sprintf("%s\n%s", checkResult.Description, vs.Message)
		} else if err != nil {
			checkResult.Err = CategoryError{category.ID, err}
		}

		if checkResult.Err != nil && time.Now().Before(c.RetryDeadline) {
			checkResult.Retry = true

			// Check if the error provided by the check should be provided
			// to the observer. If not, override it with a generic waiting message
			if !c.SurfaceErrorOnRetry {
				checkResult.Err = ErrRetry
			}

			observer(checkResult)
			time.Sleep(DefaultRetryWindow)
			continue
		}

		observer(checkResult)
		return checkResult.Err == nil
	}
}