// Package cursor defines the port for enlarging and restoring the OS cursor.
package cursor

// Enlarger swaps the system cursors to enlarged copies and restores them.
type Enlarger interface {
	Enlarge() error
	Restore() error
}
