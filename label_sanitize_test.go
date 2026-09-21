package gomonitor

import (
	"strings"
	"testing"
)

// TestFormatResult_LabelWithoutEquals verifies the Nagios plugin guideline
// that perfdata labels must not contain '=' ("Label can contain any characters
// except equals sign or single quote"). An '=' inside the quoted label shifts
// the value boundary for strict parsers. sanitizePerfToken strips it.
func TestFormatResult_LabelWithoutEquals(t *testing.T) {
	r := NewCheckResult()
	r.SetResult(OK, "check")
	r.AddPerformanceData("bad=label", PerformanceMetric{Value: 1.0, Warn: 2.0, Crit: 3.0, Min: 0.0, Max: 10.0})

	got := r.FormatResult()

	// The '=' is stripped: label becomes 'badlabel'
	if strings.Contains(got, "'bad=label'") {
		t.Errorf("FormatResult %q keeps '=' inside the label (forbidden by Nagios guidelines)", got)
	}
	if !strings.Contains(got, "'badlabel'=1.00") {
		t.Errorf("FormatResult %q does not contain the sanitized label 'badlabel'", got)
	}
}

// TestFormatResult_LabelAndUnitEquals covers '=' stripping in both sanitized
// token positions: the metric label and the unit of measure.
func TestFormatResult_LabelAndUnitEquals(t *testing.T) {
	r := NewCheckResult()
	r.SetResult(OK, "check")
	r.AddPerformanceData("temp=reading", PerformanceMetric{
		Value:  20.5,
		Warn:   25.0,
		Crit:   30.0,
		Min:    0.0,
		Max:    100.0,
		UnitOM: "C=injected",
	})

	got := r.FormatResult()

	if strings.Contains(got, "temp=reading") || strings.Contains(got, "C=injected") {
		t.Errorf("FormatResult %q keeps '=' inside a perfdata token", got)
	}
	if !strings.Contains(got, "'tempreading'=20.50Cinjected;25.00;30.00;0.00;100.00") {
		t.Errorf("FormatResult %q does not contain the sanitized label and UOM", got)
	}
	// Exactly one '=' per token, separating label from value
	perf := strings.SplitN(got, "|", 2)[1]
	for _, tok := range strings.Fields(strings.TrimSpace(perf)) {
		eq := strings.Index(tok, "=")
		if eq < 0 {
			t.Errorf("token %q has no '=' separator", tok)
			continue
		}
		if strings.Contains(tok[eq+1:], "=") {
			t.Errorf("token %q has '=' inside the value/UOM portion", tok)
		}
	}
}
