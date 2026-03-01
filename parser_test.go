package slt

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseStatement(t *testing.T) {
	// Create temporary test file
	content := `
statement ok
CREATE TABLE test (id INTEGER)

statement error
INVALID SQL
`
	tmpfile := createTempTestFile(t, content)
	defer os.Remove(tmpfile)

	// Parse file
	tests, err := ParseFile(tmpfile)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if len(tests) != 2 {
		t.Fatalf("Expected 2 tests, got %d", len(tests))
	}

	// Check first statement (ok)
	stmt1, ok := tests[0].(*Statement)
	if !ok {
		t.Fatalf("Expected Statement, got %T", tests[0])
	}
	if !stmt1.ExpectOK {
		t.Error("Expected ExpectOK=true")
	}
	if stmt1.SQL != "CREATE TABLE test (id INTEGER)" {
		t.Errorf("Unexpected SQL: %s", stmt1.SQL)
	}

	// Check second statement (error)
	stmt2, ok := tests[1].(*Statement)
	if !ok {
		t.Fatalf("Expected Statement, got %T", tests[1])
	}
	if stmt2.ExpectOK {
		t.Error("Expected ExpectOK=false")
	}
	if stmt2.SQL != "INVALID SQL" {
		t.Errorf("Unexpected SQL: %s", stmt2.SQL)
	}
}

func TestParseQuery(t *testing.T) {
	content := `
query II rowsort
SELECT id, value FROM test
----
1 100
2 200
`
	tmpfile := createTempTestFile(t, content)
	defer os.Remove(tmpfile)

	tests, err := ParseFile(tmpfile)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if len(tests) != 1 {
		t.Fatalf("Expected 1 test, got %d", len(tests))
	}

	query, ok := tests[0].(*Query)
	if !ok {
		t.Fatalf("Expected Query, got %T", tests[0])
	}

	if query.TypeLine != "II" {
		t.Errorf("Expected TypeLine 'II', got '%s'", query.TypeLine)
	}

	if query.SortMode != SortModeRow {
		t.Errorf("Expected SortModeRow, got %v", query.SortMode)
	}

	if query.SQL != "SELECT id, value FROM test" {
		t.Errorf("Unexpected SQL: %s", query.SQL)
	}

	if len(query.Expected) != 2 {
		t.Fatalf("Expected 2 result rows, got %d", len(query.Expected))
	}

	if query.Expected[0] != "1 100" {
		t.Errorf("Unexpected first row: %v", query.Expected[0])
	}

	if query.Expected[1] != "2 200" {
		t.Errorf("Unexpected second row: %v", query.Expected[1])
	}
}

func TestParseQueryNosort(t *testing.T) {
	content := `
query I nosort
SELECT id FROM test
----
3
1
2
`
	tmpfile := createTempTestFile(t, content)
	defer os.Remove(tmpfile)

	tests, err := ParseFile(tmpfile)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	query, ok := tests[0].(*Query)
	if !ok {
		t.Fatalf("Expected Query, got %T", tests[0])
	}

	if query.SortMode != SortModeNone {
		t.Errorf("Expected SortModeNone, got %v", query.SortMode)
	}
}

func TestParseEmptyResults(t *testing.T) {
	content := `
query II nosort
SELECT * FROM test WHERE id = 999
----
`
	tmpfile := createTempTestFile(t, content)
	defer os.Remove(tmpfile)

	tests, err := ParseFile(tmpfile)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	query, ok := tests[0].(*Query)
	if !ok {
		t.Fatalf("Expected Query, got %T", tests[0])
	}

	if len(query.Expected) != 0 {
		t.Errorf("Expected empty results, got %d rows", len(query.Expected))
	}
}

func TestParseTypeLine(t *testing.T) {
	tests := []struct {
		input    string
		expected []rune
		hasError bool
	}{
		{"I", []rune{'I'}, false},
		{"ITR", []rune{'I', 'T', 'R'}, false},
		{"IIITTT", []rune{'I', 'I', 'I', 'T', 'T', 'T'}, false},
		{"X", nil, true}, // invalid type
	}

	for _, tt := range tests {
		result, err := ParseTypeLine(tt.input)
		if tt.hasError {
			if err == nil {
				t.Errorf("Expected error for input '%s'", tt.input)
			}
		} else {
			if err != nil {
				t.Errorf("Unexpected error for input '%s': %v", tt.input, err)
			}
			if len(result) != len(tt.expected) {
				t.Errorf("Length mismatch for '%s': got %d, expected %d", tt.input, len(result), len(tt.expected))
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("Type mismatch at index %d for '%s': got %c, expected %c",
						i, tt.input, result[i], tt.expected[i])
				}
			}
		}
	}
}

func TestFormatValue(t *testing.T) {
	tests := []struct {
		value    string
		typeChar rune
		expected string
		hasError bool
	}{
		{"123", TypeInteger, "123", false},
		{"0", TypeInteger, "0", false},
		{"-456", TypeInteger, "-456", false},
		{"3.14159", TypeReal, "3.142", false},
		{"1.0", TypeReal, "1.000", false},
		{"hello", TypeText, "hello", false},
		{"NULL", TypeText, "NULL", false},
		{"null", TypeText, "NULL", false},
		{"abc", TypeInteger, "", true}, // invalid integer
	}

	for _, tt := range tests {
		result, err := FormatValue(tt.value, tt.typeChar)
		if tt.hasError {
			if err == nil {
				t.Errorf("Expected error for value '%s' with type %c", tt.value, tt.typeChar)
			}
		} else {
			if err != nil {
				t.Errorf("Unexpected error for value '%s' with type %c: %v", tt.value, tt.typeChar, err)
			}
			if result != tt.expected {
				t.Errorf("Format mismatch for '%s' (type %c): got '%s', expected '%s'",
					tt.value, tt.typeChar, result, tt.expected)
			}
		}
	}
}

// Helper function to create a temporary test file
func createTempTestFile(t *testing.T, content string) string {
	tmpDir := t.TempDir()
	tmpfile := filepath.Join(tmpDir, "test.slt")

	err := os.WriteFile(tmpfile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	return tmpfile
}
