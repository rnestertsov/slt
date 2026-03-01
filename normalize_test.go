package slt

import (
	"testing"
)

func TestNormalizeResults(t *testing.T) {
	results := [][]string{
		{"3", "3.14159", "hello"},
		{"1", "2.71828", "world"},
		{"2", "1.41421", "test"},
	}

	normalized, err := NormalizeResults(results, "IRT", SortModeNone)
	if err != nil {
		t.Fatalf("NormalizeResults failed: %v", err)
	}

	// Check that values were normalized and joined into single text line
	expected := "3 3.142 hello"
	if normalized[0] != expected {
		t.Errorf("Expected '%s', got '%s'", expected, normalized[0])
	}
}

func TestNormalizeResultsRowSort(t *testing.T) {
	results := [][]string{
		{"3", "charlie"},
		{"1", "alice"},
		{"2", "bob"},
	}

	normalized, err := NormalizeResults(results, "IT", SortModeRow)
	if err != nil {
		t.Fatalf("NormalizeResults failed: %v", err)
	}

	// Results should be sorted by row (as joined text lines)
	if len(normalized) != 3 {
		t.Fatalf("Expected 3 rows, got %d", len(normalized))
	}

	if normalized[0] != "1 alice" {
		t.Errorf("First row incorrect: %v", normalized[0])
	}
	if normalized[1] != "2 bob" {
		t.Errorf("Second row incorrect: %v", normalized[1])
	}
	if normalized[2] != "3 charlie" {
		t.Errorf("Third row incorrect: %v", normalized[2])
	}
}

func TestNormalizeResultsValueSort(t *testing.T) {
	results := [][]string{
		{"3", "1"},
		{"2", "4"},
	}

	normalized, err := NormalizeResults(results, "II", SortModeValue)
	if err != nil {
		t.Fatalf("NormalizeResults failed: %v", err)
	}

	// All values should be sorted as a flat list, then reshaped and joined
	// Values: [3, 1, 2, 4] -> sorted: [1, 2, 3, 4] -> reshaped: [[1, 2], [3, 4]]
	// Expected: ["1 2", "3 4"]
	if normalized[0] != "1 2" {
		t.Errorf("First row incorrect: %v", normalized[0])
	}
	if normalized[1] != "3 4" {
		t.Errorf("Second row incorrect: %v", normalized[1])
	}
}

func TestCompareResults(t *testing.T) {
	tests := []struct {
		name     string
		actual   []string
		expected []string
		hasError bool
	}{
		{
			name:     "equal results",
			actual:   []string{"1 alice", "2 bob"},
			expected: []string{"1 alice", "2 bob"},
			hasError: false,
		},
		{
			name:     "different row count",
			actual:   []string{"1 alice"},
			expected: []string{"1 alice", "2 bob"},
			hasError: true,
		},
		{
			name:     "different values",
			actual:   []string{"1 alice"},
			expected: []string{"1 alice extra"},
			hasError: true,
		},
		{
			name:     "different values second",
			actual:   []string{"1 alice"},
			expected: []string{"1 bob"},
			hasError: true,
		},
		{
			name:     "empty results",
			actual:   []string{},
			expected: []string{},
			hasError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CompareResults(tt.actual, tt.expected)
			if tt.hasError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.hasError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestComputeHash(t *testing.T) {
	results1 := []string{"1 alice", "2 bob"}
	results2 := []string{"1 alice", "2 bob"}
	results3 := []string{"2 bob", "1 alice"}

	hash1 := ComputeHash(results1)
	hash2 := ComputeHash(results2)
	hash3 := ComputeHash(results3)

	// Same results should produce same hash
	if hash1 != hash2 {
		t.Errorf("Expected equal hashes for identical results")
	}

	// Different order should produce different hash
	if hash1 == hash3 {
		t.Errorf("Expected different hashes for different order")
	}

	// Hash should be non-empty
	if hash1 == "" {
		t.Error("Hash should not be empty")
	}
}

func TestRowLess(t *testing.T) {
	tests := []struct {
		name     string
		a        []string
		b        []string
		expected bool
	}{
		{"a < b lexicographically", []string{"1 alice"}, []string{"2 bob"}, true},
		{"a > b lexicographically", []string{"2 bob"}, []string{"1 alice"}, false},
		{"a == b", []string{"1 alice"}, []string{"1 alice"}, false},
		{"shorter is less", []string{"1"}, []string{"1 alice"}, true},
		{"longer is not less", []string{"1 alice"}, []string{"1"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rowLess(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("rowLess(%v, %v) = %v, expected %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}
