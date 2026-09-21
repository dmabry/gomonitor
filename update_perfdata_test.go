package gomonitor

import (
	"strings"
	"testing"
)

// TestUpdatePerformanceData_NewMetricOnFreshResult verifies that calling
// UpdatePerformanceData with a new metric name on a fresh CheckResult does not
// panic and registers the metric so it appears in FormatResult output.
// Regression: UpdatePerformanceData wrote directly to the nil map (panic on a
// zero-value CheckResult) and never registered new names in PerfOrder, so
// metrics added via Update were invisible in output.
func TestUpdatePerformanceData_NewMetricOnFreshResult(t *testing.T) {
	r := NewCheckResult()
	r.UpdatePerformanceData("disk", PerformanceMetric{Value: 42, Warn: 80, Crit: 90, Min: 0, Max: 100})

	if len(r.PerformanceData) != 1 {
		t.Errorf("PerformanceData has %d entries, want 1", len(r.PerformanceData))
	}
	if len(r.PerfOrder) != 1 || r.PerfOrder[0] != "disk" {
		t.Errorf("PerfOrder = %v, want [disk]", r.PerfOrder)
	}

	got := r.FormatResult()
	if !strings.Contains(got, "'disk'=42.00") {
		t.Errorf("FormatResult %q does not contain the updated metric", got)
	}
}

// TestUpdatePerformanceData_NilMapNoPanic verifies UpdatePerformanceData on a
// zero-value CheckResult (nil map) initializes state instead of panicking.
func TestUpdatePerformanceData_NilMapNoPanic(t *testing.T) {
	r := &CheckResult{}

	r.UpdatePerformanceData("mem", PerformanceMetric{Value: 55})

	if len(r.PerformanceData) != 1 {
		t.Errorf("PerformanceData has %d entries, want 1", len(r.PerformanceData))
	}
	if len(r.PerfOrder) != 1 || r.PerfOrder[0] != "mem" {
		t.Errorf("PerfOrder = %v, want [mem]", r.PerfOrder)
	}
	if got := r.FormatResult(); !strings.Contains(got, "'mem'=55.00") {
		t.Errorf("FormatResult %q does not contain the metric", got)
	}
}

// TestUpdatePerformanceData_ExistingMetricKeepsOrder verifies that updating an
// existing metric replaces its value in place and preserves its position in
// PerfOrder (no duplicate entries).
func TestUpdatePerformanceData_ExistingMetricKeepsOrder(t *testing.T) {
	r := NewCheckResult()
	r.AddPerformanceData("cpu", PerformanceMetric{Value: 10})
	r.AddPerformanceData("disk", PerformanceMetric{Value: 20})

	r.UpdatePerformanceData("cpu", PerformanceMetric{Value: 33})

	if len(r.PerfOrder) != 2 {
		t.Fatalf("PerfOrder has %d entries, want 2 (no duplicates)", len(r.PerfOrder))
	}
	if r.PerfOrder[0] != "cpu" || r.PerfOrder[1] != "disk" {
		t.Errorf("PerfOrder = %v, want [cpu disk]", r.PerfOrder)
	}
	if r.PerformanceData["cpu"].Value != 33 {
		t.Errorf("cpu Value = %v, want 33 (update must replace, not keep old value)", r.PerformanceData["cpu"].Value)
	}

	got := r.FormatResult()
	if !strings.Contains(got, "'cpu'=33.00") {
		t.Errorf("FormatResult %q does not contain the updated value 33.00", got)
	}
	if strings.Contains(got, "'cpu'=10.00") {
		t.Errorf("FormatResult %q still contains the stale value 10.00", got)
	}
}

// TestUpdatePerformanceData_MatchesAddPerformanceData verifies the documented
// guarantee: for the same sequence of calls, Update and Add produce identical
// results.
func TestUpdatePerformanceData_MatchesAddPerformanceData(t *testing.T) {
	add := NewCheckResult()
	upd := NewCheckResult()

	for _, m := range []struct {
		name string
		met  PerformanceMetric
	}{
		{"cpu", PerformanceMetric{Value: 1, Warn: 2, Crit: 3, Min: 0, Max: 4, UnitOM: "%"}},
		{"disk", PerformanceMetric{Value: 5}},
		{"cpu", PerformanceMetric{Value: 9}},
	} {
		add.AddPerformanceData(m.name, m.met)
		upd.UpdatePerformanceData(m.name, m.met)
	}

	if got, want := upd.FormatResult(), add.FormatResult(); got != want {
		t.Errorf("UpdatePerformanceData result %q != AddPerformanceData result %q", got, want)
	}
}
