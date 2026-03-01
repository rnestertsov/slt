// ABOUTME: Core type definitions for the SLT (SQL Logic Test) framework.
// ABOUTME: Defines TestCase interface, Statement, Query, SortMode, and Location types.

package slt

// TestCase represents a single test case from a .slt file
type TestCase interface {
	isTestCase()
	GetLocation() Location
	GetSQL() string
}

// Statement represents a statement test (DDL/DML)
type Statement struct {
	SQL      string
	ExpectOK bool     // true if expecting success, false if expecting error
	Error    string   // expected error message (if ExpectOK is false)
	Location Location // file location for error reporting
}

func (*Statement) isTestCase() {}

func (s *Statement) GetLocation() Location {
	return s.Location
}

func (s *Statement) GetSQL() string {
	return s.SQL
}

// Query represents a query test (SELECT)
type Query struct {
	SQL           string
	TypeLine      string // e.g., "ITR" for Integer, Text, Real
	SortMode      SortMode
	HashThreshold int        // if > 0, use MD5 hash for results with more than N values
	Expected      []string   // expected result lines (each line is space-joined column values)
	ExpectedHash  string     // expected MD5 hash (if HashThreshold exceeded)
	Location      Location   // file location for error reporting
}

func (*Query) isTestCase() {}

func (q *Query) GetLocation() Location {
	return q.Location
}

func (q *Query) GetSQL() string {
	return q.SQL
}

// SortMode specifies how to sort query results
type SortMode int

const (
	SortModeNone  SortMode = iota // nosort - preserve query order
	SortModeRow                   // rowsort - sort by entire row
	SortModeValue                 // valuesort - sort all values as single list
)

// String returns a string representation of SortMode
func (s SortMode) String() string {
	switch s {
	case SortModeNone:
		return "nosort"
	case SortModeRow:
		return "rowsort"
	case SortModeValue:
		return "valuesort"
	default:
		return "unknown"
	}
}

// Location represents a position in a test file
type Location struct {
	File string
	Line int
}

// Format type characters
const (
	TypeInteger = 'I'
	TypeReal    = 'R'
	TypeText    = 'T'
)
