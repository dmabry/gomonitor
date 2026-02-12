package gomonitor

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
)

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
		Warn:   1.00,
		Crit:   2.00,
		Min:    0.00,
		Max:    10.00,
		UnitOM: "ms",
	}

	result := NewCheckResult()
	result.AddPerformanceData("test", testMetric)

	if _, ok := result.PerformanceData["test"]; !ok {
		t.Error("AddPerformanceData didn't add the 'test' performance data to the map")
	}

	testMetric2 := PerformanceMetric{
		Value:  2.34,
		Warn:   2.00,
		Crit:   3.00,
		Min:    1.00,
		Max:    20.00,
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

type ExitGetter interface {
	GetExitCode() int
}

type exitStatus struct {
	Code int
}

func (e *exitStatus) GetExitCode() int {
	return e.Code
}

var exiters = make(map[int]ExitGetter)

func mockExit(code int) {
	exiters[code] = &exitStatus{code}
	panic(fmt.Sprintf("exit %v", code))
}

func TestFormatResult(t *testing.T) {
	testCases := []struct {
		name     string
		setup    func() *CheckResult
		wantOK   bool
		contains []string
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
					Value: 1.23, Warn: 1.00, Crit: 2.00, Min: 0.00, Max: 10.00, UnitOM: "ms",
				})
				return r
			},
			wantOK:   true,
			contains: []string{"Warning", "High latency detected", "'response_time'=1.23"},
		},
		{
			name: "MultiplePerfData_MultiMetrics",
			setup: func() *CheckResult {
				r := NewCheckResult()
				r.SetResult(Critical, "CPU overloaded")
				r.AddPerformanceData("cpu_usage", PerformanceMetric{Value: 95.0, Warn: 80.0, Crit: 90.0, Min: 0.0, Max: 100.0})
				r.AddPerformanceData("memory_usage", PerformanceMetric{Value: 88.5, Warn: 85.0, Crit: 95.0, Min: 0.0, Max: 100.0})
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
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.setup()
			output := result.FormatResult()

			if tc.wantOK && output == "" {
				t.Error("FormatResult returned empty string")
			}

			for _, c := range tc.contains {
				if !containsString(output, c) {
					t.Errorf("Output %q does not contain expected substring %q", output, c)
				}
			}
		})
	}
}

func TestFormatResult_PerformanceData(t *testing.T) {
	r := NewCheckResult()
	r.SetResult(OK, "Check passed")
	r.AddPerformanceData("test_metric", PerformanceMetric{
		Value: 42.5, Warn: 30.0, Crit: 50.0, Min: 0.0, Max: 100.0, UnitOM: "%",
	})
	output := r.FormatResult()

	wantFormat := "'test_metric'=42.50%;30.00;50.00;0.00;100.00"
	if !containsString(output, wantFormat) {
		t.Errorf("Performance data format incorrect.\nGot: %s\nExpected substring: %s", output, wantFormat)
	}
}

func containsString(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && findSubstringIndex(s, substr) >= 0
}

func findSubstringIndex(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func init() {
	osExit = mockExit
}

// Mock os.Exit for testing
var osExit = func(code int) {
	os.Exit(code)
}

// Mock fmt.Printf for testing
var fmtPrintf = func(format string, a ...interface{}) (n int, err error) {
	return fmt.Printf(format, a...)
}
