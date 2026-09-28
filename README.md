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
  - [Long Output](#long-output)
  - [Performance Data and Icinga 2](#performance-data-and-icinga-2)
  - [Unit of Measure Normalization](#unit-of-measure-normalization)
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
- Output checked against Icinga 2's plugin output parser, so Icinga stores the values, units, and labels you set (with the few exceptions listed under [Performance Data and Icinga 2](#performance-data-and-icinga-2))
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

    // Add performance data. Warn, Crit, Min and Max are *float64 fields;
    // nil (or an omitted field) means the threshold is unset and it is
    // omitted from the output, matching Icinga 2.
    warn, crit, min, max := 1.00, 2.00, 0.00, 10.00
    metric := gomonitor.PerformanceMetric{
        Value:  1.23,
        Warn:   &warn,
        Crit:   &crit,
        Min:    &min,
        Max:    &max,
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
        Warn:   &warningThreshold,
        Crit:   &criticalThreshold,
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

The `Format` field controls the template used when the status prefix is enabled. It supports the `%s` verb: the first `%s` is replaced with the status string and every subsequent `%s` with the message; a template with a single `%s` receives the message (so the diagnostic text is never silently dropped — the status is still conveyed by the exit code). Any other `%` in the template is preserved literally — no escaping needed (for backward compatibility, an explicit `%%` still collapses to a single `%`):

```go
result.Format = "[%s] %s (95% sure)"
result.SetResult(gomonitor.Warning, "High latency")
fmt.Println(result.FormatResult()) // "[Warning] High latency (95% sure)"
```

An empty `Format` uses the default `"%s: %s"`, so a hand-built `&CheckResult{StatusPrefix: true, ...}` still renders its message. A template with no `%s` verb is rendered as-is, without the message.

If your message already carries its own status prefix (e.g. `"OK: Everything is fine"`), set `StatusPrefix` to `false` to avoid doubling it. Note that performance data is appended to the output automatically, so you should **not** add a `| %s` verb to your `Format` string.

### Long Output

Nagios/Icinga plugin output has two parts: the first line is the short output (message and performance data), and every subsequent line is the long output. Set the `LongOutput` field to emit multi-line output:

```go
result := gomonitor.NewCheckResult()
result.SetResult(gomonitor.OK, "CPU usage ok")
result.LongOutput = "Detail: 5 cores\nDetail: load average 0.5"
fmt.Println(result.FormatResult())
// OK: CPU usage ok
// Detail: 5 cores
// Detail: load average 0.5
```

`LongOutput` is sanitized when rendered: `\r` is stripped, every `|` that is followed by an `=` on the same line is stripped (Icinga 2's `ParseCheckOutput` would parse the rest of that line as performance data), and trailing line breaks are trimmed. `\n` is preserved as the line separator, and other pipes (e.g. `"a | b"`) are kept.

If the first line would be blank (for example `StatusPrefix` is `false` and `Message` is empty) while `LongOutput` is set, the status string (e.g. `Warning`) fills the first line. Icinga trims plugin output and skips leading empty lines, so a blank first line would otherwise turn the first long-output line into the short output.

### Performance Data and Icinga 2

Performance data is rendered so that Icinga 2 (`PluginUtility::SplitPerfdata` and `PerfdataValue::Parse`) reads back what you set. The exceptions are listed here: `=` and line breaks in labels, units Icinga cannot parse, non-finite numbers, and `::` prefixes.

- **Numbers are lossless.** Values and thresholds render in the shortest decimal form that parses back to the same `float64`, never in exponent notation: `'rta'=30.54009269051229ms`, `'m'=0.00000025`, `'m'=95`. `NaN` and `±Inf` render as an empty field (Icinga rejects them either way).
- **Labels** are always quoted. `=` and line breaks are stripped (Icinga ends a label at the first `=`); every other character is kept, including spaces, `'`, `;`, and `|`. A label that itself starts and ends with `'` gets a second pair of quotes, because Icinga unquotes labels twice. A metric whose label is empty after sanitizing is not rendered. Note that the Nagios plugin guidelines forbid `'` in labels; Icinga handles it, but other Nagios-ecosystem parsers may not.
- **Units** Icinga cannot read back are dropped whole, the same way Icinga drops units it does not recognize: a unit containing a digit, `.`, `,`, `;`, `=`, or whitespace (e.g. `"1/s"`, `"k B"`, `"m3"`) renders the value without a unit.
- **`::` labels** are check_multi prefixes to Icinga: once a label containing `::` is rendered, every following label without `::` is stored with that prefix (`app::rta` followed by `load` is stored as `app::load`). When mixing the two, give every label a prefix or add the unprefixed metrics first.

### Unit of Measure Normalization

By default, `UnitOM` passes through verbatim — the plugin output keeps the units you chose, which is what Nagios and Icinga receive. Icinga normalizes units itself when it parses the output (its perfdata writers and Icinga DB's `normalized_performance_data`), so `NormalizeUnits` is not needed for Icinga: it only rounds the values and replaces the units you chose in Icinga's raw `performance_data`. Set `NormalizeUnits` on the check result to enable Icinga 2-style normalization (mirroring `PerfdataValue::Parse` and `PerfdataValue::Format`): each metric's value and set thresholds are multiplied by the unit's factor and the canonical short unit is rendered:

```go
result := gomonitor.NewCheckResult()
result.NormalizeUnits = true
result.SetResult(gomonitor.OK, "latency ok")
rtaWarn, warn, crit := 50.0, 1.0, 2.0
result.AddPerformanceData("rta", gomonitor.PerformanceMetric{Value: 12.445, UnitOM: "ms", Warn: &rtaWarn})
result.AddPerformanceData("disk", gomonitor.PerformanceMetric{Value: 2, UnitOM: "kiB", Warn: &warn, Crit: &crit})
fmt.Println(result.FormatResult())
// OK: latency ok | 'rta'=0.012445s;0.050000 'disk'=2048B;1024;2048
```

Supported units mirror Icinga 2's tables:

- Case-sensitive: `%`, the counter unit `c`, `C` (celsius), data units `b`/`B` (bits/bytes) with SI prefixes `k`…`Y` and IEC `i`/`I` suffixes, energy units `a`/`o`/`v`/`w` (amperes/ohms/volts/watts) with prefixes, and charge/energy forms with time suffixes (`As`, `Wh`, …).
- Case-insensitive: time `ns`…`d` → seconds, mass `ng`…`t` → grams, volume `ml`/`l`/`hl` → liters, plus `packets`, `lm`, `dbm`, `f`, `k`.
- The counter unit `c` renders as `c` without scaling. An unknown unit is dropped and the value kept, matching Icinga's behavior.
- Case-insensitive matching folds ASCII letters only, as Icinga does.

Normalized numbers use Icinga's display format (`PerfdataValue::Format`): whole numbers without a decimal point and fractional numbers with six decimal places. A value that overflows when scaled renders as an empty field, where Icinga would print `inf`.

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
        Warn:   &warningThreshold,
        Crit:   &criticalThreshold,
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

A value outside `OK`..`Unknown` (e.g. `ExitCode(7)`) is reported as `Unknown`: `FormatResult()` renders the status as `Unknown` and `ResultCode()`/`SendResult()` exit with 3. Icinga records any other exit status as Unknown anyway, and for a status above 3 it appends `<Terminated with exit code N>` to the plugin output, corrupting the last performance metric. `ExitCode.String()` and `ExitCode.Int()` still return the raw value (`"ExitCode(7)"`, `7`).

### CheckResult

The `CheckResult` type represents the result of a monitoring check.

```go
type CheckResult struct {
    ExitCode        // embedded Nagios exit code (also provides String() via promotion)
    Message         string
    LongOutput      string // optional multi-line long output, rendered after the first line
    PerfOrder       []string
    PerformanceData map[string]gomonitor.PerformanceMetric
    Format          string
    NormalizeUnits  bool // Icinga 2-style UoM normalization (opt-in, off by default)
    StatusPrefix    bool // set true by NewCheckResult; zero-value structs have it false (no status prefix)
}
```

#### Methods

- `NewCheckResult()` - Creates a new check result with default values
- `SetResult(ec ExitCode, msg string)` - Sets the exit code and message for the check result
- `AddPerformanceData(metricName string, metric PerformanceMetric)` - Adds a performance metric to the check result
- `UpdatePerformanceData(metricName string, metric PerformanceMetric)` - Adds or updates a performance metric. For a new metric name the metric is registered and appears in the output (identical to `AddPerformanceData`); for an existing name the value is replaced in place and its position in the output order is preserved.
- `DeletePerformanceData(metricName string)` - Deletes a performance metric from the check result; the remaining metrics keep their order
- `FormatResult() string` - Formats the check result message with performance data (does not exit). The `Format` template supports two `%s` verbs (status, message); other `%` characters are preserved literally, and `%%` collapses to a single `%` for backward compatibility. To keep single-line output well-formed, the first line (message and `Format` template) is stripped of newlines and `|`. Labels are stripped of `=` and newlines, and units Icinga cannot read back are dropped (see [Performance Data and Icinga 2](#performance-data-and-icinga-2)).
- `SendResult()` - Outputs the formatted message and exits with the appropriate exit code. **Caution:** `os.Exit` skips deferred cleanup in the calling program; use `ResultCode()` and print `FormatResult()` yourself when deferred functions must run.
- `ResultCode() int` - Returns the integer exit code for the check result without printing or exiting, so callers can control termination (e.g. let their own defer run) instead of relying on `SendResult()`. Codes outside 0–3 return 3 (Unknown).

### PerformanceMetric

The `PerformanceMetric` type represents a performance metric.

```go
type PerformanceMetric struct {
    Value  float64  // The actual value of the metric
    Warn   *float64 // Threshold for warning state (nil to omit)
    Crit   *float64 // Threshold for critical state (nil to omit)
    Min    *float64 // Minimum expected value of the metric (nil to omit)
    Max    *float64 // Maximum expected value of the metric (nil to omit)
    UnitOM string   // Unit of measure for the metric (e.g., "ms", "%", etc.)
}
```

Unset (nil) thresholds are omitted from the output, matching Icinga 2's `PerfdataValue` behavior: a metric with only `Warn` and `Crit` set renders `'m'=5;80;90`, and a zero-value metric renders `'m'=5` without any threshold fields. Numeric fields render losslessly: whole numbers without a decimal point (`'m'=95`) and fractional numbers in the shortest form that round-trips (`'m'=1.23`). With `NormalizeUnits`, numbers use Icinga 2's display format instead (six decimal places for fractions).

## Contributing

Contributions are welcome! Here's how you can contribute to gomonitor:

1. Fork the repository and create your branch from `main`.
2. If you're adding new features, please add corresponding tests.
3. Make sure your code follows Go best practices and is well-documented.
4. Issue a pull request with a clear description of your changes.

Please open an issue first to discuss any major changes before submitting a pull request.

For more information on contributing, see the [CONTRIBUTING.md](CONTRIBUTING.md) file.

This project is licensed under the Apache License, Version 2.0. See the [LICENSE](LICENSE) file for details.
