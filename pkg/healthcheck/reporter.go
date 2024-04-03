package healthcheck

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"

	"github.com/fatih/color"
)

const (
	// JSONOutput is used to specify the json output format
	JSONOutput = "json"
	// TableOutput is used to specify the table output format
	TableOutput = "table"
	// WideOutput is used to specify the wide output format
	WideOutput = "wide"
	// ShortOutput is used to specify the short output format
	ShortOutput = "short"
)

var (
	okStatus   = color.New(color.FgGreen, color.Bold).SprintFunc()("\u221A")  // √
	warnStatus = color.New(color.FgYellow, color.Bold).SprintFunc()("\u203C") // ‼
	failStatus = color.New(color.FgRed, color.Bold).SprintFunc()("\u00D7")    // ×

	reStableVersion = regexp.MustCompile(`stable-(\d\.\d+)\.`)
)

type Reporter interface {
	HasWarning() bool
	Successful() bool
	ToJSON() ([]byte, error)
	Print(w io.Writer)
	GetResults() []*CheckResult
	Replay(observer CheckObserver) (bool, bool)
}

// SimpleReporter contains a slice of CheckResult structs.
type SimpleReporter struct {
	results []*CheckResult
	success bool
	warning bool
}

var _ Reporter = &SimpleReporter{}

// CheckOutput groups the check results for all categories
type CheckOutput struct {
	Success    bool             `json:"success"`
	Warning    bool             `json:"warning"`
	Categories []*CheckCategory `json:"categories"`
}

// CheckCategory groups a series of check for a category
type CheckCategory struct {
	Name   CategoryID `json:"categoryName"`
	Checks []*Check   `json:"checks"`
}

// CheckResultStr is a string describing the result of a check
type CheckResultStr string

const (
	CheckSuccess CheckResultStr = "success"
	CheckWarn    CheckResultStr = "warning"
	CheckErr     CheckResultStr = "error"
)

// Check is a user-facing version of `healthcheck.CheckResult`, for output via
// `linkerd check -o json`.
type Check struct {
	Description string         `json:"description"`
	Hint        string         `json:"hint,omitempty"`
	Error       string         `json:"error,omitempty"`
	Result      CheckResultStr `json:"result"`
}

func NewSimpleReporter() *SimpleReporter {
	return &SimpleReporter{
		results: make([]*CheckResult, 0),
	}
}

func (cr *SimpleReporter) Observer(r *CheckResult) {
	cr.results = append(cr.results, r)
}

func (cr *SimpleReporter) HasWarning() bool {
	return cr.warning
}

func (cr *SimpleReporter) Successful() bool {
	return cr.success
}

func (cr *SimpleReporter) ToJSON() ([]byte, error) {
	var categories []*CheckCategory

	collectJSONOutput := func(result *CheckResult) {
		if categories == nil || categories[len(categories)-1].Name != result.Category {
			categories = append(categories, &CheckCategory{
				Name:   result.Category,
				Checks: []*Check{},
			})
		}

		if !result.Retry {
			currentCategory := categories[len(categories)-1]
			// ignore checks that are going to be retried, we want only final results
			status := CheckSuccess
			if result.Warning {
				status = CheckWarn
			}
			if result.Err != nil {
				status = CheckErr
			}

			currentCheck := &Check{
				Description: result.Description,
				Result:      status,
			}

			if result.Err != nil {
				currentCheck.Error = result.Err.Error()

				if result.HintURL != "" {
					currentCheck.Hint = result.HintURL
				}
			}
			currentCategory.Checks = append(currentCategory.Checks, currentCheck)
		}
	}

	success, warning := cr.Replay(collectJSONOutput)

	outputJSON := CheckOutput{
		Success:    success,
		Warning:    warning,
		Categories: categories,
	}

	return json.Marshal(outputJSON)
}

func (cr *SimpleReporter) Print(w io.Writer) {

	errorFound := false
	printer := func(result *CheckResult) {
		status := okStatus
		if result.Warning {
			status = warnStatus
		}
		if result.Err != nil {
			errorFound = true
			status = failStatus
		}

		fmt.Fprintf(w, "[%s] %s/%s\n",
			status,
			result.Category,
			result.Description)

		if result.Err != nil {
			color.New(color.FgRed).Fprintf(w, "\tErr: %s\n", result.Err)
		}
		if result.HintURL != "" {
			if result.Err != nil || result.Warning {
				fmt.Fprintf(w, "\tSee: %s\n", result.HintURL)
			}
		}
	}

	success, warning := cr.Replay(printer)

	if success {
		color.New(color.FgGreen, color.Bold).Fprintf(w, "Ok")
	} else if warning {
		color.New(color.FgYellow, color.Bold).Fprintf(w, "Warning")
	} else if errorFound {
		color.New(color.FgRed, color.Bold).Fprintf(w, "Error")
	}
}

func (cr *SimpleReporter) GetResults() []*CheckResult {
	return cr.results
}

func (cr *SimpleReporter) Replay(observer CheckObserver) (bool, bool) {
	success := true
	warning := false
	for _, result := range cr.results {
		if result.Err != nil {
			if !result.Warning {
				success = false
			} else {
				warning = true
			}
		}
		observer(result)
	}
	return success, warning
}
