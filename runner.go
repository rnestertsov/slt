package slt

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Options configures test execution behavior
type Options struct {
	Verbose  bool   // Show detailed test output
	Compare  bool   // Run optimizer comparison mode
	FailFast bool   // Stop on first failure
	Pattern  string // Glob pattern to filter test files
}

// =============================================================================
// Overall stats
// =============================================================================

// RunStats contains results for one or more test files
type RunStats struct {
	// Aggregate statistics across all files
	TotalFiles  int
	PassedFiles int
	FailedFiles int
	ErrorFiles  int

	TotalTests  int
	PassedTests int
	FailedTests int

	// Comparison mode statistics
	QueriesCompared    int
	ComparisonSkipped  int
	Mismatches         int

	Duration time.Duration

	// Per-file results (always populated, single file = 1 element)
	FileResults []FileResult
}

// HasFailures returns true if any files had parse errors or test failures.
func (s *RunStats) HasFailures() bool {
	return s.ErrorFiles > 0 || s.FailedFiles > 0
}

// =============================================================================
// File stats
// =============================================================================

// FileResult represents outcome of a single test file
type FileResult struct {
	Path     string
	Stats    *TestFileStats // Per-file test statistics
	ParseErr error
	Duration time.Duration
}

// =============================================================================
// Test case stats
// =============================================================================

// TestFileStats tracks statistics for tests within a single file
type TestFileStats struct {
	TotalTests  int
	Passed      int
	Failed      int
	Duration    time.Duration
	FailedTests []TestResult

	// Comparison mode statistics
	QueriesCompared   int
	ComparisonSkipped int
	Mismatches        int
}

// TestResult represents the result of running a single test case
type TestResult struct {
	Test            TestCase
	Passed          bool
	Error           error
	Duration        time.Duration
	ComparisonError error // set when optimizer comparison finds a mismatch
}

// =============================================================================
// Runner
// =============================================================================

// Runner executes SLT tests
type Runner struct {
	engine  Engine
	options Options
}

// NewRunner creates a new test runner with the given engine and options.
// The engine must implement the Engine interface. For comparison mode (-c),
// the engine must also implement OptimizerToggler.
func NewRunner(engine Engine, opts Options) *Runner {
	return &Runner{
		engine:  engine,
		options: opts,
	}
}

// Run executes SLT tests from file or directory (auto-detected).
// Returns unified RunStats regardless of input type.
func (r *Runner) Run(ctx context.Context, path string) (*RunStats, error) {
	// Validate comparison mode requirements
	if r.options.Compare {
		if _, ok := r.engine.(OptimizerToggler); !ok {
			return nil, fmt.Errorf("comparison mode requires engine to implement OptimizerToggler")
		}
	}

	startTime := time.Now()

	// Auto-detect file vs directory
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat path: %w", err)
	}

	var files []string
	if info.IsDir() {
		files, err = r.scanSLTFiles(path)
		if err != nil {
			return nil, fmt.Errorf("scan directory: %w", err)
		}
		if len(files) == 0 {
			return nil, fmt.Errorf("no .slt files found in directory: %s", path)
		}
	} else {
		files = []string{path}
	}

	// Execute all files
	stats := &RunStats{
		TotalFiles:  len(files),
		FileResults: make([]FileResult, 0, len(files)),
	}

	for _, file := range files {
		result := r.runSingleFile(ctx, file)
		stats.FileResults = append(stats.FileResults, result)

		// Aggregate statistics
		r.aggregateStats(stats, result)

		// Fail-fast check
		if r.options.FailFast && (result.ParseErr != nil || (result.Stats != nil && result.Stats.Failed > 0)) {
			break
		}
	}

	stats.Duration = time.Since(startTime)
	return stats, nil
}

// scanSLTFiles scans a directory for .slt test files, applying optional pattern filter.
// Returns sorted list of file paths for deterministic execution order.
func (r *Runner) scanSLTFiles(dir string) ([]string, error) {
	var files []string

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Check if file matches *.slt pattern
		matched, err := filepath.Match("*.slt", filepath.Base(path))
		if err != nil {
			return fmt.Errorf("invalid base pattern: %w", err)
		}
		if !matched {
			return nil
		}

		// Apply optional pattern filter
		if r.options.Pattern != "" {
			relPath, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}

			matched, err := filepath.Match(r.options.Pattern, relPath)
			if err != nil {
				return fmt.Errorf("invalid pattern: %w", err)
			}
			if !matched {
				return nil
			}
		}

		files = append(files, path)
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Sort for deterministic order
	sort.Strings(files)

	return files, nil
}

// runSingleFile executes all tests in a single file and returns aggregated results.
func (r *Runner) runSingleFile(ctx context.Context, path string) FileResult {
	startTime := time.Now()
	result := FileResult{
		Path: path,
	}

	// Reset engine state for new file
	r.engine.Reset()

	// Parse test file
	tests, err := ParseFile(path)
	if err != nil {
		result.ParseErr = fmt.Errorf("parse file: %w", err)
		result.Duration = time.Since(startTime)
		return result
	}

	// Run all tests in file
	stats := &TestFileStats{}
	testStartTime := time.Now()

	for _, test := range tests {
		testResult := r.runTest(ctx, test)
		stats.TotalTests++

		if testResult.Passed {
			stats.Passed++
		} else {
			stats.Failed++
			stats.FailedTests = append(stats.FailedTests, testResult)
		}

		// Track comparison statistics
		if query, ok := test.(*Query); ok && r.options.Compare {
			if isExplainQuery(query.SQL) {
				stats.ComparisonSkipped++
			} else {
				stats.QueriesCompared++
				if testResult.ComparisonError != nil {
					stats.Mismatches++
				}
			}
		}
	}

	stats.Duration = time.Since(testStartTime)
	result.Stats = stats
	result.Duration = time.Since(startTime)

	return result
}

// aggregateStats updates RunStats with results from a single file.
func (r *Runner) aggregateStats(stats *RunStats, result FileResult) {
	if result.ParseErr != nil {
		stats.ErrorFiles++
		return
	}

	if result.Stats == nil {
		return
	}

	// Aggregate test counts
	stats.TotalTests += result.Stats.TotalTests
	stats.PassedTests += result.Stats.Passed
	stats.FailedTests += result.Stats.Failed

	// Aggregate comparison counts
	stats.QueriesCompared += result.Stats.QueriesCompared
	stats.ComparisonSkipped += result.Stats.ComparisonSkipped
	stats.Mismatches += result.Stats.Mismatches

	// Categorize file outcome
	if result.Stats.Failed > 0 {
		stats.FailedFiles++
	} else {
		stats.PassedFiles++
	}
}

// runTest executes a single test case
func (r *Runner) runTest(ctx context.Context, test TestCase) TestResult {
	startTime := time.Now()
	result := TestResult{
		Test:     test,
		Passed:   false,
		Duration: 0,
	}

	switch t := test.(type) {
	case *Statement:
		result.Error = r.runStatement(ctx, t)
	case *Query:
		result.Error = r.runQuery(ctx, t)
	default:
		result.Error = fmt.Errorf("unknown test type: %T", test)
	}

	result.Duration = time.Since(startTime)
	result.Passed = (result.Error == nil)

	// In comparison mode, compare optimized vs unoptimized for queries that passed.
	// EXPLAIN queries are skipped: they test plan structure, not data results,
	// so comparing optimizer-on vs optimizer-off output is meaningless.
	if r.options.Compare && result.Passed {
		if query, ok := test.(*Query); ok && !isExplainQuery(query.SQL) {
			compErr := r.compareOptimized(ctx, query)
			if compErr != nil {
				result.Passed = false
				result.ComparisonError = compErr
				result.Error = fmt.Errorf("optimizer comparison: %w", compErr)
			}
		}
	}

	return result
}

// runStatement executes a statement test
func (r *Runner) runStatement(ctx context.Context, stmt *Statement) error {
	err := r.engine.ExecStatement(ctx, stmt.SQL)

	if stmt.ExpectOK {
		// Expecting success
		if err != nil {
			return fmt.Errorf("statement failed (expected ok): %w", err)
		}
		return nil
	} else {
		// Expecting error
		if err == nil {
			return fmt.Errorf("statement succeeded (expected error)")
		}

		// If specific error message expected, verify it
		if stmt.Error != "" {
			errMsg := err.Error()
			if !strings.Contains(errMsg, stmt.Error) {
				return fmt.Errorf("error message mismatch: got '%s', expected to contain '%s'",
					errMsg, stmt.Error)
			}
		}

		return nil
	}
}

// runQuery executes a query test
func (r *Runner) runQuery(ctx context.Context, query *Query) error {
	// Execute query
	results, err := r.engine.ExecQuery(ctx, query.SQL)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	// Normalize results according to type line and sort mode
	normalized, err := NormalizeResults(results, query.TypeLine, query.SortMode)
	if err != nil {
		return fmt.Errorf("normalize results: %w", err)
	}

	// Check if using hash comparison
	if query.HashThreshold > 0 && len(normalized) > query.HashThreshold {
		return CompareWithHash(normalized, query.ExpectedHash)
	}

	// Compare with expected results
	return CompareResults(normalized, query.Expected)
}

// compareOptimized runs a query with and without optimizer and compares results.
func (r *Runner) compareOptimized(ctx context.Context, query *Query) error {
	toggler := r.engine.(OptimizerToggler) // validated in Run()
	defer toggler.SetOptimizerEnabled(false) // restore default disabled state

	// Run with optimizer disabled
	toggler.SetOptimizerEnabled(false)
	unoptResults, err := r.engine.ExecQuery(ctx, query.SQL)
	if err != nil {
		return fmt.Errorf("unoptimized query failed: %w", err)
	}

	// Run with optimizer enabled
	toggler.SetOptimizerEnabled(true)
	optResults, err := r.engine.ExecQuery(ctx, query.SQL)
	if err != nil {
		return fmt.Errorf("optimized query failed: %w", err)
	}

	// Normalize both result sets with the same sort mode
	unoptNormalized, err := NormalizeResults(unoptResults, query.TypeLine, query.SortMode)
	if err != nil {
		return fmt.Errorf("normalize unoptimized: %w", err)
	}

	optNormalized, err := NormalizeResults(optResults, query.TypeLine, query.SortMode)
	if err != nil {
		return fmt.Errorf("normalize optimized: %w", err)
	}

	// Compare the normalized results
	return CompareResults(unoptNormalized, optNormalized)
}

// isExplainQuery returns true if the SQL is an EXPLAIN statement.
// EXPLAIN queries test plan structure, not data results, so they are
// skipped from optimizer comparison.
func isExplainQuery(sql string) bool {
	return strings.HasPrefix(strings.TrimSpace(strings.ToUpper(sql)), "EXPLAIN")
}

// truncateSQL truncates SQL for display
func truncateSQL(sql string) string {
	const maxLen = 60
	if len(sql) <= maxLen {
		return sql
	}
	return sql[:maxLen] + "..."
}
