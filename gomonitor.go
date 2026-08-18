/*
   Copyright 2024 David Mabry

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

// Package gomonitor provides a framework for creating monitoring checks with Nagios-compatible exit codes
// and performance data. It allows you to create check results, add performance metrics, and output the
// results in a standardized format.
package gomonitor

import (
	"fmt"
	"os"
	"strings"
)

// ExitCode represents a Nagios exit code
type ExitCode int

// Status constants represent the possible states of a monitoring check.
const (
	// OK indicates that everything is fine
	OK ExitCode = iota
	// Warning indicates that there is a potential issue, but it's not critical
	Warning
	// Critical indicates that there is a serious issue that requires immediate attention
	Critical
	// Unknown indicates that the plugin was unable to determine the status of the check
	Unknown
)

// String returns a string representation of an ExitCode
func (ec ExitCode) String() string {
	switch ec {
	case OK:
		return "OK"
	case Warning:
		return "Warning"
	case Critical:
		return "Critical"
	case Unknown:
		return "Unknown"
	default:
		return fmt.Sprintf("ExitCode(%d)", ec)
	}
}

// Int returns the integer value associated with the ExitCode. The mapping is as follows:
// - OK: 0
// - Warning: 1
// - Critical: 2
// - Unknown: 3
// - For any other value, the integer value is the underlying value of the ExitCode.
func (ec ExitCode) Int() int {
	switch ec {
	case OK:
		return 0
	case Warning:
		return 1
	case Critical:
		return 2
	case Unknown:
		return 3
	default:
		return int(ec)
	}
}

// PerformanceMetric represents a performance metric with various attributes.
// - `Value` is the actual value of the metric.
// - `Warn` and `Crit` are threshold values for warning and critical states respectively.
// - `Min` and `Max` represent the minimum and maximum expected values of the metric.
// - `UnitOM` is the unit of measure for the metric.
type PerformanceMetric struct {
	Value  float64
	Warn   float64
	Crit   float64
	Min    float64
	Max    float64
	UnitOM string
}

// CheckResult represents the result of a Monitoring check.
//   - `ExitCode` is the exit code of the check, indicating the status of the check.
//   - `Message` is a descriptive message associated with the check result.
//   - `PerformanceData` is a map containing performance metrics associated with the check result.
//   - `Format` is the format string used to generate the output message.
//   - `StatusPrefix` controls whether the exit code (e.g. "OK") is prepended to the output.
//     It defaults to true. Set it to false when the message already carries its own status
//     prefix to avoid doubling (e.g. "OK: CPU usage...").
type CheckResult struct {
	ExitCode
	Message         string
	PerfOrder       []string
	PerformanceData map[string]PerformanceMetric
	Format          string
	StatusPrefix    bool
	// Map to store indices of performance metrics for efficient deletion
	perfIndexMap map[string]int
}

// SetResult sets the ExitCode and Message fields of the CheckResult to the provided values.
func (cr *CheckResult) SetResult(ec ExitCode, msg string) {
	cr.ExitCode = ec
	cr.Message = msg
}

// AddPerformanceData adds a performance metric to the CheckResult's PerformanceData map.
// If the PerformanceData map is nil, it is initialized before adding the metric.
func (cr *CheckResult) AddPerformanceData(metricName string, metric PerformanceMetric) {
	if cr.PerformanceData == nil {
		cr.PerformanceData = make(map[string]PerformanceMetric)
		cr.PerfOrder = []string{}
		cr.perfIndexMap = make(map[string]int)
	}

	if _, exists := cr.PerformanceData[metricName]; !exists {
		cr.PerfOrder = append(cr.PerfOrder, metricName)
		cr.perfIndexMap[metricName] = len(cr.PerfOrder) - 1
	}

	cr.PerformanceData[metricName] = metric
}

// UpdatePerformanceData updates the PerformanceData map of a CheckResult with the provided metric.
// The metric is added to the PerformanceData map using the metricName as the key.
func (cr *CheckResult) UpdatePerformanceData(metricName string, metric PerformanceMetric) {
	cr.PerformanceData[metricName] = metric
}

// DeletePerformanceData deletes the specified metric from the PerformanceData map of the CheckResult.
// If the PerformanceData map does not contain the specified metric, no action is taken.
func (cr *CheckResult) DeletePerformanceData(metricName string) {
	if _, exists := cr.PerformanceData[metricName]; !exists {
		return
	}

	delete(cr.PerformanceData, metricName)

	if index, exists := cr.perfIndexMap[metricName]; exists {
		delete(cr.perfIndexMap, metricName)

		// Remove the element from PerfOrder
		lastElement := cr.PerfOrder[len(cr.PerfOrder)-1]
		cr.PerfOrder[index] = lastElement
		cr.perfIndexMap[lastElement] = index

		// Resize the slice
		cr.PerfOrder = cr.PerfOrder[:len(cr.PerfOrder)-1]
	}
}

// FormatResult formats the check result message with performance data, but does not exit the program.
// This allows for more flexible usage of the library.
//
// The Format template supports the two verbs used by the default format
// ("%s: %s"): %s is replaced with the status string, and the second %s is
// replaced with the message. Any other '%' in the template is preserved
// literally, so a template such as "[%s] %s (95% sure)" is safe without
// escaping the percent as "%%". For backward compatibility, an explicit
// "%%" in the template is still collapsed to a single "%".
func (cr *CheckResult) FormatResult() string {
	var output string
	if cr.StatusPrefix {
		output = formatTemplate(cr.Format, cr.ExitCode.String(), cr.Message)
	} else {
		output = cr.Message
	}

	// Check if there is performance data to return
	if len(cr.PerformanceData) > 0 {
		performanceDataStr := ""
		for _, key := range cr.PerfOrder {
			metric := cr.PerformanceData[key]
			metricStr := fmt.Sprintf("'%s'=%.2f%s;%.2f;%.2f;%.2f;%.2f ",
				key, metric.Value, metric.UnitOM, metric.Warn, metric.Crit, metric.Min, metric.Max)
			performanceDataStr += metricStr
		}

		// Append performance data to the message
		output = fmt.Sprintf("%s | %s", output, performanceDataStr)
	}

	return output
}

// formatTemplate renders the Format template safely. It recognizes only the
// two verbs used by this library — "%s" for status and "%s" for message —
// and passes through every other '%' literally (no Sprintf interpretation,
// so stray percents in a template cannot produce "%!s(MISSING)" garbage).
// For backward compatibility, a "%%" in the template still collapses to a
// single "%" (the previous Sprintf escape hatch).
func formatTemplate(template, status, message string) string {
	parts := strings.Split(template, "%s")

	// Collapse "%%" escapes in the template itself, matching the old
	// Sprintf-based behavior. Status/message arguments are substituted
	// afterwards, so percents in user content pass through untouched.
	for i := range parts {
		parts[i] = strings.ReplaceAll(parts[i], "%%", "%")
	}

	// Templates without any verb are returned as-is.
	if len(parts) == 1 {
		return parts[0]
	}

	var b strings.Builder
	b.WriteString(parts[0])
	b.WriteString(status)
	for i := 1; i < len(parts)-1; i++ {
		b.WriteString(parts[i])
		b.WriteString(message)
	}
	b.WriteString(parts[len(parts)-1])
	return b.String()
}

// SendResult outputs the formatted message and exits with the appropriate exit code.
// This is a convenience method that combines FormatResult with os.Exit.
func (cr *CheckResult) SendResult() {
	output := cr.FormatResult()
	fmt.Println(output)
	os.Exit(cr.ExitCode.Int())
}

// NewCheckResult initializes a new check result with default values.
func NewCheckResult() *CheckResult {
	return &CheckResult{
		ExitCode:        OK,
		Format:          "%s: %s",
		StatusPrefix:    true,
		PerformanceData: make(map[string]PerformanceMetric),
		PerfOrder:       []string{},
		perfIndexMap:    make(map[string]int),
	}
}
