// ABOUTME: Parser for sqllogictest (.slt) files.
// ABOUTME: Reads test cases (statements and queries) from the SLT file format.

package slt

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// Parser parses sqllogictest files
type Parser struct {
	scanner  *bufio.Scanner
	file     *os.File
	filename string
	lineNum  int
}

// NewParser creates a new parser for the given file
func NewParser(filename string) (*Parser, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}

	return &Parser{
		scanner:  bufio.NewScanner(file),
		file:     file,
		filename: filename,
		lineNum:  0,
	}, nil
}

// Close closes the underlying file
func (p *Parser) Close() error {
	if p.file != nil {
		return p.file.Close()
	}
	return nil
}

// ParseAll parses all test cases from the file
func (p *Parser) ParseAll() ([]TestCase, error) {
	var tests []TestCase

	for {
		test, err := p.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if test != nil {
			tests = append(tests, test)
		}
	}

	return tests, nil
}

// Next parses and returns the next test case
func (p *Parser) Next() (TestCase, error) {
	for p.scanner.Scan() {
		p.lineNum++
		line := p.scanner.Text()

		// Skip empty lines and comments
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse command
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}

		cmd := parts[0]
		switch cmd {
		case "statement":
			return p.parseStatement(parts[1:])
		case "query":
			return p.parseQuery(parts[1:])
		case "hash-threshold":
			// Skip hash-threshold directives for now
			continue
		case "halt":
			// Stop processing at halt directive
			return nil, io.EOF
		default:
			return nil, fmt.Errorf("%s:%d: unknown command: %s", p.filename, p.lineNum, cmd)
		}
	}

	if err := p.scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan error: %w", err)
	}

	return nil, io.EOF
}

// parseStatement parses a statement test case
func (p *Parser) parseStatement(args []string) (*Statement, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("%s:%d: statement requires ok/error argument", p.filename, p.lineNum)
	}

	stmt := &Statement{
		Location: Location{File: p.filename, Line: p.lineNum},
	}

	expectType := args[0]
	switch expectType {
	case "ok":
		stmt.ExpectOK = true
	case "error":
		stmt.ExpectOK = false
		// Optional error message follows
		if len(args) > 1 {
			stmt.Error = strings.Join(args[1:], " ")
		}
	default:
		return nil, fmt.Errorf("%s:%d: statement expects 'ok' or 'error', got: %s", p.filename, p.lineNum, expectType)
	}

	// Read SQL (can be multi-line until next command or empty line)
	sql, err := p.readSQL()
	if err != nil {
		return nil, err
	}
	stmt.SQL = sql

	return stmt, nil
}

// parseQuery parses a query test case
func (p *Parser) parseQuery(args []string) (*Query, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("%s:%d: query requires type line", p.filename, p.lineNum)
	}

	query := &Query{
		Location:      Location{File: p.filename, Line: p.lineNum},
		TypeLine:      args[0],
		SortMode:      SortModeRow, // default to rowsort
		HashThreshold: 0,
	}

	// Parse optional sort mode
	if len(args) > 1 {
		switch args[1] {
		case "nosort":
			query.SortMode = SortModeNone
		case "rowsort":
			query.SortMode = SortModeRow
		case "valuesort":
			query.SortMode = SortModeValue
		default:
			return nil, fmt.Errorf("%s:%d: unknown sort mode: %s", p.filename, p.lineNum, args[1])
		}
	}

	// Read SQL
	sql, err := p.readSQL()
	if err != nil {
		return nil, err
	}
	query.SQL = sql

	// Read expected results (----\n followed by result lines)
	expected, err := p.readExpectedResults(query.TypeLine)
	if err != nil {
		return nil, err
	}
	query.Expected = expected

	return query, nil
}

// readSQL reads SQL text until a ---- separator or empty line.
// Blank lines are required between test cases in .slt files.
func (p *Parser) readSQL() (string, error) {
	var lines []string

	for p.scanner.Scan() {
		p.lineNum++
		line := p.scanner.Text()

		// Stop at separator (----)
		if strings.HasPrefix(strings.TrimSpace(line), "----") {
			break
		}

		// Stop at empty line (for statements without expected results)
		if strings.TrimSpace(line) == "" {
			break
		}

		lines = append(lines, line)
	}

	if len(lines) == 0 {
		return "", fmt.Errorf("%s:%d: expected SQL text", p.filename, p.lineNum)
	}

	return strings.TrimSpace(strings.Join(lines, "\n")), nil
}

// readExpectedResults reads expected results after ---- separator.
// Results are compared as text, line-by-line. Stops at empty line.
func (p *Parser) readExpectedResults(typeLine string) ([]string, error) {
	var results []string

	for p.scanner.Scan() {
		p.lineNum++
		line := p.scanner.Text()

		// Stop at empty line
		if strings.TrimSpace(line) == "" {
			break
		}

		// Preserve full line, only trim trailing spaces to normalize line endings
		results = append(results, strings.TrimRight(line, " \t"))
	}

	return results, nil
}

// ParseFile is a convenience function to parse an entire file
func ParseFile(filename string) ([]TestCase, error) {
	parser, err := NewParser(filename)
	if err != nil {
		return nil, err
	}
	defer parser.Close()

	return parser.ParseAll()
}

// ParseTypeLine parses a type line (e.g., "ITR") into individual type codes
func ParseTypeLine(typeLine string) ([]rune, error) {
	types := []rune{}
	for _, ch := range typeLine {
		switch ch {
		case TypeInteger, TypeReal, TypeText:
			types = append(types, ch)
		default:
			return nil, fmt.Errorf("unknown type character: %c", ch)
		}
	}
	return types, nil
}

// FormatValue formats a value according to its type for comparison
func FormatValue(value string, typeChar rune) (string, error) {
	switch typeChar {
	case TypeInteger:
		// Parse and reformat to normalize representation
		val, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return "", fmt.Errorf("invalid integer: %s", value)
		}
		return strconv.FormatInt(val, 10), nil

	case TypeReal:
		// Parse and reformat to normalize representation
		val, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return "", fmt.Errorf("invalid float: %s", value)
		}
		// Format with 3 decimal places (standard SLT format)
		return fmt.Sprintf("%.3f", val), nil

	case TypeText:
		// Text values are used as-is, but normalize NULL
		if strings.ToUpper(value) == "NULL" {
			return "NULL", nil
		}
		return value, nil

	default:
		return "", fmt.Errorf("unknown type character: %c", typeChar)
	}
}
