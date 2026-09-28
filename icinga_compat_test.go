package gomonitor

import (
	"math"
	"strings"
	"testing"
)

// The tests in this file pin behavior against upstream Icinga 2
// (github.com/Icinga/icinga2 f68f3bc): PluginCheckTask::ProcessFinishedHandler,
// PluginUtility::ParseCheckOutput/SplitPerfdata and PerfdataValue::Parse.

// TestResultCode_OutOfRangeIsUnknown pins the exit-code clamp. Icinga maps
// every exit status other than 0-2 to Unknown (ExitStatusToState), and for a
// status above 3 it appends "<Terminated with exit code N (0xN).>" directly to
// the output, which glues onto the last perfdata token or long-output line.
// os.Exit(-1) is reported as 255, so negative codes are affected too.
func TestResultCode_OutOfRangeIsUnknown(t *testing.T) {
	for _, code := range []ExitCode{ExitCode(4), ExitCode(7), ExitCode(255), ExitCode(-1)} {
		r := NewCheckResult()
		r.SetResult(code, "x")
		if got := r.ResultCode(); got != 3 {
			t.Errorf("ResultCode() for %v = %d, want 3 (Unknown)", code, got)
		}
	}
}

// TestFormatResult_OutOfRangeStatusPrefix pins that the rendered status
// matches the state Icinga records: Unknown, not "ExitCode(7)".
func TestFormatResult_OutOfRangeStatusPrefix(t *testing.T) {
	r := NewCheckResult()
	r.SetResult(ExitCode(7), "x")
	if got, want := r.FormatResult(), "Unknown: x"; got != want {
		t.Errorf("FormatResult() = %q, want %q", got, want)
	}
}

// TestFormatResult_FullPrecision pins lossless numeric rendering. Icinga's
// PerfdataValue::Parse reads values with boost::lexical_cast<double> and the
// perfdata writers use the parsed double, so rendering only six decimals
// would round the data Icinga stores (30.54009269051229ms arrived as 30.54).
// The shortest round-trip decimal form never uses an exponent, which keeps
// thresholds within ParseWarnCritMinMaxToken's "+-0123456789.eE" charset.
func TestFormatResult_FullPrecision(t *testing.T) {
	testCases := []struct {
		name   string
		metric PerformanceMetric
		want   string
	}{
		{name: "fractional value", metric: PerformanceMetric{Value: 30.54009269051229, UnitOM: "ms"}, want: "'m'=30.54009269051229ms"},
		{name: "tiny value", metric: PerformanceMetric{Value: 0.00000025}, want: "'m'=0.00000025"},
		{name: "tiny threshold", metric: PerformanceMetric{Value: 1, Warn: new(0.0000005)}, want: "'m'=1;0.0000005"},
		{name: "large whole number", metric: PerformanceMetric{Value: 1e21}, want: "'m'=1000000000000000000000"},
		{name: "negative fractional", metric: PerformanceMetric{Value: -1.5}, want: "'m'=-1.5"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := NewCheckResult()
			r.SetResult(OK, "check")
			r.AddPerformanceData("m", tc.metric)
			if got := r.FormatResult(); got != "OK: check | "+tc.want {
				t.Errorf("FormatResult() = %q, want perfdata %q", got, tc.want)
			}
		})
	}
}

// TestFormatResult_UnsafeUnitDropped pins unit handling. Icinga splits value
// from unit at the last digit or '.' (PerfdataValue::Parse), splits perfdata
// tokens on spaces (SplitPerfdata) and rejects any token containing ','. A
// unit containing any of those, or ';' / '=', cannot round-trip, so it is
// dropped whole — Icinga's own treatment of units it does not recognize —
// rather than stripped into a different unit ("k B" would become "kB").
func TestFormatResult_UnsafeUnitDropped(t *testing.T) {
	testCases := []struct {
		unit string
		want string
	}{
		{unit: "1/s", want: "'m'=596;1"},
		{unit: "k B", want: "'m'=596;1"},
		{unit: "k\tB", want: "'m'=596;1"},
		{unit: "x,y", want: "'m'=596;1"},
		{unit: ".", want: "'m'=596;1"},
		{unit: "m3", want: "'m'=596;1"},
		{unit: "ms;injected", want: "'m'=596;1"},
		{unit: "C=injected", want: "'m'=596;1"},
		{unit: "ms", want: "'m'=596ms;1"},
		{unit: "KiB", want: "'m'=596KiB;1"},
		{unit: "%", want: "'m'=596%;1"},
		{unit: "°C", want: "'m'=596°C;1"},
	}

	for _, tc := range testCases {
		t.Run(tc.unit, func(t *testing.T) {
			r := NewCheckResult()
			r.SetResult(OK, "check")
			r.AddPerformanceData("m", PerformanceMetric{Value: 596, Warn: new(1.0), UnitOM: tc.unit})
			if got := r.FormatResult(); got != "OK: check | "+tc.want {
				t.Errorf("FormatResult() = %q, want perfdata %q", got, tc.want)
			}
		})
	}
}

// TestLookupUoM_ASCIILowercase pins Icinga's ASCII-only case folding
// (boost::algorithm::to_lower in the classic locale). Go's Unicode
// strings.ToLower folds the Kelvin sign U+212A to 'k', which would resolve
// "Kg" to kilograms where Icinga reports an unknown unit.
func TestLookupUoM_ASCIILowercase(t *testing.T) {
	for _, unit := range []string{"K", "Kg", "KB"} {
		factor, canonical, _, known := lookupUoM(unit)
		if known || factor != 1 || canonical != "" {
			t.Errorf("lookupUoM(%+q) = (%v, %q, known=%v), want unknown unit with factor 1", unit, factor, canonical, known)
		}
	}
}

// TestLookupUoM_NanoFactorsMatchIcinga pins the ns/ng factors bit-for-bit.
// Icinga computes 1.0 / 1000 / 1000 / 1000 step by step in double precision,
// giving 9.999999999999999e-10; a Go constant expression is evaluated
// exactly and gives 1e-09, which diverges on large normalized values.
func TestLookupUoM_NanoFactorsMatchIcinga(t *testing.T) {
	icinga := 1.0
	for range 3 {
		icinga /= 1000
	}
	for _, unit := range []string{"ns", "ng", "NS", "Ng"} {
		if factor, _, _, _ := lookupUoM(unit); factor != icinga {
			t.Errorf("lookupUoM(%q) factor = %v, want %v (Icinga's sequential division)", unit, factor, icinga)
		}
	}
}

// TestFormatResult_LabelRules pins the label rules. '=' is stripped because
// Icinga's SplitPerfdata ends the label at the first '=', line breaks would
// split the output, and "'" is stripped because the Nagios plugin guidelines
// forbid it (Icinga does not unescape the spec's two-single-quote escape, so
// it would store the doubled quote). ';' and '|' are allowed by the guidelines and
// handled by Icinga, so they are kept.
func TestFormatResult_LabelRules(t *testing.T) {
	testCases := []struct {
		label string
		want  string
	}{
		{label: "it's", want: "'its'=1"},
		{label: "'ab'", want: "'ab'=1"},
		{label: "semi;colon", want: "'semi;colon'=1"},
		{label: "pipe|x", want: "'pipe|x'=1"},
		{label: "disk usage", want: "'disk usage'=1"},
		{label: "a=b", want: "'ab'=1"},
		{label: "line\r\nbreak", want: "'linebreak'=1"},
	}

	for _, tc := range testCases {
		t.Run(tc.label, func(t *testing.T) {
			r := NewCheckResult()
			r.SetResult(OK, "check")
			r.AddPerformanceData(tc.label, PerformanceMetric{Value: 1})
			if got := r.FormatResult(); got != "OK: check | "+tc.want {
				t.Errorf("FormatResult() = %q, want perfdata %q", got, tc.want)
			}
		})
	}
}

// TestFormatResult_EmptyLabelSkipped pins that a metric whose label is empty
// after sanitizing is not rendered: Icinga strips the quotes only from labels
// longer than two characters, so an empty quoted label would be stored as the
// two quote characters themselves.
func TestFormatResult_EmptyLabelSkipped(t *testing.T) {
	r := NewCheckResult()
	r.SetResult(OK, "check")
	r.AddPerformanceData("", PerformanceMetric{Value: 1})
	r.AddPerformanceData("=", PerformanceMetric{Value: 2})
	r.AddPerformanceData("''", PerformanceMetric{Value: 4})
	r.AddPerformanceData("ok", PerformanceMetric{Value: 3})

	if got, want := r.FormatResult(), "OK: check | 'ok'=3"; got != want {
		t.Errorf("FormatResult() = %q, want %q", got, want)
	}

	r.DeletePerformanceData("ok")
	if got, want := r.FormatResult(), "OK: check"; got != want {
		t.Errorf("FormatResult() with only empty labels = %q, want %q", got, want)
	}
}

// TestFormatResult_BlankShortOutputWithLongOutput pins the short-output
// fallback. Icinga trims the plugin output and ParseCheckOutput skips leading
// empty lines, so a blank first line would promote the first long-output
// line to the short output. The status string fills the blank line instead.
func TestFormatResult_BlankShortOutputWithLongOutput(t *testing.T) {
	testCases := []struct {
		name    string
		message string
		perf    bool
		want    string
	}{
		{name: "empty message", message: "", want: "Warning\ndetail"},
		{name: "whitespace message", message: "   ", want: "Warning\ndetail"},
		{name: "message stripped to empty", message: "\n", want: "Warning\ndetail"},
		{name: "empty message with perfdata", message: "", perf: true, want: "Warning | 'm'=1\ndetail"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := NewCheckResult()
			r.StatusPrefix = false
			r.SetResult(Warning, tc.message)
			r.LongOutput = "detail"
			if tc.perf {
				r.AddPerformanceData("m", PerformanceMetric{Value: 1})
			}
			if got := r.FormatResult(); got != tc.want {
				t.Errorf("FormatResult() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestFormatResult_LongOutputPipeRule pins Icinga's per-line perfdata rule
// (ParseCheckOutput): a line is split at its first '|' only when an '='
// follows it. Only pipes followed by an '=' are stripped; other pipes are
// ordinary long-output text.
func TestFormatResult_LongOutputPipeRule(t *testing.T) {
	testCases := []struct {
		long string
		want string
	}{
		{long: "a | b", want: "a | b"},
		{long: "x | y=1", want: "x  y=1"},
		{long: "p|q|r=1", want: "pqr=1"},
		{long: "a=1 | b", want: "a=1 | b"},
		{long: "ok\nx | y=1\nc | d", want: "ok\nx  y=1\nc | d"},
	}

	for _, tc := range testCases {
		t.Run(tc.long, func(t *testing.T) {
			r := NewCheckResult()
			r.SetResult(OK, "check")
			r.LongOutput = tc.long
			if got := r.FormatResult(); got != "OK: check\n"+tc.want {
				t.Errorf("FormatResult() = %q, want long output %q", got, tc.want)
			}
		})
	}
}

// TestDeletePerformanceData_PreservesInsertionOrder pins order preservation:
// removing a middle metric must not move later metrics ahead of earlier ones.
func TestDeletePerformanceData_PreservesInsertionOrder(t *testing.T) {
	r := NewCheckResult()
	for _, name := range []string{"a", "b", "c", "d"} {
		r.AddPerformanceData(name, PerformanceMetric{Value: 1})
	}

	r.DeletePerformanceData("b")

	if got, want := strings.Join(r.PerfOrder, ","), "a,c,d"; got != want {
		t.Errorf("PerfOrder after delete = %q, want %q", got, want)
	}
}

// TestFormatResult_EmptyFormatUsesDefault pins the empty-Format fallback: a
// hand-built result with StatusPrefix true and no Format renders with the
// NewCheckResult default "%s: %s" instead of an empty string.
func TestFormatResult_EmptyFormatUsesDefault(t *testing.T) {
	r := &CheckResult{Message: "disk fine", StatusPrefix: true}
	if got, want := r.FormatResult(), "OK: disk fine"; got != want {
		t.Errorf("FormatResult() = %q, want %q", got, want)
	}
}

// TestFormatResult_BlankShortOutputIsASCIIWhitespace pins that only ASCII
// whitespace counts as blank, matching Icinga's boost::algorithm::trim in the
// classic locale: a message of U+00A0 is kept as the short output.
func TestFormatResult_BlankShortOutputIsASCIIWhitespace(t *testing.T) {
	r := NewCheckResult()
	r.StatusPrefix = false
	r.SetResult(OK, " ")
	r.LongOutput = "d"
	if got, want := r.FormatResult(), " \nd"; got != want {
		t.Errorf("FormatResult() = %+q, want %+q", got, want)
	}
}

// TestFormatResult_FormatTemplateSanitized pins that the Format template
// cannot corrupt the output either: a '|' in it would become Icinga's
// perfdata delimiter (ParseCheckOutput splits at the first '|' followed by
// an '='), and a line break would move the message into the long output.
func TestFormatResult_FormatTemplateSanitized(t *testing.T) {
	testCases := []struct {
		format string
		want   string
	}{
		{format: "%s | %s", want: "OK  msg | 'a'=1"},
		{format: "\n%s: %s\r", want: "OK: msg | 'a'=1"},
	}

	for _, tc := range testCases {
		t.Run(tc.format, func(t *testing.T) {
			r := NewCheckResult()
			r.Format = tc.format
			r.SetResult(OK, "msg")
			r.AddPerformanceData("a", PerformanceMetric{Value: 1})
			if got := r.FormatResult(); got != tc.want {
				t.Errorf("FormatResult() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestFormatResult_UndeterminedValueIsU pins the Monitoring Plugins guideline
// that a value which could not be determined is the literal "U". NaN and
// +/-Inf have no valid numeric rendering, so they render as "U" with the unit
// suppressed. Non-finite thresholds render as empty (null) fields, since "U"
// is only defined for the value. Icinga rejects the token either way.
func TestFormatResult_UndeterminedValueIsU(t *testing.T) {
	testCases := []struct {
		name      string
		metric    PerformanceMetric
		normalize bool
		want      string
	}{
		{name: "NaN", metric: PerformanceMetric{Value: math.NaN(), UnitOM: "ms", Warn: new(2.0)}, want: "'m'=U;2"},
		{name: "+Inf", metric: PerformanceMetric{Value: math.Inf(1)}, want: "'m'=U"},
		{name: "-Inf", metric: PerformanceMetric{Value: math.Inf(-1), Crit: new(math.NaN()), Max: new(1.0)}, want: "'m'=U;;;;1"},
		{name: "overflow when normalized", metric: PerformanceMetric{Value: 1e300, UnitOM: "YB"}, normalize: true, want: "'m'=U"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := NewCheckResult()
			r.NormalizeUnits = tc.normalize
			r.SetResult(OK, "check")
			r.AddPerformanceData("m", tc.metric)
			if got := r.FormatResult(); got != "OK: check | "+tc.want {
				t.Errorf("FormatResult() = %q, want perfdata %q", got, tc.want)
			}
		})
	}
}
