package gomonitor

import (
	"strings"
	"testing"
)

// TestFormatResult_NormalizedUnits pins the Icinga 2 unit-of-measure
// normalization (PerfdataValue::Parse followed by PerfdataValue::Format):
// values and thresholds scale by the unit's factor and the canonical short
// unit renders, mirroring l_CsUoMs, l_CiUoMs and l_FormatUoMs.
func TestFormatResult_NormalizedUnits(t *testing.T) {
	testCases := []struct {
		name   string
		metric PerformanceMetric
		want   string
	}{
		{name: "milliseconds to seconds", metric: PerformanceMetric{Value: 12.445, UnitOM: "ms"}, want: "'rta'=0.012445s"},
		{name: "percent", metric: PerformanceMetric{Value: 95, UnitOM: "%"}, want: "'cpu'=95%"},
		{name: "kiB scales by 1024", metric: PerformanceMetric{Value: 2, UnitOM: "kiB"}, want: "'disk'=2048B"},
		{name: "kib is bits", metric: PerformanceMetric{Value: 2, UnitOM: "kib"}, want: "'disk'=2048b"},
		{name: "kb scales by 1000", metric: PerformanceMetric{Value: 1, UnitOM: "kb"}, want: "'net'=1000b"},
		{name: "mb uses power 2", metric: PerformanceMetric{Value: 1, UnitOM: "mb"}, want: "'net'=1000000b"},
		{name: "counter unit", metric: PerformanceMetric{Value: 42, UnitOM: "c"}, want: "'count'=42c"},
		{name: "celsius", metric: PerformanceMetric{Value: 20.5, UnitOM: "C"}, want: "'temp'=20.500000C"},
		{name: "fahrenheit", metric: PerformanceMetric{Value: 98.6, UnitOM: "f"}, want: "'temp'=98.600000F"},
		{name: "kelvin", metric: PerformanceMetric{Value: 300, UnitOM: "k"}, want: "'temp'=300K"},
		{name: "kilograms with scaled thresholds", metric: PerformanceMetric{Value: 1, Warn: new(2), Crit: new(3), UnitOM: "kg"}, want: "'mass'=1000g;2000;3000"},
		{name: "watt-hour", metric: PerformanceMetric{Value: 1, UnitOM: "wh"}, want: "'energy'=1Wh"},
		{name: "kilowatt-hour", metric: PerformanceMetric{Value: 1, UnitOM: "kwh"}, want: "'energy'=1000Wh"},
		{name: "milliampere-hour", metric: PerformanceMetric{Value: 1, UnitOM: "mah"}, want: "'charge'=3.600000As"},
		{name: "days to seconds", metric: PerformanceMetric{Value: 2, UnitOM: "d"}, want: "'uptime'=172800s"},
		{name: "mega-ampere", metric: PerformanceMetric{Value: 1, UnitOM: "MA"}, want: "'current'=1000000A"},
		{name: "nanoseconds", metric: PerformanceMetric{Value: 1, UnitOM: "ns"}, want: "'lat'=0.000000s"},
		{name: "unknown unit dropped", metric: PerformanceMetric{Value: 5, UnitOM: "banana"}, want: "'weird'=5"},
		{name: "zero-value metric", metric: PerformanceMetric{}, want: "'zero'=0"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := NewCheckResult()
			r.NormalizeUnits = true
			r.SetResult(OK, "check")

			var name string
			switch {
			case strings.Contains(tc.want, "'rta'"):
				name = "rta"
			case strings.Contains(tc.want, "'cpu'"):
				name = "cpu"
			case strings.Contains(tc.want, "'disk'"):
				name = "disk"
			case strings.Contains(tc.want, "'net'"):
				name = "net"
			case strings.Contains(tc.want, "'count'"):
				name = "count"
			case strings.Contains(tc.want, "'temp'"):
				name = "temp"
			case strings.Contains(tc.want, "'mass'"):
				name = "mass"
			case strings.Contains(tc.want, "'energy'"):
				name = "energy"
			case strings.Contains(tc.want, "'charge'"):
				name = "charge"
			case strings.Contains(tc.want, "'uptime'"):
				name = "uptime"
			case strings.Contains(tc.want, "'current'"):
				name = "current"
			case strings.Contains(tc.want, "'lat'"):
				name = "lat"
			case strings.Contains(tc.want, "'weird'"):
				name = "weird"
			default:
				name = "zero"
			}
			r.AddPerformanceData(name, tc.metric)

			if got := r.FormatResult(); !strings.Contains(got, tc.want) {
				t.Errorf("FormatResult %q does not contain normalized perfdata %q", got, tc.want)
			}
		})
	}
}

// TestFormatResult_NormalizedUnitsOff pins the default: without
// NormalizeUnits, the unit passes through verbatim and values are not
// scaled, matching the plugin-output convention Icinga receives.
func TestFormatResult_NormalizedUnitsOff(t *testing.T) {
	r := NewCheckResult()
	r.SetResult(OK, "check")
	r.AddPerformanceData("rta", PerformanceMetric{Value: 12.445, UnitOM: "ms"})
	r.AddPerformanceData("disk", PerformanceMetric{Value: 2, UnitOM: "kib", Warn: new(1)})

	got := r.FormatResult()

	for _, want := range []string{"'rta'=12.445ms", "'disk'=2kib;1"} {
		if !strings.Contains(got, want) {
			t.Errorf("FormatResult %q does not contain unnormalized perfdata %q", got, want)
		}
	}
}

// TestLookupUoM pins the resolution order: case-sensitive table first, then
// the lowercased unit in the case-insensitive table, then unknown.
func TestLookupUoM(t *testing.T) {
	testCases := []struct {
		unit      string
		factor    float64
		counter   bool
		known     bool
		canonical string
	}{
		{unit: "ms", factor: 1.0 / 1000, known: true, canonical: "seconds"},
		{unit: "MS", factor: 1.0 / 1000, known: true, canonical: "seconds"},
		{unit: "%", factor: 1, known: true, canonical: "percent"},
		{unit: "c", factor: 1, counter: true, known: true, canonical: ""},
		{unit: "C", factor: 1, known: true, canonical: "degrees-celsius"},
		{unit: "kib", factor: 1024, known: true, canonical: "bits"},
		{unit: "Kib", factor: 1024, known: true, canonical: "bits"},
		{unit: "KiB", factor: 1024, known: true, canonical: "bytes"},
		{unit: "KWH", factor: 1000, known: true, canonical: "watt-hours"},
		{unit: "m", factor: 60, known: true, canonical: "seconds"},
		// Bare "M" is not in the case-sensitive table, so it falls back
		// through the lowercase to seconds x 60, matching Icinga.
		{unit: "M", factor: 60, known: true, canonical: "seconds"},
		{unit: "banana", factor: 1, known: false},
		{unit: "", factor: 1, known: true, canonical: ""},
	}

	for _, tc := range testCases {
		t.Run(tc.unit, func(t *testing.T) {
			factor, canonical, counter, known := lookupUoM(tc.unit)
			if factor != tc.factor {
				t.Errorf("lookupUoM(%q) factor = %v, want %v", tc.unit, factor, tc.factor)
			}
			if counter != tc.counter {
				t.Errorf("lookupUoM(%q) counter = %v, want %v", tc.unit, counter, tc.counter)
			}
			if known != tc.known {
				t.Errorf("lookupUoM(%q) known = %v, want %v", tc.unit, known, tc.known)
			}
			if canonical != tc.canonical {
				t.Errorf("lookupUoM(%q) canonical = %q, want %q", tc.unit, canonical, tc.canonical)
			}
		})
	}
}

// TestNormalized_ThresholdsStayNil pins that unset thresholds stay unset
// through normalization — only set thresholds are scaled, matching Icinga's
// non-empty warn/crit/min/max handling.
func TestNormalized_ThresholdsStayNil(t *testing.T) {
	m := PerformanceMetric{Value: 1, Warn: new(2), UnitOM: "kg"}

	out, unit := m.normalized()

	if unit != "g" {
		t.Errorf("normalized unit = %q, want %q", unit, "g")
	}
	if out.Value != 1000 {
		t.Errorf("normalized Value = %v, want 1000", out.Value)
	}
	if out.Warn == nil || *out.Warn != 2000 {
		t.Errorf("normalized Warn = %v, want 2000", out.Warn)
	}
	if out.Crit != nil || out.Min != nil || out.Max != nil {
		t.Errorf("normalized unset thresholds must stay nil, got Crit=%v Min=%v Max=%v", out.Crit, out.Min, out.Max)
	}
}
