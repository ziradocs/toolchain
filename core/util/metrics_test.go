// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"strings"
	"testing"
	"time"
)

func TestProcessingMetrics_ChangeInBytes(t *testing.T) {
	tests := []struct {
		name     string
		original int
		processed int
		expected string
	}{
		{"No change", 100, 100, "sin cambios"},
		{"Increase", 100, 150, "+50 bytes"},
		{"Decrease", 100, 50, "-50 bytes"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &ProcessingMetrics{
				OriginalSize:  tt.original,
				ProcessedSize: tt.processed,
			}
			result := m.ChangeInBytes()
			if result != tt.expected {
				t.Errorf("ChangeInBytes() = %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestSizeString(t *testing.T) {
	tests := []struct {
		bytes    int
		expected string
	}{
		{500, "500 bytes"},
		{1024, "1.0KB"},
		{2048, "2.0KB"},
		{1024 * 1024, "1.0MB"},
		{1024*1024 + 512*1024, "1.5MB"},
	}
	
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := SizeString(tt.bytes)
			if result != tt.expected {
				t.Errorf("SizeString(%d) = %s, want %s", tt.bytes, result, tt.expected)
			}
		})
	}
}

func TestDurationString(t *testing.T) {
	tests := []struct {
		duration time.Duration
		contains string
	}{
		{500 * time.Nanosecond, "μs"},
		{5 * time.Millisecond, "ms"},
		{2 * time.Second, "s"},
	}
	
	for _, tt := range tests {
		t.Run(tt.contains, func(t *testing.T) {
			result := DurationString(tt.duration)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("DurationString(%v) = %s, should contain %s", tt.duration, result, tt.contains)
			}
		})
	}
}

func TestProgressTracker(t *testing.T) {
	logger := NewNoop()
	tracker := NewProgressTracker(logger, "BUILD", "Compiling", 100)
	
	if tracker == nil {
		t.Fatal("NewProgressTracker() returned nil")
	}
	
	// Test Update
	tracker.Update(50)
	if tracker.current != 50 {
		t.Errorf("Update(50) set current to %d, want 50", tracker.current)
	}
	
	// Test Increment
	tracker.Increment()
	if tracker.current != 51 {
		t.Errorf("Increment() set current to %d, want 51", tracker.current)
	}
	
	// Test Finish
	tracker.Finish()
	if tracker.current != 100 {
		t.Errorf("Finish() set current to %d, want 100", tracker.current)
	}
}

func TestProgressTracker_WithZeroTotal(t *testing.T) {
	logger := NewNoop()
	tracker := NewProgressTracker(logger, "TEST", "Operation", 0)
	
	// Should not panic with zero total
	tracker.Update(10)
	tracker.Increment()
	tracker.Finish()
}

func TestOperationLogger(t *testing.T) {
	logger := NewNoop()
	opLogger := NewOperationLogger(logger, "TEST", "test operation")
	
	if opLogger == nil {
		t.Fatal("NewOperationLogger() returned nil")
	}
	
	// Test Finish without metrics
	opLogger.Finish(nil)
	
	// Test Finish with metrics
	opLogger2 := NewOperationLogger(logger, "TEST", "test operation 2")
	time.Sleep(10 * time.Millisecond) // Ensure some time passes
	metrics := &ProcessingMetrics{
		OriginalSize:   1000,
		ProcessedSize:  1100,
		ItemsProcessed: 5,
	}
	opLogger2.Finish(metrics)
	
	if metrics.Duration == 0 {
		t.Error("Finish() should set Duration in metrics")
	}
}

func TestOperationLogger_Duration(t *testing.T) {
	logger := NewNoop()
	opLogger := NewOperationLogger(logger, "TEST", "duration test")
	
	start := time.Now()
	time.Sleep(50 * time.Millisecond)
	
	metrics := &ProcessingMetrics{
		OriginalSize:  100,
		ProcessedSize: 100,
	}
	opLogger.Finish(metrics)
	
	elapsed := time.Since(start)
	
	// Duration should be approximately equal to elapsed time
	if metrics.Duration < 40*time.Millisecond || metrics.Duration > 100*time.Millisecond {
		t.Errorf("Duration = %v, expected roughly 50ms (between 40-100ms), elapsed = %v", metrics.Duration, elapsed)
	}
}
