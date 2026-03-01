// ABOUTME: Formatter writes formatted SLT test results to an io.Writer.
// ABOUTME: Provides reusable output formatting so library consumers don't reimplement it.

package slt

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Formatter writes formatted SLT test results to an io.Writer.
type Formatter struct {
	w    io.Writer
	opts Options
}

// NewFormatter creates a new Formatter that writes to w using the given options.
func NewFormatter(w io.Writer, opts Options) *Formatter {
	return &Formatter{w: w, opts: opts}
}

// PrintResults writes the complete formatted output for a test run.
// This is the main entry point — prints header, per-file progress,
// comparison summary (if compare mode), and overall summary.
func (f *Formatter) PrintResults(stats *RunStats) {
	// Print header
	if stats.TotalFiles > 1 {
		fmt.Fprintf(f.w, "\nRunning %d test files\n\n", stats.TotalFiles)
	} else {
		fmt.Fprintf(f.w, "\nRunning: %s\n\n", stats.FileResults[0].Path)
	}

	// Print per-file results (progress lines)
	for i, result := range stats.FileResults {
		f.printFileResult(result, i+1, stats.TotalFiles)
	}

	// Print overall summary
	f.printSummary(stats)
}

func (f *Formatter) printFileResult(result FileResult, index, total int) {
	relPath := result.Path
	if cwd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(cwd, result.Path); err == nil {
			relPath = rel
		}
	}

	status := "PASS"
	details := ""

	if result.ParseErr != nil {
		status = "ERROR"
		details = "(parse failed)"
	} else if result.Stats != nil && result.Stats.Failed > 0 {
		status = "FAIL"
		details = fmt.Sprintf("(%d tests, %.2fs)", result.Stats.TotalTests, result.Duration.Seconds())
	} else if result.Stats != nil {
		details = fmt.Sprintf("(%d tests, %.2fs)", result.Stats.TotalTests, result.Duration.Seconds())
	}

	// Format progress line with padding
	padding := 40 - len(relPath)
	if padding < 1 {
		padding = 1
	}

	if total > 1 {
		fmt.Fprintf(f.w, "[%d/%d] %s %s %s %s\n", index, total, relPath, dots(padding), status, details)
	} else {
		fmt.Fprintf(f.w, "%s %s %s %s\n", relPath, dots(padding), status, details)
	}
}

func (f *Formatter) printSummary(stats *RunStats) {
	// Print comparison summary before main summary
	if f.opts.Compare {
		f.printComparisonSummary(stats)
	}

	fmt.Fprintln(f.w)
	fmt.Fprintln(f.w, "========================================")
	fmt.Fprintln(f.w, "Summary")
	fmt.Fprintln(f.w, "========================================")

	if stats.TotalFiles > 1 {
		fmt.Fprintf(f.w, "Files:    %d (%d passed, %d failed", stats.TotalFiles, stats.PassedFiles, stats.FailedFiles)
		if stats.ErrorFiles > 0 {
			fmt.Fprintf(f.w, ", %d error", stats.ErrorFiles)
		}
		fmt.Fprintln(f.w, ")")
	}

	if f.opts.Compare && stats.Mismatches > 0 {
		testFailures := stats.FailedTests - stats.Mismatches
		fmt.Fprintf(f.w, "Tests:    %d (%d passed, %d failed, %d mismatched)\n",
			stats.TotalTests, stats.PassedTests, testFailures, stats.Mismatches)
	} else {
		fmt.Fprintf(f.w, "Tests:    %d (%d passed, %d failed)\n", stats.TotalTests, stats.PassedTests, stats.FailedTests)
	}
	fmt.Fprintf(f.w, "Duration: %.2fs\n", stats.Duration.Seconds())
	fmt.Fprintln(f.w, "========================================")

	// Print parse errors if any
	f.printParseErrors(stats)

	// Print failed files if any
	f.printFailedFiles(stats)

	fmt.Fprintln(f.w)
}

func (f *Formatter) printComparisonSummary(stats *RunStats) {
	if stats.QueriesCompared == 0 && stats.ComparisonSkipped == 0 {
		return
	}

	fmt.Fprintln(f.w)
	fmt.Fprintln(f.w, "========================================")
	fmt.Fprintln(f.w, "Optimizer Comparison")
	fmt.Fprintln(f.w, "========================================")
	fmt.Fprintf(f.w, "Queries compared:  %d\n", stats.QueriesCompared)
	if stats.ComparisonSkipped > 0 {
		fmt.Fprintf(f.w, "Skipped (EXPLAIN): %d\n", stats.ComparisonSkipped)
	}
	fmt.Fprintf(f.w, "Matched:           %d\n", stats.QueriesCompared-stats.Mismatches)
	fmt.Fprintf(f.w, "Mismatched:        %d\n", stats.Mismatches)
	fmt.Fprintln(f.w, "========================================")

	if stats.Mismatches == 0 {
		fmt.Fprintln(f.w, "\nAll queries produced identical results with and without optimizer!")
	}
}

func (f *Formatter) printParseErrors(stats *RunStats) {
	var parseErrors []FileResult
	for _, result := range stats.FileResults {
		if result.ParseErr != nil {
			parseErrors = append(parseErrors, result)
		}
	}

	if len(parseErrors) > 0 {
		fmt.Fprintln(f.w)
		fmt.Fprintln(f.w, "Parse errors:")
		for _, result := range parseErrors {
			fmt.Fprintf(f.w, "  - %s: %v\n", result.Path, result.ParseErr)
		}
	}
}

func (f *Formatter) printFailedFiles(stats *RunStats) {
	var failedFiles []FileResult
	for _, result := range stats.FileResults {
		if result.Stats != nil && result.Stats.Failed > 0 {
			failedFiles = append(failedFiles, result)
		}
	}

	if len(failedFiles) == 0 {
		return
	}

	// Print file list with per-file breakdown
	fmt.Fprintln(f.w)
	fmt.Fprintln(f.w, "Failed files:")
	for _, result := range failedFiles {
		fmt.Fprintf(f.w, "  - %s %s\n", result.Path, f.fileFailureSummary(result))
	}

	// In comparison mode, split failures from mismatches
	if f.opts.Compare {
		f.printFailureDetails(failedFiles)
		f.printMismatchDetails(failedFiles)
		return
	}

	// Normal mode: print all failure details
	f.printTestDetails(failedFiles)
}

// fileFailureSummary returns a parenthetical like "(1 failed)", "(2 mismatched)",
// or "(1 failed, 2 mismatched)" depending on the failure types in the file.
func (f *Formatter) fileFailureSummary(result FileResult) string {
	if !f.opts.Compare || result.Stats.Mismatches == 0 {
		return fmt.Sprintf("(%d failed)", result.Stats.Failed)
	}
	testFailures := result.Stats.Failed - result.Stats.Mismatches
	if testFailures == 0 {
		return fmt.Sprintf("(%d mismatched)", result.Stats.Mismatches)
	}
	return fmt.Sprintf("(%d failed, %d mismatched)", testFailures, result.Stats.Mismatches)
}

// printTestDetails prints all failed test details (used in normal mode).
func (f *Formatter) printTestDetails(failedFiles []FileResult) {
	fmt.Fprintln(f.w)
	for _, result := range failedFiles {
		for _, ft := range result.Stats.FailedTests {
			f.printSingleFailure(ft)
		}
	}
}

// printFailureDetails prints details for regular test failures (non-mismatch).
func (f *Formatter) printFailureDetails(failedFiles []FileResult) {
	var failures []TestResult
	for _, result := range failedFiles {
		for _, ft := range result.Stats.FailedTests {
			if ft.ComparisonError == nil {
				failures = append(failures, ft)
			}
		}
	}
	if len(failures) == 0 {
		return
	}
	fmt.Fprintln(f.w)
	fmt.Fprintln(f.w, "Failures:")
	fmt.Fprintln(f.w)
	for _, ft := range failures {
		f.printSingleFailure(ft)
	}
}

// printMismatchDetails prints details for optimizer comparison mismatches.
func (f *Formatter) printMismatchDetails(failedFiles []FileResult) {
	var mismatches []TestResult
	for _, result := range failedFiles {
		for _, ft := range result.Stats.FailedTests {
			if ft.ComparisonError != nil {
				mismatches = append(mismatches, ft)
			}
		}
	}
	if len(mismatches) == 0 {
		return
	}
	fmt.Fprintln(f.w)
	fmt.Fprintln(f.w, "Optimizer Mismatches:")
	fmt.Fprintln(f.w)
	for _, ft := range mismatches {
		f.printSingleFailure(ft)
	}
}

// printSingleFailure prints location, SQL, and error for one failed test.
func (f *Formatter) printSingleFailure(ft TestResult) {
	location := formatTestLocation(ft.Test)
	fmt.Fprintf(f.w, "%s %s\n", location, ft.Test.GetSQL())
	if ft.Error != nil {
		fmt.Fprintf(f.w, "  Error: %v\n", ft.Error)
	}
}

func dots(n int) string {
	if n < 1 {
		return " "
	}
	result := make([]byte, n)
	for i := range result {
		result[i] = '.'
	}
	return string(result)
}

func formatTestLocation(test TestCase) string {
	return fmt.Sprintf("%s:%d:", test.GetLocation().File, test.GetLocation().Line)
}
