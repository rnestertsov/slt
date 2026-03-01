package slt

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"sort"
	"strings"
)

// NormalizeResults normalizes query results according to type line and sort mode.
// Returns joined text lines where each element is a space-joined row of column values.
func NormalizeResults(results [][]string, typeLine string, sortMode SortMode) ([]string, error) {
	if len(results) == 0 {
		return nil, nil
	}

	// Parse type line
	types, err := ParseTypeLine(typeLine)
	if err != nil {
		return nil, fmt.Errorf("parse type line: %w", err)
	}

	// Normalize each value according to its type
	normalized := make([][]string, len(results))
	for i, row := range results {
		normalizedRow := make([]string, len(row))
		for j, value := range row {
			if j >= len(types) {
				return nil, fmt.Errorf("row %d has more columns than type line specifies", i)
			}
			normalizedValue, err := FormatValue(value, types[j])
			if err != nil {
				return nil, fmt.Errorf("row %d, col %d: %w", i, j, err)
			}
			normalizedRow[j] = normalizedValue
		}
		normalized[i] = normalizedRow
	}

	// Apply sort mode and join columns into text lines
	switch sortMode {
	case SortModeNone:
		return joinRows(normalized), nil

	case SortModeRow:
		sort.Slice(normalized, func(i, j int) bool {
			return rowLess(normalized[i], normalized[j])
		})
		return joinRows(normalized), nil

	case SortModeValue:
		sorted := sortByValues(normalized)
		return joinRows(sorted), nil

	default:
		return nil, fmt.Errorf("unknown sort mode: %d", sortMode)
	}
}

// joinRows joins each row's columns into a single space-separated text line
func joinRows(results [][]string) []string {
	joined := make([]string, len(results))
	for i, row := range results {
		joined[i] = strings.Join(row, " ")
	}
	return joined
}

// rowLess compares two rows lexicographically
func rowLess(a, b []string) bool {
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}

	for i := 0; i < minLen; i++ {
		if a[i] < b[i] {
			return true
		}
		if a[i] > b[i] {
			return false
		}
	}

	return len(a) < len(b)
}

// sortByValues flattens all values, sorts them, then reshapes into original dimensions
func sortByValues(results [][]string) [][]string {
	if len(results) == 0 {
		return results
	}

	// Flatten all values
	var allValues []string
	colCount := len(results[0])
	for _, row := range results {
		allValues = append(allValues, row...)
	}

	// Sort all values
	sort.Strings(allValues)

	// Reshape back into rows
	sorted := make([][]string, len(results))
	idx := 0
	for i := range sorted {
		sorted[i] = make([]string, colCount)
		for j := range sorted[i] {
			if idx < len(allValues) {
				sorted[i][j] = allValues[idx]
				idx++
			}
		}
	}

	return sorted
}

// formatResultsAsBytes converts result lines to bytes for diff
func formatResultsAsBytes(results []string) []byte {
	var buf bytes.Buffer
	for _, line := range results {
		buf.WriteString(line)
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

// CompareResults compares actual results with expected results.
// Returns nil if they match, or an error describing the difference.
func CompareResults(actual, expected []string) error {
	if len(actual) != len(expected) {
		diff := Diff(formatResultsAsBytes(expected),
			formatResultsAsBytes(actual), true)
		return fmt.Errorf("row count mismatch: got %d rows, expected %d rows\n%s",
			len(actual), len(expected), string(diff))
	}

	for i := range actual {
		if actual[i] != expected[i] {
			diff := Diff(formatResultsAsBytes(expected),
				formatResultsAsBytes(actual), true)
			return fmt.Errorf("row %d: value mismatch\n%s", i, string(diff))
		}
	}

	return nil
}

// ComputeHash computes MD5 hash of results for hash-threshold comparison
func ComputeHash(results []string) string {
	var sb strings.Builder
	for _, line := range results {
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	hash := md5.Sum([]byte(sb.String()))
	return fmt.Sprintf("%x", hash)
}

// CompareWithHash compares results using MD5 hash for large result sets
func CompareWithHash(actual []string, expectedHash string) error {
	actualHash := ComputeHash(actual)
	if actualHash != expectedHash {
		return fmt.Errorf("hash mismatch: got %s, expected %s", actualHash, expectedHash)
	}
	return nil
}
