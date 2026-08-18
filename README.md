# gomonitor

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://www.apache.org/licenses/LICENSE-2.0)

A Go library for creating monitoring plugins with Nagios-compatible exit codes and performance data.

gomonitor provides a framework for creating monitoring checks that follow the Nagios plugin development guidelines. It allows you to create check results, add performance metrics, and output the results in a standardized format that can be consumed by monitoring systems like Nagios, Icinga, Zabbix, and others.

## Table of Contents

- [Features](#features)
- [Installation](#installation)
- [Usage](#usage)
  - [Basic Example](#basic-example)
  - [Adding Performance Data](#adding-performance-data)
  - [Command-line Options and Verbose Output](#command-line-options-and-verbose-output)
  - [Complete Example: Load Average Check](#complete-example-load-average-check)
- [API Reference](#api-reference)
  - [ExitCode](#exitcode)
  - [CheckResult](#checkresult)
  - [PerformanceMetric](#performancemetric)
- [Contributing](#contributing)

- [License](#license)

## Features

gomonitor provides a simple and flexible framework for creating monitoring plugins with the following features:

- Standardized Nagios-compatible exit codes (OK, Warning, Critical, Unknown)
- Support for performance data in a format compatible with Nagios and other monitoring systems
- Easy-to-use API for creating check results, adding performance metrics, and outputting results
- Comprehensive test suite to ensure reliability
- Support for command-line options and verbose output levels
- Proper handling of error conditions and unknown states
- Well-documented code with examples

## Installation

To install gomonitor, you can use the `go get` command:

```bash
go get github.com/dmabry/gomonitor
```

Alternatively, you can add the following import to your Go code:

```go
import "github.com/dmabry/gomonitor"
```

### Building from Source

If you want to build gomonitor from source or contribute to its development, follow these steps:

1. Clone the repository:
   ```bash
   git clone https://github.com/dmabry/gomonitor.git
   cd gomonitor
   ```

2. Build the library:
   ```bash
   go build
   ```

3. Run tests to ensure everything is working correctly:
   ```bash
   go test ./...
   ```

## Usage

### Basic Example

Here's a simple example of how to use gomonitor to create a monitoring plugin:

```go
package main

import (
    "github.com/dmabry/gomonitor"
)

func main() {
    // Create a new check result
    result := gomonitor.NewCheckResult()

    // Set the exit code and message
    result.SetResult(gomonitor.OK, "Everything is fine")

    // Output the result and exit with the appropriate exit code
    result.SendResult()
}
```

### Adding Performance Data

You can also add performance data to your check results:

```go
package main

import (
    "github.com/dmabry/gomonitor"
)

func main() {
    // Create a new check result
    result := gomonitor.NewCheckResult()

    // Add performance data
    metric := gomonitor.PerformanceMetric{
        Value:  1.23,
        Warn:   1.00,
        Crit:   2.00,
        Min:    0.00,
        Max:    10.00,
        UnitOM: "ms",
    }
    result.AddPerformanceData("response_time", metric)

    // Set the exit code and message
    result.SetResult(gomonitor.OK, "Everything is fine")

    // Output the result and exit with the appropriate exit code
    result.SendResult()
}
```

### Command-line Options and Verbose Output

To create a more complete Nagios plugin, you'll want to handle command-line options for thresholds and verbose output. Here's an example using Go's `flag` package:

```go
package main

import (
    "flag"
    "github.com/dmabry/gomonitor"
)

func main() {
    var warningThreshold float64
    var criticalThreshold float64
    var verbose int

    flag.Float64Var(&warningThreshold, "w", 5.0, "Warning threshold")
    flag.Float64Var(&criticalThreshold, "c", 10.0, "Critical threshold")
    flag.IntVar(&verbose, "v", 0, "Verbose mode (0-3)")
    flag.Parse()

    // Create a new check result
    result := gomonitor.NewCheckResult()

    // Set the exit code and message based on thresholds
    value := getMetricValue() // Implement this function to get your metric value

    if value > criticalThreshold {
        result.SetResult(gomonitor.Critical, "Value exceeds threshold")
    } else if value > warningThreshold {
        result.SetResult(gomonitor.Warning, "Value above warning threshold")
    } else {
        result.SetResult(gomonitor.OK, "Value within normal range")
    }

    // Add performance data
    metric := gomonitor.PerformanceMetric{
        Value:  value,
        Warn:   warningThreshold,
        Crit:   criticalThreshold,
        Min:    0,
        Max:    100,
        UnitOM: "",
    }
    result.AddPerformanceData("metric_name", metric)

    // Output the result and exit with the appropriate exit code
    result.SendResult()
}
```

### Format and Status Prefix

By default, `FormatResult()` prepends the status (e.g. `OK: `) to the message:

```go
result := gomonitor.NewCheckResult()
result.SetResult(gomonitor.OK, "Everything is fine")
fmt.Println(result.FormatResult()) // "OK: Everything is fine"
```

The `Format` field controls the template used when the status prefix is enabled. It supports two `%s` verbs: the first is replaced with the status string, the second with the message. Any other `%` in the template is preserved literally — no escaping needed (for backward compatibility, an explicit `%%` still collapses to a single `%`):

```go
result.Format = "[%s] %s (95% sure)"
result.SetResult(gomonitor.Warning, "High latency")
fmt.Println(result.FormatResult()) // "[Warning] High latency (95% sure)"
```

If your message already carries its own status prefix (e.g. `"OK: Everything is fine"`), set `StatusPrefix` to `false` to avoid doubling it. Note that performance data is appended to the output automatically, so you should **not** add a `| %s` verb to your `Format` string.

### Complete Example: Load Average Check

Here's a complete example of a Nagios plugin that checks system load average:

```go
package main

import (
    "flag"
    "fmt"
    "os"
    "strconv"
    "strings"

    "github.com/dmabry/gomonitor"
)

func getLoadAverage() (float64, error) {
    loadAvgStr := os.Getenv("LOADAVG")
    if loadAvgStr == "" {
        return 0, fmt.Errorf("LOADAVG environment variable not set")
    }

    loadAvgs := strings.Split(loadAvgStr, " ")
    if len(loadAvgs) < 1 {
        return 0, fmt.Errorf("invalid LOADAVG format")
    }

    loadAvg, err := strconv.ParseFloat(loadAvgs[0], 64)
    if err != nil {
        return 0, fmt.Errorf("could not parse load average: %v", err)
    }

    return loadAvg, nil
}

func main() {
    var warningThreshold float64
    var criticalThreshold float64
    var verbose int

    flag.Float64Var(&warningThreshold, "w", 5.0, "Warning threshold for load average")
    flag.Float64Var(&criticalThreshold, "c", 10.0, "Critical threshold for load average")
    flag.IntVar(&verbose, "v", 0, "Verbose mode (0-3)")
    flag.Parse()

    loadAvg, err := getLoadAverage()
    if err != nil {
        result := gomonitor.NewCheckResult()
        result.SetResult(gomonitor.Unknown, fmt.Sprintf("%s", err))
        result.SendResult()
    }

    var state gomonitor.ExitCode
    var statusMsg string

    if loadAvg > criticalThreshold {
        state = gomonitor.Critical
        statusMsg = fmt.Sprintf("Load average %.2f is above critical threshold %.2f", loadAvg, criticalThreshold)
    } else if loadAvg > warningThreshold {
        state = gomonitor.Warning
        statusMsg = fmt.Sprintf("Load average %.2f is above warning threshold %.2f", loadAvg, warningThreshold)
    } else {
        state = gomonitor.OK
        statusMsg = fmt.Sprintf("Load average %.2f is below thresholds (warning=%.2f, critical=%.2f)", loadAvg, warningThreshold, criticalThreshold)
    }

    result := gomonitor.NewCheckResult()
    result.SetResult(state, statusMsg)

    // Add performance data
    metric := gomonitor.PerformanceMetric{
        Value:  loadAvg,
        Warn:   warningThreshold,
        Crit:   criticalThreshold,
        Min:    0,
        Max:    100, // Example max value
        UnitOM: "",
    }
    result.AddPerformanceData("load1", metric)

    // Output the result and exit with the appropriate exit code
    result.SendResult()
}
```

## API Reference

### ExitCode

The `ExitCode` type represents a Nagios exit code.

```go
type ExitCode int

const (
    OK      ExitCode = iota // 0 - Everything is fine
    Warning                // 1 - Potential issue, but not critical
    Critical               // 2 - Serious issue that requires immediate attention
    Unknown                // 3 - Plugin was unable to determine the status of the check
)
```

### CheckResult

The `CheckResult` type represents the result of a monitoring check.

```go
type CheckResult struct {
    ExitCode        gomonitor.ExitCode
    Message         string
    PerfOrder       []string
    PerformanceData map[string]gomonitor.PerformanceMetric
    Format          string
    StatusPrefix    bool // true by default; set false to omit the status prefix
}
```

#### Methods

- `NewCheckResult()` - Creates a new check result with default values
- `SetResult(ec ExitCode, msg string)` - Sets the exit code and message for the check result
- `AddPerformanceData(metricName string, metric PerformanceMetric)` - Adds a performance metric to the check result
- `UpdatePerformanceData(metricName string, metric PerformanceMetric)` - Updates an existing performance metric
- `DeletePerformanceData(metricName string)` - Deletes a performance metric from the check result
- `FormatResult() string` - Formats the check result message with performance data (does not exit). The `Format` template supports two `%s` verbs (status, message); other `%` characters are preserved literally, and `%%` collapses to a single `%` for backward compatibility.
- `SendResult()` - Outputs the formatted message and exits with the appropriate exit code

### PerformanceMetric

The `PerformanceMetric` type represents a performance metric.

```go
type PerformanceMetric struct {
    Value  float64 // The actual value of the metric
    Warn   float64 // Threshold for warning state
    Crit   float64 // Threshold for critical state
    Min    float64 // Minimum expected value of the metric
    Max    float64 // Maximum expected value of the metric
    UnitOM string  // Unit of measure for the metric (e.g., "ms", "%", etc.)
}
```

## Contributing

Contributions are welcome! Here's how you can contribute to gomonitor:

1. Fork the repository and create your branch from `main`.
2. If you're adding new features, please add corresponding tests.
3. Make sure your code follows Go best practices and is well-documented.
4. Issue a pull request with a clear description of your changes.

Please open an issue first to discuss any major changes before submitting a pull request.

For more information on contributing, see the [CONTRIBUTING.md](CONTRIBUTING.md) file.

This project is licensed under the Apache License, Version 2.0. See the [LICENSE](LICENSE) file for details.
