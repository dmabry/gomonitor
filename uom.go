package gomonitor

import "math"

// This file mirrors the unit-of-measure handling of Icinga 2's
// PerfdataValue (lib/base/perfdatavalue.cpp): the UoM tables, the factor
// that values are scaled by when normalizing, the canonical short unit
// rendered in output, and the counter unit "c".

// uomFactor describes a unit of measure: the factor that values expressed
// in the unit are multiplied by when normalized, and the canonical unit
// name used to look up the short form rendered in output.
type uomFactor struct {
	factor float64
	out    string
}

// caseSensitiveUoMs mirrors Icinga 2's l_CsUoMs: units resolved
// case-sensitively before the case-insensitive table is consulted.
var caseSensitiveUoMs = buildCsUoMs()

func buildCsUoMs() map[string]uomFactor {
	uoms := map[string]uomFactor{
		"":  {1, ""},
		"%": {1, "percent"},
		"c": {1, ""},
		"C": {1, "degrees-celsius"},
		"b": {1, "bits"},
		"B": {1, "bytes"},
	}

	dataBases := map[byte]string{
		'b': "bits",
		'B': "bytes",
	}

	// Data (rate) units: SI prefixes k..Y with a 1000 factor, or IEC
	// "i"/"I" suffixes with a 1024 factor, applied to bits/bytes. The
	// prefix power mapping (m at power 2, not milli) matches Icinga 2.
	prefixes := []struct {
		char  string
		power float64
	}{
		{"k", 1}, {"K", 1},
		{"m", 2}, {"M", 2},
		{"g", 3}, {"G", 3},
		{"t", 4}, {"T", 4},
		{"p", 5}, {"P", 5},
		{"e", 6}, {"E", 6},
		{"z", 7}, {"Z", 7},
		{"y", 8}, {"Y", 8},
	}
	siIecs := []struct {
		char   string
		factor float64
	}{
		{"", 1000},
		{"i", 1024},
		{"I", 1024},
	}
	for _, prefix := range prefixes {
		for _, siIec := range siIecs {
			factor := math.Pow(siIec.factor, prefix.power)
			for base, out := range dataBases {
				uoms[prefix.char+siIec.char+string(base)] = uomFactor{factor, out}
			}
		}
	}

	// Energy units: amperes, ohms, volts and watts with SI prefixes
	// scaled by powers of 1000.
	energyPrefixes := []struct {
		char  string
		power float64
	}{
		{"n", -3}, {"N", -3},
		{"u", -2}, {"U", -2},
		{"m", -1},
		{"", 0},
		{"k", 1}, {"K", 1},
		{"M", 2},
		{"g", 3}, {"G", 3},
		{"t", 4}, {"T", 4},
		{"p", 5}, {"P", 5},
		{"e", 6}, {"E", 6},
		{"z", 7}, {"Z", 7},
		{"y", 8}, {"Y", 8},
	}
	energyBases := map[byte]string{
		'a': "amperes",
		'A': "amperes",
		'o': "ohms",
		'O': "ohms",
		'v': "volts",
		'V': "volts",
		'w': "watts",
		'W': "watts",
	}
	for _, prefix := range energyPrefixes {
		factor := math.Pow(1000, prefix.power)
		for base, out := range energyBases {
			uoms[prefix.char+string(base)] = uomFactor{factor, out}
		}
	}

	// Charge and energy units with time suffixes: ampere-seconds
	// (a/A) and watt-hours (w/W) scaled by the prefix and the time
	// suffix, with watt-hours divided by 3600 to keep the base unit.
	timeSuffixes := []struct {
		char   string
		factor float64
	}{
		{"s", 1}, {"S", 1},
		{"m", 60}, {"M", 60},
		{"h", 60 * 60}, {"H", 60 * 60},
	}
	chargeBases := map[byte]uomFactor{
		'a': {1, "ampere-seconds"},
		'A': {1, "ampere-seconds"},
		'w': {60 * 60, "watt-hours"},
		'W': {60 * 60, "watt-hours"},
	}
	for _, prefix := range energyPrefixes {
		prefixFactor := math.Pow(1000, prefix.power)
		for base, chargeBase := range chargeBases {
			for _, suffix := range timeSuffixes {
				uoms[prefix.char+string(base)+suffix.char] = uomFactor{
					prefixFactor * suffix.factor / chargeBase.factor,
					chargeBase.out,
				}
			}
		}
	}

	return uoms
}

// caseInsensitiveUoMs mirrors Icinga 2's l_CiUoMs: time, mass, volume and
// misc units resolved from the lowercased unit.
//
// The nano factors are written as literals: Icinga evaluates
// 1.0 / 1000 / 1000 / 1000 step by step in double precision, which gives
// 9.999999999999999e-10, while the same Go constant expression is evaluated
// exactly and gives 1e-09.
var caseInsensitiveUoMs = map[string]uomFactor{
	// Time:
	"ns": {9.999999999999999e-10, "seconds"},
	"us": {1.0 / 1000 / 1000, "seconds"},
	"ms": {1.0 / 1000, "seconds"},
	"s":  {1, "seconds"},
	"m":  {60, "seconds"},
	"h":  {60 * 60, "seconds"},
	"d":  {60 * 60 * 24, "seconds"},

	// Mass:
	"ng": {9.999999999999999e-10, "grams"},
	"ug": {1.0 / 1000 / 1000, "grams"},
	"mg": {1.0 / 1000, "grams"},
	"g":  {1, "grams"},
	"kg": {1000, "grams"},
	"t":  {1000 * 1000, "grams"},

	// Volume:
	"ml": {1.0 / 1000, "liters"},
	"l":  {1, "liters"},
	"hl": {100, "liters"},

	// Misc:
	"packets": {1, "packets"},
	"lm":      {1, "lumens"},
	"dbm":     {1, "decibel-milliwatts"},
	"f":       {1, "degrees-fahrenheit"},
	"k":       {1, "degrees-kelvin"},
}

// formatUoMs maps canonical unit names to the short form rendered in
// perfdata output, mirroring Icinga 2's l_FormatUoMs. Canonical names not
// present here (e.g. "packets") render with no unit.
var formatUoMs = map[string]string{
	"ampere-seconds":     "As",
	"amperes":            "A",
	"bits":               "b",
	"bytes":              "B",
	"decibel-milliwatts": "dBm",
	"degrees-celsius":    "C",
	"degrees-fahrenheit": "F",
	"degrees-kelvin":     "K",
	"grams":              "g",
	"liters":             "l",
	"lumens":             "lm",
	"ohms":               "O",
	"percent":            "%",
	"seconds":            "s",
	"volts":              "V",
	"watt-hours":         "Wh",
	"watts":              "W",
}

// lookupUoM resolves a unit of measure the way Icinga 2's
// PerfdataValue::Parse does: the case-sensitive table first, then the
// ASCII-lowercased unit in the case-insensitive table. The counter unit "c" sets
// the counter flag with a factor of 1. An unknown unit is reported as
// unknown with a factor of 1 — Icinga keeps the value and drops the unit.
func lookupUoM(unit string) (factor float64, canonical string, counter bool, known bool) {
	if unit == "c" {
		return 1, "", true, true
	}

	if u, ok := caseSensitiveUoMs[unit]; ok {
		return u.factor, u.out, false, true
	}

	if u, ok := caseInsensitiveUoMs[asciiLower(unit)]; ok {
		return u.factor, u.out, false, true
	}

	return 1, "", false, false
}

// asciiLower lowercases only ASCII letters, matching Icinga 2's
// boost::algorithm::to_lower in the classic locale. strings.ToLower would also
// fold non-ASCII letters, e.g. the Kelvin sign U+212A to 'k'.
func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}

// normalized returns a copy of the metric with its value and set thresholds
// multiplied by the unit of measure's factor, and the canonical short unit
// rendered for output, mirroring Icinga 2's PerfdataValue::Parse followed
// by PerfdataValue::Format. The counter unit "c" scales nothing and renders
// as "c"; an unknown unit scales nothing and renders with no unit, matching
// Icinga's behavior of keeping the value and dropping the unit.
func (m PerformanceMetric) normalized() (PerformanceMetric, string) {
	factor, canonical, counter, known := lookupUoM(m.UnitOM)
	if counter {
		return m, "c"
	}

	unit := ""
	if known {
		unit = formatUoMs[canonical]
	}

	out := PerformanceMetric{Value: m.Value * factor, UnitOM: unit}
	if m.Warn != nil {
		out.Warn = new(*m.Warn * factor)
	}
	if m.Crit != nil {
		out.Crit = new(*m.Crit * factor)
	}
	if m.Min != nil {
		out.Min = new(*m.Min * factor)
	}
	if m.Max != nil {
		out.Max = new(*m.Max * factor)
	}

	return out, unit
}
