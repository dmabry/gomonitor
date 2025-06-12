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

## Installation

To install gomonitor, you can use the `go get` command:

```bash
go get github.com/dmabry/gomonitor
```

Alternatively, you can add the following import to your Go code:

```go
import "github.com/dmabry/gomonitor"
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
}
```

#### Methods

- `NewCheckResult()` - Creates a new check result with default values
- `SetResult(ec ExitCode, msg string)` - Sets the exit code and message for the check result
- `AddPerformanceData(metricName string, metric PerformanceMetric)` - Adds a performance metric to the check result
- `UpdatePerformanceData(metricName string, metric PerformanceMetric)` - Updates an existing performance metric
- `DeletePerformanceData(metricName string)` - Deletes a performance metric from the check result
- `FormatResult() string` - Formats the check result message with performance data (does not exit)
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

Contributions are welcome! Please open an issue or submit a pull request on GitHub.

## License

This project is licensed under the Apache License, Version 2.0. See the [LICENSE](LICENSE) file for details.
