package gomonitor

import (
	"strings"
	"testing"
)

// TestDeletePerformanceData_LastMetricKeepsOrder pins the delete bookkeeping:
// deleting the last-added metric removes it from both PerformanceData and
// PerfOrder, keeping the invariant len(PerformanceData) == len(PerfOrder).
func TestDeletePerformanceData_LastMetricKeepsOrder(t *testing.T) {
	r := NewCheckResult()
	r.AddPerformanceData("cpu", PerformanceMetric{Value: 1})
	r.AddPerformanceData("mem", PerformanceMetric{Value: 2})
	r.AddPerformanceData("disk", PerformanceMetric{Value: 3})

	r.DeletePerformanceData("disk")

	if len(r.PerformanceData) != len(r.PerfOrder) {
		t.Errorf("PerformanceData len %d != PerfOrder len %d (invariant broken)", len(r.PerformanceData), len(r.PerfOrder))
	}
	wantOrder := []string{"cpu", "mem"}
	for i, name := range wantOrder {
		if r.PerfOrder[i] != name {
			t.Errorf("PerfOrder[%d] = %q, want %q", i, r.PerfOrder[i], name)
		}
		if _, ok := r.PerformanceData[name]; !ok {
			t.Errorf("PerformanceData missing %q after delete", name)
		}
	}
}

// TestDeletePerformanceData_MiddleMetricKeepsOrder ensures a middle delete
// still swaps the last element into the deleted slot and keeps the remaining
// metrics in their original relative order.
func TestDeletePerformanceData_MiddleMetricKeepsOrder(t *testing.T) {
	r := NewCheckResult()
	r.AddPerformanceData("cpu", PerformanceMetric{Value: 1})
	r.AddPerformanceData("mem", PerformanceMetric{Value: 2})
	r.AddPerformanceData("disk", PerformanceMetric{Value: 3})

	r.DeletePerformanceData("mem")

	if len(r.PerformanceData) != len(r.PerfOrder) {
		t.Errorf("PerformanceData len %d != PerfOrder len %d", len(r.PerformanceData), len(r.PerfOrder))
	}
	wantOrder := []string{"cpu", "disk"}
	for i, name := range wantOrder {
		if r.PerfOrder[i] != name {
			t.Errorf("PerfOrder[%d] = %q, want %q", i, r.PerfOrder[i], name)
		}
	}
	if _, ok := r.PerformanceData["mem"]; ok {
		t.Errorf("deleted metric 'mem' still present in PerformanceData: %v", r.PerformanceData)
	}
}

// TestDeletePerformanceData_TruncatedPerfOrderNoPanic pins the crash fix:
// PerfOrder is an exported field, so callers can modify it. DeletePerformanceData
// used to index len(PerfOrder)-1 and panic ("index out of range [-1]") when the
// metric was present in PerformanceData but missing from PerfOrder.
func TestDeletePerformanceData_TruncatedPerfOrderNoPanic(t *testing.T) {
	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("DeletePerformanceData panicked on externally truncated PerfOrder: %v", rec)
		}
	}()

	r := NewCheckResult()
	r.AddPerformanceData("cpu", PerformanceMetric{Value: 1})
	r.PerfOrder = []string{} // external modification via the exported field

	r.DeletePerformanceData("cpu") // must not panic; metric is dropped from the map

	if _, ok := r.PerformanceData["cpu"]; ok {
		t.Error("metric should be deleted from PerformanceData even when PerfOrder was truncated")
	}
}

// TestDeletePerformanceData_PerfOrderShorterThanMap covers the other
// external-modification shape: PerfOrder contains other names but not the
// deleted one.
func TestDeletePerformanceData_PerfOrderShorterThanMap(t *testing.T) {
	r := NewCheckResult()
	r.AddPerformanceData("cpu", PerformanceMetric{Value: 1})
	r.AddPerformanceData("mem", PerformanceMetric{Value: 2})
	r.PerfOrder = []string{"mem"} // 'cpu' missing from the exported order slice

	r.DeletePerformanceData("cpu") // must not panic or disturb 'mem'

	if _, ok := r.PerformanceData["cpu"]; ok {
		t.Error("metric 'cpu' should be deleted from PerformanceData")
	}
	if r.PerfOrder[0] != "mem" {
		t.Errorf("PerfOrder[0] = %q, want 'mem' (untouched)", r.PerfOrder[0])
	}
}

// TestFormatResult_SingleVerbTemplateGetsMessage pins the fix for the
// silent-drop footgun: a single-verb Format ("%s") used to substitute the
// status and discard the message entirely. A single-verb template has only
// one slot, so it receives the message — the diagnostic payload — and the
// status remains conveyed by the exit code (no status prefix can be
// rendered: the template provides no literal between two verbs).
func TestFormatResult_SingleVerbTemplateGetsMessage(t *testing.T) {
	r := NewCheckResult()
	r.Format = "%s"
	r.SetResult(OK, "the message you expected to see")

	got := r.FormatResult()
	want := "the message you expected to see"
	if got != want {
		t.Errorf("FormatResult with single-verb Format got %q, want %q (message must not be dropped)", got, want)
	}
}

// TestFormatResult_MultiVerbTemplateBehavior documents and pins the >2-verb
// behavior: the first verb is the status, every subsequent verb receives the
// message.
func TestFormatResult_MultiVerbTemplateBehavior(t *testing.T) {
	r := NewCheckResult()
	r.Format = "%s - %s / %s"
	r.SetResult(Warning, "high latency")

	got := r.FormatResult()
	want := "Warning - high latency / high latency"
	if got != want {
		t.Errorf("FormatResult with 3-verb Format got %q, want %q", got, want)
	}
}

// TestFormatResult_NoVerbTemplate pins the no-verb passthrough that already
// existed: a template without %s is returned as-is.
func TestFormatResult_NoVerbTemplate(t *testing.T) {
	r := NewCheckResult()
	r.Format = "static status line"
	r.SetResult(OK, "ignored message")

	if got := r.FormatResult(); got != "static status line" {
		t.Errorf("FormatResult with no-verb Format got %q, want %q", got, "static status line")
	}
}

// TestZeroValueCheckResultStatusPrefix documents the zero-value behavior: a
// struct literal has StatusPrefix false, so no status prefix is prepended.
// NewCheckResult sets it to true.
func TestZeroValueCheckResultStatusPrefix(t *testing.T) {
	r := &CheckResult{Message: "hello from a struct literal"}

	if got := r.FormatResult(); got != "hello from a struct literal" {
		t.Errorf("zero-value CheckResult got %q, want %q (StatusPrefix is false on zero value)", got, "hello from a struct literal")
	}

	r.StatusPrefix = true
	r.Format = "%s: %s"
	if got := r.FormatResult(); got != "OK: hello from a struct literal" {
		t.Errorf("zero-value CheckResult with StatusPrefix=true got %q, want %q", got, "OK: hello from a struct literal")
	}
}

// TestAddPerformanceData_PartialInitializationNoPanic pins the crash fix:
// PerformanceData, PerfOrder, and StatusPrefix are exported, so callers can
// build a CheckResult by hand with only some fields initialized. Add used to
// skip its init block whenever PerformanceData was non-nil and then write to
// a nil PerfOrder slice or assign into a nil internal map, panicking with
// "assignment to entry in nil map". Every partially-initialized shape must
// add the metric without panicking.
func TestAddPerformanceData_PartialInitializationNoPanic(t *testing.T) {
	testCases := []struct {
		name   string
		result *CheckResult
	}{
		{"map only", &CheckResult{PerformanceData: make(map[string]PerformanceMetric)}},
		{"order only", &CheckResult{PerfOrder: []string{}}},
		{"zero value", &CheckResult{}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if rec := recover(); rec != nil {
					t.Fatalf("AddPerformanceData panicked on %s: %v", tc.name, rec)
				}
			}()

			tc.result.AddPerformanceData("cpu", PerformanceMetric{Value: 1})

			if len(tc.result.PerformanceData) != 1 {
				t.Errorf("PerformanceData has %d entries, want 1", len(tc.result.PerformanceData))
			}
			if len(tc.result.PerfOrder) != 1 || tc.result.PerfOrder[0] != "cpu" {
				t.Errorf("PerfOrder = %v, want [cpu]", tc.result.PerfOrder)
			}
			if got := tc.result.FormatResult(); !strings.Contains(got, "'cpu'=1.00") {
				t.Errorf("FormatResult %q does not contain the added metric", got)
			}
		})
	}
}

// TestFormatResult_SkipsStalePerfOrderEntries pins the ghost-metric fix:
// PerfOrder is an exported field, so it can contain names that are no longer
// (or never were) in PerformanceData. FormatResult used to index the map
// directly and rendered such stale entries as zero-value metrics
// ('ghost'=0.00;...). Stale names must be skipped, and the remaining metrics
// must keep their order.
func TestFormatResult_SkipsStalePerfOrderEntries(t *testing.T) {
	r := NewCheckResult()
	r.SetResult(OK, "check")
	r.AddPerformanceData("cpu", PerformanceMetric{Value: 1})
	r.AddPerformanceData("mem", PerformanceMetric{Value: 2})
	r.PerfOrder = append(r.PerfOrder, "ghost") // external modification: stale name

	got := r.FormatResult()

	if strings.Contains(got, "ghost") {
		t.Errorf("FormatResult %q renders stale PerfOrder entry 'ghost' as a zero-value metric", got)
	}
	if !strings.Contains(got, "'cpu'=1.00") || !strings.Contains(got, "'mem'=2.00") {
		t.Errorf("FormatResult %q does not contain the real metrics", got)
	}
	if cpuIdx, memIdx := strings.Index(got, "'cpu'"), strings.Index(got, "'mem'"); cpuIdx > memIdx {
		t.Errorf("FormatResult %q lost metric order after skipping stale entry", got)
	}
}

// TestDeleteThenReAddKeepsConsistency exercises a delete/re-add cycle across
// all positions, asserting the map/order invariant every time: PerfOrder and
// PerformanceData have equal lengths, every ordered name exists in the map,
// and PerfOrder contains no duplicates.
func TestDeleteThenReAddKeepsConsistency(t *testing.T) {
	assertConsistent := func(r *CheckResult, stage string) {
		t.Helper()
		if len(r.PerformanceData) != len(r.PerfOrder) {
			t.Fatalf("%s: PerformanceData len %d != PerfOrder len %d", stage, len(r.PerformanceData), len(r.PerfOrder))
		}
		seen := make(map[string]int)
		for i, name := range r.PerfOrder {
			if _, ok := r.PerformanceData[name]; !ok {
				t.Fatalf("%s: PerfOrder[%d] = %q is missing from PerformanceData", stage, i, name)
			}
			if prev, dup := seen[name]; dup {
				t.Fatalf("%s: PerfOrder contains duplicate %q at %d and %d", stage, name, prev, i)
			}
			seen[name] = i
		}
	}

	r := NewCheckResult()
	for _, name := range []string{"a", "b", "c", "d", "e"} {
		r.AddPerformanceData(name, PerformanceMetric{Value: 1})
	}

	r.DeletePerformanceData("e") // last
	assertConsistent(r, "after delete last")
	r.DeletePerformanceData("a") // first
	assertConsistent(r, "after delete first")
	r.DeletePerformanceData("c") // middle of [b c d]
	assertConsistent(r, "after delete middle")

	r.AddPerformanceData("f", PerformanceMetric{Value: 2})
	r.DeletePerformanceData("b") // middle again
	assertConsistent(r, "after re-add and delete")
	r.DeletePerformanceData("f")
	r.DeletePerformanceData("d") // down to empty
	assertConsistent(r, "after delete to empty")

	if len(r.PerfOrder) != 0 || len(r.PerformanceData) != 0 {
		t.Errorf("expected empty order/map, got PerfOrder=%v PerformanceData=%v", r.PerfOrder, r.PerformanceData)
	}
}
