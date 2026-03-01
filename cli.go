// ABOUTME: CLI entry point for SLT test runners.
// ABOUTME: Parses flags, runs tests, formats output, returns exit code.

package slt

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// CliRun runs the SLT test runner CLI, parsing flags from os.Args.
// Returns an exit code following testing.MainStart.Run() convention:
//   - 0: all tests passed
//   - 1: one or more tests failed
//   - 2: usage error or runtime error
func CliRun(ctx context.Context, engine Engine) int {
	opts, path, err := parseCliFlags()
	if err == flag.ErrHelp {
		return 0
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		printCliUsage(os.Stderr)
		return 2
	}

	runner := NewRunner(engine, opts)
	stats, err := runner.Run(ctx, path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 2
	}

	formatter := NewFormatter(os.Stdout, opts)
	formatter.PrintResults(stats)

	if stats.HasFailures() {
		return 1
	}
	return 0
}

func parseCliFlags() (Options, string, error) {
	name := filepath.Base(os.Args[0])
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.Usage = func() { printCliUsage(fs.Output()) }

	verbose := fs.Bool("v", false, "verbose output")
	verboseLong := fs.Bool("verbose", false, "verbose output")
	compare := fs.Bool("c", false, "optimizer comparison mode")
	compareLong := fs.Bool("compare", false, "optimizer comparison mode")
	failFast := fs.Bool("fail-fast", false, "exit on first failure")
	pattern := fs.String("pattern", "", "filter files by glob pattern")

	if err := fs.Parse(os.Args[1:]); err != nil {
		return Options{}, "", err
	}

	if fs.NArg() == 0 {
		return Options{}, "", fmt.Errorf("missing required path argument")
	}
	if fs.NArg() > 1 {
		return Options{}, "", fmt.Errorf("too many arguments: expected 1 path, got %d", fs.NArg())
	}

	path := fs.Arg(0)
	opts := Options{
		Verbose:  *verbose || *verboseLong,
		Compare:  *compare || *compareLong,
		FailFast: *failFast,
		Pattern:  *pattern,
	}

	return opts, path, nil
}

func printCliUsage(w io.Writer) {
	name := filepath.Base(os.Args[0])
	fmt.Fprintln(w, "SLT Test Runner - SQL Logic Test executor")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintf(w, "  %s <path> [flags]\n", name)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Arguments:")
	fmt.Fprintln(w, "  path          File or directory containing .slt tests")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  -v, --verbose       Show detailed test output")
	fmt.Fprintln(w, "  -c, --compare       Run optimizer comparison mode")
	fmt.Fprintln(w, "  --fail-fast         Exit on first test failure")
	fmt.Fprintln(w, "  --pattern <glob>    Filter test files by pattern")
	fmt.Fprintln(w, "  -h, --help          Show this help")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintf(w, "  %s testdata/slt/basic.slt\n", name)
	fmt.Fprintf(w, "  %s -c testdata/slt/\n", name)
	fmt.Fprintf(w, "  %s --pattern \"optimizer/*\" testdata/slt/\n", name)
}
