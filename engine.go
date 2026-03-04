// ABOUTME: Engine interface for pluggable SQL engine adapters.
// ABOUTME: Defines the contract that any SQL engine must implement for SLT testing.

package slt

import "context"

// Engine is the interface that SQL engines must implement for SLT testing.
// The SLT library calls these methods to execute test cases. The engine
// is responsible for converting its internal result representation
// (e.g., Arrow RecordBatches) to [][]string before returning.
type Engine interface {
	// ExecStatement executes a DDL/DML statement (CREATE TABLE, INSERT, etc.).
	// Returns nil on success, error on failure.
	ExecStatement(ctx context.Context, sql string) error

	// ExecQuery executes a SELECT query and returns results as string rows.
	// Each row is a slice of column values formatted as strings.
	// An empty result set returns nil/empty slice, not an error.
	ExecQuery(ctx context.Context, sql string) ([][]string, error)

	// Reset resets engine state for a new test file.
	// Called before each .slt file to ensure clean state.
	Reset()
}

// OptimizerToggler is an optional interface for engines that support
// toggling optimizer state. Required only for comparison mode (-c flag).
// If the engine does not implement this interface, comparison mode
// returns an error.
type OptimizerToggler interface {
	SetOptimizerEnabled(enabled bool)
	OptimizerEnabled() bool
}
