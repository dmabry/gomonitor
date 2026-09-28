package gomonitor

import (
	"strings"
	"testing"
)

// TestFormatResult_LabelWithoutEquals verifies that perfdata labels never
// contain '=': Icinga's SplitPerfdata ends the label at the first '=', so an
// '=' inside the quoted label shifts the value boundary. sanitizeLabel strips
// it.
func TestFormatResult_LabelWithoutEquals(t *testing.T) {
	r := NewCheckResult()
	r.SetResult(OK, "check")
	r.AddPerformanceData("bad=label", PerformanceMetric{Value: 1.0, Warn: new(2.0), Crit: new(3.0), Min: new(0.0), Max: new(10.0)})

	got := r.FormatResult()

	// The '=' is stripped: label becomes 'badlabel'
	if strings.Contains(got, "'bad=label'") {
		t.Errorf("FormatResult %q keeps '=' inside the label (Icinga ends the label at the first '=')", got)
	}
	if !strings.Contains(got, "'badlabel'=1") {
		t.Errorf("FormatResult %q does not contain the sanitized label 'badlabel'", got)
	}
}

// TestFormatResult_LabelAndUnitEquals covers '=' in both token positions: it
// is stripped from the metric label, and a unit containing it cannot
// round-trip through Icinga's parser, so the unit is dropped.
func TestFormatResult_LabelAndUnitEquals(t *testing.T) {
	r := NewCheckResult()
	r.SetResult(OK, "check")
	r.AddPerformanceData("temp=reading", PerformanceMetric{
		Value:  20.5,
		Warn:   new(25.0),
		Crit:   new(30.0),
		Min:    new(0.0),
		Max:    new(100.0),
		UnitOM: "C=injected",
	})

	got := r.FormatResult()

	if strings.Contains(got, "temp=reading") || strings.Contains(got, "C=injected") {
		t.Errorf("FormatResult %q keeps '=' inside a perfdata token", got)
	}
	if !strings.Contains(got, "'tempreading'=20.5;25;30;0;100") {
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
