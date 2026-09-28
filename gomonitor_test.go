package gomonitor

import (
	"math"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// fp returns a pointer to v, for building PerformanceMetric literals with the
// optional (*float64) threshold fields.
func new(v float64) *float64 { return &v }

func TestExitCodeString(t *testing.T) {
	testCases := []struct {
		name string
		code ExitCode
		want string
	}{
		{"Test OK", OK, "OK"},
		{"Test Warning", Warning, "Warning"},
		{"Test Critical", Critical, "Critical"},
		{"Test Unknown", Unknown, "Unknown"},
		{"Test Non-Exist", ExitCode(100), "ExitCode(100)"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.code.String()
			if got != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestExitCodeInt(t *testing.T) {
	testCases := []struct {
		name string
		code ExitCode
		want int
	}{
		{"Test OK", OK, 0},
		{"Test Warning", Warning, 1},
		{"Test Critical", Critical, 2},
		{"Test Unknown", Unknown, 3},
		{"Test Non-Exist", ExitCode(100), 100},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.code.Int()
			if got != tc.want {
				t.Errorf("got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestNewCheckResult(t *testing.T) {
	result := NewCheckResult()

	if result.ExitCode != OK {
		t.Errorf("NewCheckResult got exitCode %d, want 0", result.ExitCode)
	}
	if len(result.PerformanceData) != 0 {
		t.Errorf("NewCheckResult got PerformanceData length %d, want 0", len(result.PerformanceData))
	}
}

func TestSetResult(t *testing.T) {
	result := NewCheckResult()
	result.SetResult(Warning, "Test message")

	if result.ExitCode != Warning {
		t.Errorf("SetResult got exitCode %d, want 1", result.ExitCode)
	}
	if result.Message != "Test message" {
		t.Errorf("SetResult got message %s, want 'Test message'", result.Message)
	}
}

func TestPerformanceData(t *testing.T) {
	testMetric := PerformanceMetric{
		Value:  1.23,
		Warn:   new(1.00),
		Crit:   new(2.00),
		Min:    new(0.00),
		Max:    new(10.00),
		UnitOM: "ms",
	}

	result := NewCheckResult()
	result.AddPerformanceData("test", testMetric)

	if _, ok := result.PerformanceData["test"]; !ok {
		t.Error("AddPerformanceData didn't add the 'test' performance data to the map")
	}

	testMetric2 := PerformanceMetric{
		Value:  2.34,
		Warn:   new(2.00),
		Crit:   new(3.00),
		Min:    new(1.00),
		Max:    new(20.00),
		UnitOM: "s",
	}
	result.UpdatePerformanceData("test", testMetric2)

	updatedMetric := result.PerformanceData["test"]
	if updatedMetric.Value != 2.34 {
		t.Error("UpdatePerformanceData didn't correctly update the 'test' performance data")
	}

	result.DeletePerformanceData("test")
	if _, ok := result.PerformanceData["test"]; ok {
		t.Error("DeletePerformanceData didn't delete the 'test' performance data from the map")
	}
}

func TestSendResult(t *testing.T) {
	if os.Getenv("BE_CRASHER") == "1" {
		result := NewCheckResult()
		result.SetResult(OK, "Test Message")
		result.SendResult()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestSendResult")
	cmd.Env = append(os.Environ(), "BE_CRASHER=1")
	err := cmd.Run()

	// err is not nil when the program exits with a non-zero exit code.

	if exitError, ok := err.(*exec.ExitError); ok { // Program has exited with a non-zero exit code.
		if status := exitError.ExitCode(); status != OK.Int() {
			t.Fatalf("process ran with err %v, want exit status %d", err, OK.Int())
		}
	} else if err != nil {
		t.Fatal("cmd.Run() failed with an unexpected error:", err)
	}
}

func TestResultCode(t *testing.T) {
	testCases := []struct {
		name string
		code ExitCode
		want int
	}{
		{"Test OK", OK, 0},
		{"Test Warning", Warning, 1},
		{"Test Critical", Critical, 2},
		{"Test Unknown", Unknown, 3},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := NewCheckResult()
			r.SetResult(tc.code, "message")

			if got := r.ResultCode(); got != tc.want {
				t.Errorf("ResultCode got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestFormatResult(t *testing.T) {
	testCases := []struct {
		name     string
		setup    func() *CheckResult
		wantOK   bool
		contains []string
		want     string // exact output when non-empty
	}{
		{
			name: "NoPerfData_NoMetrics",
			setup: func() *CheckResult {
				r := NewCheckResult()
				r.SetResult(OK, "Everything is fine")
				return r
			},
			wantOK:   true,
			contains: []string{"OK", "Everything is fine"},
		},
		{
			name: "SinglePerfData_WithMetrics",
			setup: func() *CheckResult {
				r := NewCheckResult()
				r.SetResult(Warning, "High latency detected")
				r.AddPerformanceData("response_time", PerformanceMetric{
					Value: 1.23, Warn: new(1.00), Crit: new(2.00), Min: new(0.00), Max: new(10.00), UnitOM: "ms",
				})
				return r
			},
			wantOK:   true,
			contains: []string{"Warning", "High latency detected", "'response_time'=1.23ms;1;2;0;10"},
		},
		{
			name: "MultiplePerfData_MultiMetrics",
			setup: func() *CheckResult {
				r := NewCheckResult()
				r.SetResult(Critical, "CPU overloaded")
				r.AddPerformanceData("cpu_usage", PerformanceMetric{Value: 95.0, Warn: new(80.0), Crit: new(90.0), Min: new(0.0), Max: new(100.0)})
				r.AddPerformanceData("memory_usage", PerformanceMetric{Value: 88.5, Warn: new(85.0), Crit: new(95.0), Min: new(0.0), Max: new(100.0)})
				return r
			},
			wantOK:   true,
			contains: []string{"Critical", "CPU overloaded"},
		},
		{
			name: "CustomFormatString",
			setup: func() *CheckResult {
				r := NewCheckResult()
				r.Format = "[%s] %s (details: %%s)"
				r.SetResult(Unknown, "Plugin unable to determine status")
				return r
			},
			wantOK:   true,
			contains: []string{"[Unknown]", "Plugin unable to determine status"},
			// Pins the %% escape: "%%s" must render as a literal "%s",
			// not a third verb slot that duplicates the message.
			want: "[Unknown] Plugin unable to determine status (details: %s)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.setup()
			output := result.FormatResult()

			if tc.wantOK && output == "" {
				t.Error("FormatResult returned empty string")
			}

			if tc.want != "" && output != tc.want {
				t.Errorf("FormatResult got %q, want %q", output, tc.want)
			}

			for _, c := range tc.contains {
				if !containsString(output, c) {
					t.Errorf("Output %q does not contain expected substring %q", output, c)
				}
			}
		})
	}
}

func TestFormatResult_DefaultFormatPrefix(t *testing.T) {
	testCases := []struct {
		name string
		code ExitCode
		want string
	}{
		{name: "OK", code: OK, want: "OK: Everything is fine"},
		{name: "Warning", code: Warning, want: "Warning: Everything is fine"},
		{name: "Critical", code: Critical, want: "Critical: Everything is fine"},
		{name: "Unknown", code: Unknown, want: "Unknown: Everything is fine"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := NewCheckResult()
			r.SetResult(tc.code, "Everything is fine")

			if got := r.FormatResult(); got != tc.want {
				t.Errorf("FormatResult got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFormatResult_StatusPrefixDisabled(t *testing.T) {
	r := NewCheckResult()
	r.StatusPrefix = false
	r.SetResult(OK, "OK: Everything is fine")

	if got := r.FormatResult(); got != "OK: Everything is fine" {
		t.Errorf("FormatResult got %q, want %q (no status prefix prepended)", got, "OK: Everything is fine")
	}

	r2 := NewCheckResult()
	r2.StatusPrefix = false
	r2.SetResult(Warning, "High latency")
	r2.AddPerformanceData("response_time", PerformanceMetric{
		Value: 1.23, Warn: new(1.00), Crit: new(2.00), Min: new(0.00), Max: new(10.00), UnitOM: "ms",
	})
	if got := r2.FormatResult(); !containsString(got, "High latency | 'response_time'=1.23ms;1;2;0;10") {
		t.Errorf("FormatResult with perfdata got %q, want to contain %q", got, "High latency | 'response_time'=1.23ms;1;2;0;10")
	}
}

func TestFormatResult_PerformanceData(t *testing.T) {
	r := NewCheckResult()
	r.SetResult(OK, "Check passed")
	r.AddPerformanceData("test_metric", PerformanceMetric{
		Value: 42.5, Warn: new(30.0), Crit: new(50.0), Min: new(0.0), Max: new(100.0), UnitOM: "%",
	})
	output := r.FormatResult()

	wantFormat := "'test_metric'=42.5%;30;50;0;100"
	if !containsString(output, wantFormat) {
		t.Errorf("Performance data format incorrect.\nGot: %s\nExpected substring: %s", output, wantFormat)
	}
}

func TestFormatResult_SanitizesPerfData(t *testing.T) {
	r := NewCheckResult()
	r.SetResult(OK, "check\nok")
	r.AddPerformanceData("label|with;bad'\nchar", PerformanceMetric{
		Value: 1.0, Warn: new(2.0), Crit: new(3.0), Min: new(0.0), Max: new(10.0), UnitOM: "ms;injected",
	})

	got := r.FormatResult()

	// Only the line break is stripped from the label; the unit contains ';'
	// and cannot round-trip through Icinga's parser, so it is dropped.
	wantLabel := "'label|with;bad'char'=1;2;3;0;10"
	if !strings.Contains(got, wantLabel) {
		t.Errorf("FormatResult %q does not contain sanitized perfdata %q", got, wantLabel)
	}
	if !strings.Contains(got, "checkok") {
		t.Errorf("FormatResult %q should have stripped newline from message", got)
	}
	if strings.Contains(got, "\n") {
		t.Errorf("FormatResult %q still contains an unsanitized newline", got)
	}
	if strings.Contains(got, "injected") {
		t.Errorf("FormatResult %q still contains the unsafe unit", got)
	}
}

func TestFormatResult_NonFinitePerfData(t *testing.T) {
	testCases := []struct {
		name   string
		metric PerformanceMetric
		want   string
	}{
		{
			name:   "NaN value",
			metric: PerformanceMetric{Value: math.NaN(), Warn: new(2.0), Crit: new(3.0), Min: new(0.0), Max: new(10.0)},
			want:   "'m'=;2;3;0;10",
		},
		{
			// Pins the unit suppression: a non-finite value with a
			// non-empty UOM must not emit the unit string in the value
			// position ('m'=ms), which corrupts the token for strict
			// Nagios parsers.
			name:   "NaN value with unit",
			metric: PerformanceMetric{Value: math.NaN(), UnitOM: "ms", Warn: new(2.0), Crit: new(3.0), Min: new(0.0), Max: new(10.0)},
			want:   "'m'=;2;3;0;10",
		},
		{
			name:   "Infinity thresholds",
			metric: PerformanceMetric{Value: 1.0, Warn: new(math.Inf(1)), Crit: new(math.Inf(-1)), Min: new(0.0), Max: new(10.0)},
			want:   "'m'=1;;;0;10",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := NewCheckResult()
			r.SetResult(OK, "check")
			r.AddPerformanceData("m", tc.metric)

			got := r.FormatResult()
			if !strings.Contains(got, tc.want) {
				t.Errorf("FormatResult %q does not contain expected perfdata %q", got, tc.want)
			}
			if strings.Contains(got, "NaN") || strings.Contains(got, "Inf") {
				t.Errorf("FormatResult %q contains a non-finite token", got)
			}
		})
	}
}

func TestFormatResult_NoTrailingSpace(t *testing.T) {
	r := NewCheckResult()
	r.SetResult(OK, "check")
	r.AddPerformanceData("m", PerformanceMetric{Value: 1, Warn: new(2), Crit: new(3), Min: new(0), Max: new(10)})

	got := r.FormatResult()
	want := "OK: check | 'm'=1;2;3;0;10"
	if got != want {
		t.Errorf("FormatResult got %q, want %q (output must not end with a trailing space)", got, want)
	}
}

// TestFormatResult_SanitizesMessage pins the message sanitization: '|', '\r',
// and '\n' are stripped from the message so single-line Nagios output stays
// well-formed and the '|' cannot forge a fake perfdata section (documented on
// sanitizeMessage and in README.md).
func TestFormatResult_SanitizesMessage(t *testing.T) {
	r := NewCheckResult()
	r.SetResult(OK, "forged|pipe\rline\nbreak")

	got := r.FormatResult()
	want := "OK: forgedpipelinebreak"
	if got != want {
		t.Errorf("FormatResult got %q, want %q (message must be stripped of '|', '\\r', '\\n')", got, want)
	}
}

// TestFormatResult_LongOutput pins the multi-line output capability: LongOutput
// is appended after the first line as the long output of the check, following
// the Nagios/Icinga plugin output convention (Icinga 2 stores short and long
// output separately via CompatUtility::GetCheckResultOutput and
// GetCheckResultLongOutput).
func TestFormatResult_LongOutput(t *testing.T) {
	r := NewCheckResult()
	r.SetResult(OK, "CPU usage ok")
	r.LongOutput = "detail line 1\ndetail line 2"
	r.AddPerformanceData("cpu", PerformanceMetric{Value: 5, Warn: new(80), Crit: new(90)})

	got := r.FormatResult()
	want := "OK: CPU usage ok | 'cpu'=5;80;90\ndetail line 1\ndetail line 2"
	if got != want {
		t.Errorf("FormatResult got %q, want %q (long output must follow the first line)", got, want)
	}
}

// TestFormatResult_LongOutputSanitized pins the long-output sanitization:
// '\r' is stripped, a '|' followed by '=' on a long-output line is stripped
// (Icinga 2's ParseCheckOutput would parse it as a performance data token)
// and trailing line breaks are trimmed. Interior '\n' line breaks are
// preserved.
func TestFormatResult_LongOutputSanitized(t *testing.T) {
	r := NewCheckResult()
	r.SetResult(OK, "check")
	r.LongOutput = "line 1|injected=1\r\nline 2\n\n"

	got := r.FormatResult()
	want := "OK: check\nline 1injected=1\nline 2"
	if got != want {
		t.Errorf("FormatResult got %q, want %q (long output must strip '|', '\\r' and trailing '\\n')", got, want)
	}
}

// TestFormatResult_EmptyLongOutputOmitted ensures an empty LongOutput does not
// add a dangling newline to the output.
func TestFormatResult_EmptyLongOutputOmitted(t *testing.T) {
	r := NewCheckResult()
	r.SetResult(OK, "check")
	r.LongOutput = ""

	if got := r.FormatResult(); got != "OK: check" {
		t.Errorf("FormatResult got %q, want %q (empty long output must be omitted)", got, "OK: check")
	}
}

// TestFormatResult_UnsetThresholds pins the optional-threshold behavior
// matching Icinga 2's PerfdataValue::Format: unset (nil) thresholds are
// omitted, so a zero-value metric renders only its value and never emits
// misleading "0.00" thresholds.
func TestFormatResult_UnsetThresholds(t *testing.T) {
	testCases := []struct {
		name   string
		metric PerformanceMetric
		want   string
	}{
		{
			name:   "zero-value metric",
			metric: PerformanceMetric{Value: 5},
			want:   "'m'=5",
		},
		{
			name:   "warn and crit only",
			metric: PerformanceMetric{Value: 5, Warn: new(80), Crit: new(90)},
			want:   "'m'=5;80;90",
		},
		{
			// Unset warn keeps its position: Icinga 2 emits an empty
			// field between set fields (";;crit").
			name:   "crit with unset warn",
			metric: PerformanceMetric{Value: 5, Crit: new(90)},
			want:   "'m'=5;;90",
		},
		{
			name:   "min and max only",
			metric: PerformanceMetric{Value: 5, Min: new(0), Max: new(10)},
			want:   "'m'=5;;;0;10",
		},
		{
			name:   "warn only",
			metric: PerformanceMetric{Value: 5, Warn: new(80)},
			want:   "'m'=5;80",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := NewCheckResult()
			r.SetResult(OK, "check")
			r.AddPerformanceData("m", tc.metric)

			if got := r.FormatResult(); !strings.Contains(got, tc.want) {
				t.Errorf("FormatResult %q does not contain expected perfdata %q", got, tc.want)
			}
		})
	}
}

// TestFormatResult_NumberFormat pins the numeric rendering of normal output:
// whole numbers render without a decimal point and fractional numbers render
// in the shortest decimal form that round-trips, so Icinga's
// PerfdataValue::Parse reads back exactly the value the plugin set.
func TestFormatResult_NumberFormat(t *testing.T) {
	testCases := []struct {
		name  string
		value float64
		want  string
	}{
		{name: "whole number", value: 95, want: "'m'=95"},
		{name: "fractional", value: 1.23, want: "'m'=1.23"},
		{name: "small fractional beyond 2 decimals", value: 0.125, want: "'m'=0.125"},
		{name: "negative whole", value: -3, want: "'m'=-3"},
		{name: "negative fractional", value: -1.5, want: "'m'=-1.5"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := NewCheckResult()
			r.SetResult(OK, "check")
			r.AddPerformanceData("m", PerformanceMetric{Value: tc.value})

			if got := r.FormatResult(); !strings.Contains(got, tc.want) {
				t.Errorf("FormatResult %q does not contain expected perfdata %q", got, tc.want)
			}
		})
	}
}

func containsString(s, substr string) bool {
	return strings.Contains(s, substr)
}
