# slt — SQL Logic Test Framework for Go

A standalone, engine-agnostic framework for running [SQL Logic Tests](https://www.sqlite.org/sqllogictest/doc/trunk/about.wiki) (.slt files). Plug in any SQL engine by implementing a single interface.

## Install

```bash
go get github.com/rnestertsov/slt
```

## Quick Start

Implement the `Engine` interface and hand it to `CliRun`:

```go
package main

import (
    "context"
    "os"

    "github.com/rnestertsov/slt"
)

type MyEngine struct { /* your SQL engine */ }

func (e *MyEngine) ExecStatement(ctx context.Context, sql string) error {
    // Execute DDL/DML (CREATE TABLE, INSERT, etc.)
    return e.db.Exec(sql)
}

func (e *MyEngine) ExecQuery(ctx context.Context, sql string) ([][]string, error) {
    // Execute SELECT and return rows as [][]string
    return e.db.Query(sql)
}

func (e *MyEngine) Reset() {
    // Reset to clean state between test files
    e.db = newDB()
}

func main() {
    os.Exit(slt.CliRun(context.Background(), &MyEngine{}))
}
```

Build and run:

```bash
go build -o my-slt .
./my-slt testdata/basic.slt          # single file
./my-slt testdata/                   # entire directory
./my-slt --fail-fast testdata/       # stop on first failure
./my-slt --pattern "math/*" testdata/  # filter by glob
```

## SLT File Format

```
# Comments start with #

statement ok
CREATE TABLE t (id INTEGER, name TEXT)

statement ok
INSERT INTO t VALUES (1, 'Alice'), (2, 'Bob')

statement error
INSERT INTO nonexistent VALUES (1)

query IT rowsort
SELECT id, name FROM t
----
1 Alice
2 Bob
```

### Directives

| Directive | Description |
|---|---|
| `statement ok` | Execute SQL, expect success |
| `statement error` | Execute SQL, expect failure |
| `query <types> [sort]` | Execute SELECT, compare results after `----` |
| `halt` | Stop processing the file |

### Type Characters

| Char | Type |
|---|---|
| `I` | Integer |
| `R` | Real (formatted to 3 decimal places) |
| `T` | Text |

### Sort Modes

| Mode | Description |
|---|---|
| `nosort` | Compare in query order |
| `rowsort` | Sort rows before comparing (default) |
| `valuesort` | Flatten all values, sort, then compare |

## Optimizer Comparison Mode

Most SQL engines include a query optimizer that rewrites logical plans for better performance (constant folding, filter merging, predicate pushdown, etc.). These rewrites must never change query results. Comparison mode (`-c`) automates that verification.

**How it works:** For every query that passes its expected-results check, the runner executes it a second time with the optimizer toggled. It normalizes both result sets using the query's type line and sort mode, then compares them. Any difference is reported as a mismatch — distinct from a regular test failure — so you can tell at a glance whether a bug is in the optimizer or in the engine itself.

`EXPLAIN` queries are automatically skipped from comparison because they test plan structure, not data results, and their output is expected to differ between optimized and unoptimized modes.

To enable comparison mode, your engine must implement the `OptimizerToggler` interface:

```go
// OptimizerToggler is an optional interface for engines that support
// toggling optimizer state. Required only for comparison mode (-c flag).
type OptimizerToggler interface {
    SetOptimizerEnabled(enabled bool)
}
```

Example implementation:

```go
func (e *MyEngine) SetOptimizerEnabled(enabled bool) {
    e.optimizer = enabled
}
```

Run with the `-c` flag:

```bash
./my-slt -c testdata/
```

Sample output:

```
========================================
Optimizer Comparison
========================================
Queries compared:  52
Skipped (EXPLAIN): 12
Matched:           52
Mismatched:        0
========================================

All queries produced identical results with and without optimizer!
```

## Programmatic Usage

Use `Runner` directly instead of `CliRun` for embedding in test suites:

```go
func TestSLT(t *testing.T) {
    engine := &MyEngine{}
    runner := slt.NewRunner(engine, slt.Options{})

    stats, err := runner.Run(context.Background(), "testdata/")
    if err != nil {
        t.Fatal(err)
    }
    if stats.HasFailures() {
        t.Fatalf("%d/%d tests failed", stats.FailedTests, stats.TotalTests)
    }
}
```

## License

MIT
