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

package gomonitor

import (
	"fmt"
	"os"
)

// ExitCode represents a Nagios exit code
type ExitCode int

// OK indicates that everything is fine
// Warning indicates that there is a potential issue, but it's not critical
// Critical indicates that there is a serious issue that requires immediate attention
// Unknown indicates that the plugin was unable to determine the status of the check
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
// - `ExitCode` is the exit code of the check, indicating the status of the check.
// - `Message` is a descriptive message associated with the check result.
// - `PerformanceData` is a map containing performance metrics associated with the check result.
// - `Format` is the format string used to generate the output message.
type CheckResult struct {
	ExitCode
	Message         string
	PerformanceData map[string]PerformanceMetric
	Format          string
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
	delete(cr.PerformanceData, metricName)
}

// SendResult will output the formatted message and exit with the appropriate exit code
func (cr *CheckResult) SendResult() {
	output := fmt.Sprintf(cr.Format, cr.ExitCode.String(), cr.Message)
	// Check if there is performance data to return
	if len(cr.PerformanceData) > 0 {
		performanceDataStr := ""
		for key, metric := range cr.PerformanceData {
			metricStr := fmt.Sprintf("'%s'=%v%s;%v;%v;%v;%v ",
				key, metric.Value, metric.UnitOM, metric.Warn, metric.Crit, metric.Min, metric.Max)
			performanceDataStr += metricStr
		}

		// Append performance data to the message
		output = fmt.Sprintf("%s | %s", output, performanceDataStr)
	}
	fmt.Println(output)
	os.Exit(cr.ExitCode.Int())
}

// NewCheckResult initializes a new check result
func NewCheckResult() *CheckResult {
	return &CheckResult{
		ExitCode:        OK,
		Format:          "%s - %s",
		PerformanceData: make(map[string]PerformanceMetric),
	}
}
