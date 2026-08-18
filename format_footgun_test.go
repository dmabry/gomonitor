package gomonitor

import (
	"strings"
	"testing"
)

// TestFormatResult_PercentInMessage ensures messages that contain literal '%'
// characters (e.g. "CPU 95% usage") pass through unchanged. The message is an
// argument to the Format template, so stray percents in it must not be
// interpreted as Sprintf verbs.
func TestFormatResult_PercentInMessage(t *testing.T) {
	testCases := []struct {
		name    string
		message string
	}{
		{name: "single percent", message: "CPU usage is 95%"},
		{name: "trailing percent", message: "Load average is 99%"},
		{name: "multiple percents", message: "Disk 90% full, mem 80%"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := NewCheckResult()
			r.SetResult(OK, tc.message)

			got := r.FormatResult()
			want := "OK: " + tc.message
			if got != want {
				t.Errorf("FormatResult got %q, want %q (message must pass through unchanged)", got, want)
			}
		})
	}
}

// TestFormatResult_PercentInMessageWithPerfData is the same scenario but with
// performance data attached, exercising the message + perfdata code path.
func TestFormatResult_PercentInMessageWithPerfData(t *testing.T) {
	r := NewCheckResult()
	r.SetResult(OK, "CPU usage is 95%")
	r.AddPerformanceData("cpu", PerformanceMetric{Value: 95.0, Warn: 80.0, Crit: 90.0, Min: 0.0, Max: 100.0})

	got := r.FormatResult()
	for _, want := range []string{"CPU usage is 95%", "'cpu'=95.00"} {
		if !strings.Contains(got, want) {
			t.Errorf("FormatResult %q does not contain %q", got, want)
		}
	}
	if strings.Contains(got, "%!") {
		t.Errorf("FormatResult %q contains Sprintf error placeholder %q", got, "%!")
	}
}

// TestFormatResult_CustomFormatEscapedPercent ensures the old "%%" escape
// hatch still works for backward compatibility with the Sprintf-based
// implementation: "%%" in a custom Format collapses to a single "%".
func TestFormatResult_CustomFormatEscapedPercent(t *testing.T) {
	r := NewCheckResult()
	r.Format = "[%s] %s (95%%)"
	r.SetResult(Warning, "High latency")

	if got := r.FormatResult(); got != "[Warning] High latency (95%)" {
		t.Errorf("FormatResult got %q, want %q", got, "[Warning] High latency (95%)")
	}
}

// TestFormatResult_PercentEscapeInMessage ensures "%%" inside the message
// argument is preserved verbatim — only the template collapses "%%", never
// the substituted values.
func TestFormatResult_PercentEscapeInMessage(t *testing.T) {
	r := NewCheckResult()
	r.SetResult(OK, "100%% sure")

	if got := r.FormatResult(); got != "OK: 100%% sure" {
		t.Errorf("FormatResult got %q, want %q", got, "OK: 100%% sure")
	}
}

// TestFormatResult_CustomFormatLiteralPercent reproduces the footgun: a
// literal (unescaped) "%" in a custom Format must not produce Sprintf
// placeholder garbage such as "%!s(MISSING)".
func TestFormatResult_CustomFormatLiteralPercent(t *testing.T) {
	r := NewCheckResult()
	r.Format = "[%s] %s (95% sure)"
	r.SetResult(Warning, "High latency")

	got := r.FormatResult()
	want := "[Warning] High latency (95% sure)"
	if got != want {
		t.Errorf("FormatResult got %q, want %q", got, want)
	}
	if strings.Contains(got, "%!") {
		t.Errorf("FormatResult %q contains Sprintf error placeholder %q", got, "%!")
	}
}
