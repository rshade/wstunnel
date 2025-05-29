package tunnel

import (
	"bytes"
	"strings"
	"testing"

	"gopkg.in/inconshreveable/log15.v2"
)

func TestValidateLimit(t *testing.T) {
	tests := []struct {
		name         string
		value        int
		min          int
		max          int
		defaultValue int
		expected     int
		expectLog    string
	}{
		{
			name:         "value within range",
			value:        50,
			min:          10,
			max:          100,
			defaultValue: 20,
			expected:     50,
			expectLog:    "",
		},
		{
			name:         "value below minimum",
			value:        5,
			min:          10,
			max:          100,
			defaultValue: 20,
			expected:     20,
			expectLog:    "Configuration limit below minimum, using default",
		},
		{
			name:         "value above maximum",
			value:        150,
			min:          10,
			max:          100,
			defaultValue: 20,
			expected:     100,
			expectLog:    "Configuration limit above maximum, using maximum",
		},
		{
			name:         "value at minimum",
			value:        10,
			min:          10,
			max:          100,
			defaultValue: 20,
			expected:     10,
			expectLog:    "",
		},
		{
			name:         "value at maximum",
			value:        100,
			min:          10,
			max:          100,
			defaultValue: 20,
			expected:     100,
			expectLog:    "",
		},
		{
			name:         "high value warning",
			value:        150,
			min:          10,
			max:          200,
			defaultValue: 20,
			expected:     150,
			expectLog:    "Configuration limit is high",
		},
		{
			name:         "value exactly 100 no warning",
			value:        100,
			min:          10,
			max:          200,
			defaultValue: 20,
			expected:     100,
			expectLog:    "",
		},
		{
			name:         "value just above 100 with warning",
			value:        101,
			min:          10,
			max:          200,
			defaultValue: 20,
			expected:     101,
			expectLog:    "Configuration limit is high",
		},
		{
			name:         "negative value below minimum",
			value:        -5,
			min:          0,
			max:          100,
			defaultValue: 10,
			expected:     10,
			expectLog:    "Configuration limit below minimum, using default",
		},
		{
			name:         "zero value valid",
			value:        0,
			min:          0,
			max:          100,
			defaultValue: 10,
			expected:     0,
			expectLog:    "",
		},
		{
			name:         "zero minimum with value below",
			value:        -1,
			min:          0,
			max:          100,
			defaultValue: 5,
			expected:     5,
			expectLog:    "Configuration limit below minimum, using default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture log output
			var logOutput bytes.Buffer
			logger := log15.New("test", "validateLimit")
			logger.SetHandler(log15.StreamHandler(&logOutput, log15.LogfmtFormat()))

			result := validateLimit(logger, "test-param", tt.value, tt.min, tt.max, tt.defaultValue)

			if result != tt.expected {
				t.Errorf("Expected validateLimit to return %d, got %d", tt.expected, result)
			}

			logStr := logOutput.String()
			if tt.expectLog != "" {
				if !strings.Contains(logStr, tt.expectLog) {
					t.Errorf("Expected log to contain %q, but log was: %s", tt.expectLog, logStr)
				}
			} else {
				if logStr != "" {
					t.Errorf("Expected no log output, but got: %s", logStr)
				}
			}
		})
	}
}

func TestValidateLimit_ParameterLogging(t *testing.T) {
	var logOutput bytes.Buffer
	logger := log15.New("test", "validateLimit")
	logger.SetHandler(log15.StreamHandler(&logOutput, log15.LogfmtFormat()))

	paramName := "max-connections-per-token"

	// Test below minimum - should log param name, value, min, and default
	validateLimit(logger, paramName, 5, 10, 100, 25)

	logStr := logOutput.String()

	// Check that all expected values are logged
	expectedValues := []string{
		paramName,
		"param=" + paramName,
		"value=5",
		"min=10",
		"default=25",
	}

	for _, expected := range expectedValues {
		if !strings.Contains(logStr, expected) {
			t.Errorf("Expected log to contain %q, but log was: %s", expected, logStr)
		}
	}
}

func TestValidateLimit_AboveMaximumLogging(t *testing.T) {
	var logOutput bytes.Buffer
	logger := log15.New("test", "validateLimit")
	logger.SetHandler(log15.StreamHandler(&logOutput, log15.LogfmtFormat()))

	paramName := "max-requests-per-tunnel"

	// Test above maximum - should log param name, value, and max
	result := validateLimit(logger, paramName, 150, 10, 100, 25)

	if result != 100 {
		t.Errorf("Expected result to be 100 (maximum), got %d", result)
	}

	logStr := logOutput.String()

	// Check that all expected values are logged
	expectedValues := []string{
		paramName,
		"param=" + paramName,
		"value=150",
		"max=100",
	}

	for _, expected := range expectedValues {
		if !strings.Contains(logStr, expected) {
			t.Errorf("Expected log to contain %q, but log was: %s", expected, logStr)
		}
	}
}

func TestValidateLimit_HighValueWarning(t *testing.T) {
	var logOutput bytes.Buffer
	logger := log15.New("test", "validateLimit")
	logger.SetHandler(log15.StreamHandler(&logOutput, log15.LogfmtFormat()))

	paramName := "test-high-value"

	// Test high value warning - should log param name, value, and recommendation
	result := validateLimit(logger, paramName, 500, 10, 1000, 25)

	if result != 500 {
		t.Errorf("Expected result to be 500 (input value), got %d", result)
	}

	logStr := logOutput.String()

	// Check that warning is logged with recommendation
	expectedValues := []string{
		"Configuration limit is high",
		paramName,
		"param=" + paramName,
		"value=500",
		"recommended=10-100",
	}

	for _, expected := range expectedValues {
		if !strings.Contains(logStr, expected) {
			t.Errorf("Expected log to contain %q, but log was: %s", expected, logStr)
		}
	}
}

func TestValidateLimit_EdgeCases(t *testing.T) {
	logger := log15.New("test", "validateLimit")
	logger.SetHandler(log15.DiscardHandler()) // Discard logs for these tests

	tests := []struct {
		name         string
		value        int
		min          int
		max          int
		defaultValue int
		expected     int
	}{
		{
			name:         "min equals max",
			value:        50,
			min:          10,
			max:          10,
			defaultValue: 10,
			expected:     10, // Should use default since value > max
		},
		{
			name:         "min greater than max",
			value:        15,
			min:          20,
			max:          10,
			defaultValue: 5,
			expected:     5, // Should use default since value < min
		},
		{
			name:         "large numbers",
			value:        1000000,
			min:          100000,
			max:          2000000,
			defaultValue: 500000,
			expected:     1000000,
		},
		{
			name:         "negative range",
			value:        -5,
			min:          -10,
			max:          -1,
			defaultValue: -3,
			expected:     -5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateLimit(logger, "test-param", tt.value, tt.min, tt.max, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("Expected validateLimit to return %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestValidateLimit_RealWorldScenarios(t *testing.T) {
	logger := log15.New("test", "validateLimit")
	logger.SetHandler(log15.DiscardHandler()) // Discard logs for these tests

	tests := []struct {
		name         string
		paramName    string
		value        int
		min          int
		max          int
		defaultValue int
		expected     int
		description  string
	}{
		{
			name:         "max requests per tunnel - normal",
			paramName:    "max-requests-per-tunnel",
			value:        20,
			min:          1,
			max:          10000,
			defaultValue: 20,
			expected:     20,
			description:  "Normal configuration for request limit",
		},
		{
			name:         "max requests per tunnel - too low",
			paramName:    "max-requests-per-tunnel",
			value:        0,
			min:          1,
			max:          10000,
			defaultValue: 20,
			expected:     20,
			description:  "Zero requests not allowed, use default",
		},
		{
			name:         "max clients per token - unlimited",
			paramName:    "max-clients-per-token",
			value:        0,
			min:          0,
			max:          10000,
			defaultValue: 0,
			expected:     0,
			description:  "Zero means unlimited clients",
		},
		{
			name:         "max clients per token - reasonable limit",
			paramName:    "max-clients-per-token",
			value:        10,
			min:          0,
			max:          10000,
			defaultValue: 0,
			expected:     10,
			description:  "Reasonable client limit",
		},
		{
			name:         "max clients per token - excessive",
			paramName:    "max-clients-per-token",
			value:        50000,
			min:          0,
			max:          10000,
			defaultValue: 0,
			expected:     10000,
			description:  "Too many clients, cap at maximum",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateLimit(logger, tt.paramName, tt.value, tt.min, tt.max, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("%s: Expected %d, got %d", tt.description, tt.expected, result)
			}
		})
	}
}
