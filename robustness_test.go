package gomonitor

import (
	"testing"
)

// TestDeletePerformanceData_LastMetricNoGhostEntry pins the fix for the stale
// index-map entry: deleting the last-added metric used to re-insert the
// deleted key into perfIndexMap, breaking the invariant
// len(perfIndexMap) == len(PerfOrder).
func TestDeletePerformanceData_LastMetricNoGhostEntry(t *testing.T) {
	r := NewCheckResult()
	r.AddPerformanceData("cpu", PerformanceMetric{Value: 1})
	r.AddPerformanceData("mem", PerformanceMetric{Value: 2})
	r.AddPerformanceData("disk", PerformanceMetric{Value: 3})

	r.DeletePerformanceData("disk")

	if _, ghost := r.perfIndexMap["disk"]; ghost {
		t.Errorf("deleted metric 'disk' still present in perfIndexMap: %v", r.perfIndexMap)
	}
	if len(r.perfIndexMap) != len(r.PerfOrder) {
		t.Errorf("perfIndexMap len %d != PerfOrder len %d (invariant broken)", len(r.perfIndexMap), len(r.PerfOrder))
	}
	wantOrder := []string{"cpu", "mem"}
	for i, name := range wantOrder {
		if r.PerfOrder[i] != name {
			t.Errorf("PerfOrder[%d] = %q, want %q", i, r.PerfOrder[i], name)
		}
		if idx, ok := r.perfIndexMap[name]; !ok || idx != i {
			t.Errorf("perfIndexMap[%q] = %d, want %d", name, idx, i)
		}
	}
}

// TestDeletePerformanceData_MiddleMetricKeepsOrder ensures a middle delete
// still swaps the last element into the deleted slot and reindexes it.
func TestDeletePerformanceData_MiddleMetricKeepsOrder(t *testing.T) {
	r := NewCheckResult()
	r.AddPerformanceData("cpu", PerformanceMetric{Value: 1})
	r.AddPerformanceData("mem", PerformanceMetric{Value: 2})
	r.AddPerformanceData("disk", PerformanceMetric{Value: 3})

	r.DeletePerformanceData("mem")

	if len(r.perfIndexMap) != len(r.PerfOrder) {
		t.Errorf("perfIndexMap len %d != PerfOrder len %d", len(r.perfIndexMap), len(r.PerfOrder))
	}
	wantOrder := []string{"cpu", "disk"}
	for i, name := range wantOrder {
		if r.PerfOrder[i] != name {
			t.Errorf("PerfOrder[%d] = %q, want %q", i, r.PerfOrder[i], name)
		}
	}
	if idx := r.perfIndexMap["disk"]; idx != 1 {
		t.Errorf("perfIndexMap[\"disk\"] = %d, want 1 (reindexed after swap)", idx)
	}
	if _, ok := r.perfIndexMap["mem"]; ok {
		t.Errorf("deleted metric 'mem' still present in perfIndexMap: %v", r.perfIndexMap)
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

// TestDeleteThenReAddKeepsConsistency exercises a delete/re-add cycle across
// all positions, asserting the map/order/index invariant every time.
func TestDeleteThenReAddKeepsConsistency(t *testing.T) {
	assertConsistent := func(r *CheckResult, stage string) {
		t.Helper()
		if len(r.perfIndexMap) != len(r.PerfOrder) {
			t.Fatalf("%s: perfIndexMap len %d != PerfOrder len %d", stage, len(r.perfIndexMap), len(r.PerfOrder))
		}
		for i, name := range r.PerfOrder {
			if idx, ok := r.perfIndexMap[name]; !ok || idx != i {
				t.Fatalf("%s: perfIndexMap[%q] = %d, want %d", stage, name, idx, i)
			}
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

	if len(r.PerfOrder) != 0 || len(r.perfIndexMap) != 0 {
		t.Errorf("expected empty order/index, got PerfOrder=%v perfIndexMap=%v", r.PerfOrder, r.perfIndexMap)
	}
}
