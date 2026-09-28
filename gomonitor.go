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
	"math"
	"os"
	"slices"
	"strconv"
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
	return int(ec)
}

// PerformanceMetric represents a performance metric with various attributes.
//   - `Value` is the actual value of the metric.
//   - `Warn` and `Crit` are optional threshold values for warning and critical
//     states respectively. A nil pointer means the threshold is unset and it
//     is omitted from FormatResult output, matching Icinga 2's PerfdataValue,
//     which drops empty threshold fields; a zero-value metric therefore
//     renders only its value. This is a breaking change from earlier
//     releases, which used plain float64 fields that could not represent an
//     unset threshold.
//   - `Min` and `Max` represent the optional minimum and maximum expected
//     values of the metric (nil to omit).
//   - `UnitOM` is the unit of measure for the metric. A unit Icinga 2 cannot
//     read back (one containing a digit, '.', ',', ';', '=' or whitespace)
//     is dropped from the output, as Icinga drops units it does not know.
type PerformanceMetric struct {
	Value  float64
	Warn   *float64
	Crit   *float64
	Min    *float64
	Max    *float64
	UnitOM string
}

// CheckResult represents the result of a Monitoring check.
//   - `ExitCode` is the exit code of the check, indicating the status of the check.
//     It is embedded, so CheckResult also satisfies fmt.Stringer through
//     promotion: printing a CheckResult with %s renders the exit status string
//     (e.g. "OK"), not the message.
//   - `Message` is a descriptive message associated with the check result.
//     It forms the short (first) line of the output.
//   - `LongOutput` is optional multi-line text rendered after the first line,
//     following the Nagios/Icinga plugin output convention: the first line is
//     the short output, every subsequent line is long output (Icinga 2 stores
//     it separately in CompatUtility::GetCheckResultLongOutput). FormatResult
//     sanitizes it by stripping '\r' and every '|' that is followed by an
//     '=' on the same line (Icinga would parse the rest of that line as
//     performance data) and trimming trailing line breaks; '\n' is preserved
//     as the line separator and other pipes are kept.
//   - `PerformanceData` is a map containing performance metrics associated with the check result.
//   - `Format` is the format string used to generate the output message.
//   - `NormalizeUnits` enables Icinga 2-style unit-of-measure normalization
//     (mirroring PerfdataValue::Parse followed by PerfdataValue::Format):
//     each metric's value and set thresholds are multiplied by the unit's
//     factor and the canonical short unit is rendered (e.g. "ms" scales by
//     1/1000 and renders as "s", "KiB" scales by 1024 and renders as "B").
//     The counter unit "c" renders as "c" without scaling; an unknown unit
//     is dropped and the value kept, matching Icinga. Normalized numbers use
//     Icinga's display format (six decimal places for fractions), so they are
//     rounded, and a value that overflows when scaled renders blank where
//     Icinga would print "inf". Off by default, and not needed for Icinga:
//     Icinga normalizes perfdata itself when it parses the plugin output
//     (the perfdata writers and Icinga DB's normalized_performance_data),
//     while its raw performance_data keeps the units the author chose.
//     Normalizing in the plugin only loses precision and the original units.
//   - `StatusPrefix` controls whether the exit code (e.g. "OK") is prepended to the output.
//     It is set to true by NewCheckResult. Note that a zero-value CheckResult
//     (e.g. &CheckResult{Message: "..."}) has StatusPrefix false, so the
//     status prefix is omitted; set it to true explicitly when building the
//     struct by hand. Set it to false when the message already carries its own
//     status prefix to avoid doubling (e.g. "OK: CPU usage...").
type CheckResult struct {
	ExitCode
	Message         string
	LongOutput      string
	PerfOrder       []string
	PerformanceData map[string]PerformanceMetric
	Format          string
	NormalizeUnits  bool
	StatusPrefix    bool
}

// SetResult sets the ExitCode and Message fields of the CheckResult to the provided values.
func (cr *CheckResult) SetResult(ec ExitCode, msg string) {
	cr.ExitCode = ec
	cr.Message = msg
}

// AddPerformanceData adds a performance metric to the CheckResult's PerformanceData map.
//
// Icinga 2 treats "::" in a label as a check_multi prefix (SplitPerfdata):
// once a label containing "::" is rendered, every following label without
// "::" is stored with that prefix, so "app::rta" followed by "load" is stored
// as "app::load". Give every label a prefix, or add unprefixed metrics first,
// when mixing the two. A label that is empty after sanitizing is not rendered.
//
// If the PerformanceData map or the PerfOrder slice is nil (e.g. on a
// zero-value or partially hand-built CheckResult), each is initialized before
// the metric is stored, so the method never panics on missing internal state.
func (cr *CheckResult) AddPerformanceData(metricName string, metric PerformanceMetric) {
	if cr.PerformanceData == nil {
		cr.PerformanceData = make(map[string]PerformanceMetric)
	}
	if cr.PerfOrder == nil {
		cr.PerfOrder = []string{}
	}

	// PerfOrder is exported and can already contain the name while
	// PerformanceData does not (e.g. a hand-built result with an order
	// slice but nil map). Appending unconditionally would register the
	// name twice and render the metric twice in FormatResult, so scan
	// the order slice before appending.
	if _, exists := cr.PerformanceData[metricName]; !exists && !slices.Contains(cr.PerfOrder, metricName) {
		cr.PerfOrder = append(cr.PerfOrder, metricName)
	}

	cr.PerformanceData[metricName] = metric
}

// UpdatePerformanceData adds or updates a performance metric in the
// CheckResult's PerformanceData map, registering new metric names in PerfOrder
// so they appear in FormatResult output. If the PerformanceData map is nil, it
// is initialized before the metric is stored. For an existing metric name the
// value is replaced in place and its position in PerfOrder is preserved; for a
// new name it is appended. Behavior is identical to AddPerformanceData.
func (cr *CheckResult) UpdatePerformanceData(metricName string, metric PerformanceMetric) {
	cr.AddPerformanceData(metricName, metric)
}

// DeletePerformanceData deletes the specified metric from the PerformanceData map of the CheckResult.
// If the PerformanceData map does not contain the specified metric, no action is taken.
//
// After the delete, the metric name is absent from both PerformanceData and
// PerfOrder, and the remaining metrics keep their relative order. If PerfOrder
// was modified externally through the exported field and no longer contains
// the metric, the order slice is left alone rather than indexed out of range.
func (cr *CheckResult) DeletePerformanceData(metricName string) {
	if _, exists := cr.PerformanceData[metricName]; !exists {
		return
	}

	delete(cr.PerformanceData, metricName)

	// Locate the metric's position in PerfOrder. It may be absent when
	// PerfOrder was modified externally through the exported field; in that
	// case there is nothing to remove from the order slice.
	if index := slices.Index(cr.PerfOrder, metricName); index >= 0 {
		cr.PerfOrder = slices.Delete(cr.PerfOrder, index, index+1)
	}
}

// FormatResult formats the check result message with performance data, but does not exit the program.
// This allows for more flexible usage of the library.
//
// If LongOutput is set, it is appended after the first line as the long
// output of the check, matching the Nagios/Icinga plugin output convention.
// When the first line would be blank and LongOutput is set, the status string
// fills it: Icinga trims the output and skips leading empty lines, so a blank
// first line would promote the first long-output line to the short output.
//
// The Format template supports the two verbs used by the default format
// ("%s: %s"): %s is replaced with the status string, and the second %s is
// replaced with the message. Any other '%' in the template is preserved
// literally, so a template such as "[%s] %s (95% sure)" is safe without
// escaping the percent as "%%". For backward compatibility, an explicit
// "%%" in the template is still collapsed to a single "%". An empty Format
// uses the default "%s: %s"; a template with no %s verb is rendered as-is,
// without the message.
//
// An ExitCode outside OK..Unknown renders as "Unknown", the state Icinga
// records for it.
func (cr *CheckResult) FormatResult() string {
	status := cr.state().String()
	output := sanitizeMessage(cr.Message)
	if cr.StatusPrefix {
		format := cr.Format
		if format == "" {
			format = defaultFormat
		}
		output = formatTemplate(format, status, output)
	}

	long := sanitizeLongOutput(cr.LongOutput)
	if long != "" && strings.TrimSpace(output) == "" {
		output = status
	}

	// Append performance data to the message. Each metric is preceded by a
	// separator space. When no metric renders (e.g. every PerfOrder entry is
	// stale after an external modification, or every label is empty),
	// nothing is appended rather than emitting an empty perfdata section
	// with a dangling '|'.
	var perf strings.Builder
	for _, key := range cr.PerfOrder {
		metric, ok := cr.PerformanceData[key]
		if !ok {
			// PerfOrder may be modified externally through the exported
			// field; skip names that are not in PerformanceData so a stale
			// entry cannot render a zero-value metric.
			continue
		}
		label := sanitizeLabel(key)
		if label == "" {
			// Icinga strips the quotes only from labels longer than two
			// characters, so "''" would be stored as a literal label.
			continue
		}
		perf.WriteByte(' ')
		perf.WriteString(metric.format(label, cr.NormalizeUnits))
	}
	if perf.Len() > 0 {
		output = output + " |" + perf.String()
	}

	// Append the long output. In the Nagios/Icinga plugin output format the
	// first line is the short output (message and performance data) and every
	// following line is long output; Icinga 2 keeps them separately in
	// CompatUtility::GetCheckResultOutput and GetCheckResultLongOutput.
	if long != "" {
		output = output + "\n" + long
	}

	return output
}

// format renders one perfdata token, 'label'=value[UOM];warn;crit;min;max.
//
// Normal output renders numbers losslessly (formatPerfFloat) with the unit the
// author chose. With normalize set, the metric is first scaled to its
// canonical unit and rendered exactly as Icinga 2's PerfdataValue::Format
// would render it (formatIcingaFloat).
func (m PerformanceMetric) format(label string, normalize bool) string {
	unit := perfUnit(m.UnitOM)
	formatFloat := formatPerfFloat
	if normalize {
		// Normalization derives the unit from the Icinga 2 UoM tables in
		// uom.go; units Icinga cannot parse are unknown there and dropped.
		m, unit = m.normalized()
		formatFloat = formatIcingaFloat
	}

	value := formatFloat(m.Value)
	if value == "" {
		// A non-finite value renders blank; emitting the unit right after
		// '=' would put a non-numeric unit string in the value position
		// (e.g. 'm'=ms), so the unit is suppressed along with the value.
		unit = ""
	}

	return "'" + label + "'=" + value + unit + formatThresholds(formatFloat, m.Warn, m.Crit, m.Min, m.Max)
}

// state returns the exit code Icinga records for the result. Icinga maps every
// exit status other than 0, 1 and 2 to Unknown (PluginUtility::ExitStatusToState),
// and for a status above 3 it appends "<Terminated with exit code N (0xN).>"
// to the plugin output, which would corrupt the last perfdata token.
func (cr *CheckResult) state() ExitCode {
	if cr.ExitCode < OK || cr.ExitCode > Unknown {
		return Unknown
	}
	return cr.ExitCode
}

// sanitizeLongOutput strips characters from long output that would corrupt
// the Nagios plugin output format or allow perfdata injection. '\r' is
// stripped because Icinga 2's ParseCheckOutput treats it as a line separator.
// ParseCheckOutput splits a line at its first '|' only when an '=' follows it,
// so every '|' that precedes the last '=' on a line is stripped and other
// pipes are kept as text. '\n' is preserved as the line separator; trailing
// line breaks are trimmed so the output ends cleanly.
func sanitizeLongOutput(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.TrimRight(s, "\n")
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if eq := strings.LastIndexByte(line, '='); eq > 0 {
			lines[i] = strings.ReplaceAll(line[:eq], "|", "") + line[eq:]
		}
	}
	return strings.Join(lines, "\n")
}

// sanitizeMessage strips characters from a plugin message that would break
// single-line Nagios output or allow output injection through the message:
// line breaks and the '|' perfdata separator. Everything after the first '|'
// is treated as perfdata by Nagios, so a '|' inside a message could forge
// additional output lines.
func sanitizeMessage(s string) string {
	s = strings.ReplaceAll(s, "|", "")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	return s
}

// sanitizeLabel strips the characters Icinga 2 cannot carry in a perfdata
// label. SplitPerfdata ends the label at the first '=', and a line break
// would end the perfdata line. Everything else round-trips: the label is
// wrapped in single quotes and Icinga strips only the outer pair, so an inner
// "'" survives, ';' is harmless before the last '=', and '|' is harmless after
// the perfdata delimiter.
func sanitizeLabel(s string) string {
	s = strings.ReplaceAll(s, "=", "")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	return s
}

// perfUnit returns the unit of measure to render, or "" when Icinga 2 could not
// read it back. PerfdataValue::Parse splits value from unit at the last digit
// or '.', SplitPerfdata splits tokens on spaces, and a ',' anywhere makes Icinga
// reject the token; ';' and '=' would shift the threshold fields or the label,
// and ASCII whitespace would split the token or the output line. Such a unit
// is dropped whole, which is how Icinga treats units it does not recognize,
// rather than stripped into a different unit ("k B" is not "kB").
func perfUnit(unit string) string {
	if strings.ContainsAny(unit, "0123456789.,;= \t\n\v\f\r") {
		return ""
	}
	return unit
}

// formatPerfFloat renders a perfdata numeric field losslessly: the shortest
// decimal string that parses back to the same float64, never in exponent
// notation, so a whole number renders without a decimal point. Icinga 2's
// PerfdataValue::Parse reads values and thresholds at full precision and the
// perfdata writers use the parsed double, so rounding here would change the
// data Icinga stores. Non-finite values (NaN, +Inf, -Inf) render as an empty
// string: Icinga rejects them, and an empty field is valid Nagios syntax.
func formatPerfFloat(f float64) string {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return ""
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// formatIcingaFloat renders a number the way Icinga 2 formats doubles in
// Convert::ToString(double) (lib/base/convert.cpp), which PerfdataValue::Format
// uses for normalized perfdata: a whole number renders without a decimal point,
// and a fractional number renders with six decimal places. Non-finite values
// render as an empty string, as in formatPerfFloat (Icinga would print "inf").
func formatIcingaFloat(f float64) string {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return ""
	}
	if f == math.Trunc(f) {
		return strconv.FormatFloat(f, 'f', 0, 64)
	}
	return strconv.FormatFloat(f, 'f', 6, 64)
}

// formatThresholds renders the ;warn;crit;min;max section of a perfdata token
// the way Icinga 2's PerfdataValue::Format does (lib/base/perfdatavalue.cpp):
// unset (nil) fields between two set fields keep their position as an empty
// field, while trailing unset fields are omitted entirely, so a metric with
// only warn and crit set renders ";warn;crit" and a metric with no set field
// renders no section at all. This keeps zero-value metrics from emitting
// misleading "0.00" thresholds.
func formatThresholds(formatFloat func(float64) string, warn, crit, min, max *float64) string {
	fields := []*float64{warn, crit, min, max}
	last := -1
	for i, field := range fields {
		if field != nil {
			last = i
		}
	}
	if last < 0 {
		return ""
	}

	var b strings.Builder
	for _, field := range fields[:last+1] {
		b.WriteByte(';')
		if field != nil {
			b.WriteString(formatFloat(*field))
		}
	}
	return b.String()
}

// formatTemplate renders the Format template safely. It substitutes the
// message for a single-verb template and, for multi-verb templates, the
// status for the first verb and the message for every subsequent verb —
// every other '%' passes through literally (no Sprintf interpretation,
// so stray percents in a template cannot produce "%!s(MISSING)" garbage).
// For backward compatibility, a "%%" in the template still collapses to a
// single "%" (the previous Sprintf escape hatch) and is scanned before the
// "%s" verbs, so a trailing "s" after "%%" stays literal.
func formatTemplate(template, status, message string) string {
	// Tokenize left to right: "%%" collapses to "%" before any "%s" verb
	// matching, matching the old Sprintf-based behavior where "%%s"
	// rendered as a literal "%s"; percents in user content pass through
	// untouched because only the template is scanned.
	type token struct {
		text string // literal text to emit verbatim
		verb bool   // "%s" verb to substitute
	}
	var tokens []token
	var text strings.Builder
	for i := 0; i < len(template); {
		switch {
		case strings.HasPrefix(template[i:], "%%"):
			text.WriteByte('%')
			i += 2
		case strings.HasPrefix(template[i:], "%s"):
			if text.Len() > 0 {
				tokens = append(tokens, token{text: text.String()})
				text.Reset()
			}
			tokens = append(tokens, token{verb: true})
			i += 2
		default:
			text.WriteByte(template[i])
			i++
		}
	}
	if text.Len() > 0 {
		tokens = append(tokens, token{text: text.String()})
	}

	verbCount := 0
	for _, tok := range tokens {
		if tok.verb {
			verbCount++
		}
	}

	var b strings.Builder
	verbSeen := 0
	for _, tok := range tokens {
		if tok.verb {
			verbSeen++
			switch {
			case verbCount == 1:
				// A single-verb template receives the message: the message
				// is the payload of the output and silently dropping it
				// would hide the diagnostic text. The status is still
				// conveyed by the exit code.
				b.WriteString(message)
			case verbSeen == 1:
				// The first verb of a multi-verb template is the status.
				b.WriteString(status)
			default:
				// Every subsequent verb receives the message.
				b.WriteString(message)
			}
			continue
		}
		b.WriteString(tok.text)
	}
	return b.String()
}

// SendResult outputs the formatted message and exits with the appropriate exit code.
// This is a convenience method that combines FormatResult with os.Exit.
//
// Note: os.Exit does not run deferred functions, so any cleanup registered
// with defer in the calling program is skipped. Callers that need deferred
// cleanup to run should print FormatResult() themselves and exit with the
// value returned by ResultCode().
func (cr *CheckResult) SendResult() {
	output := cr.FormatResult()
	fmt.Println(output)
	os.Exit(cr.ResultCode())
}

// ResultCode returns the integer exit code for the check result, matching
// what SendResult would pass to os.Exit, but without printing or exiting the
// program. This lets callers control termination themselves so that deferred
// functions in their own code still run.
//
// An ExitCode outside OK..Unknown returns 3 (Unknown): Icinga records any
// other exit status as Unknown anyway, and for a status above 3 (including
// os.Exit(-1), reported as 255) it appends "<Terminated with exit code N>"
// to the plugin output, corrupting the last perfdata token.
func (cr *CheckResult) ResultCode() int {
	return cr.state().Int()
}

// defaultFormat is the Format template set by NewCheckResult and used when
// Format is empty.
const defaultFormat = "%s: %s"

// NewCheckResult initializes a new check result with default values.
func NewCheckResult() *CheckResult {
	return &CheckResult{
		ExitCode:        OK,
		Format:          defaultFormat,
		StatusPrefix:    true,
		PerformanceData: make(map[string]PerformanceMetric),
		PerfOrder:       []string{},
	}
}
