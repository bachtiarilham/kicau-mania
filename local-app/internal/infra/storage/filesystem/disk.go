// Package filesystem provides file operations, disk capacity, and exclusive file locks.
package filesystem

import "os"

// Disk supplies filesystem operations; lifecycle decisions belong to the usecase.
type Disk struct{}

func (Disk) Prepare(path string) error { return os.MkdirAll(path, 0700) }
func (Disk) Size(path string) (int64, error) {
	st, e := os.Stat(path)
	if e != nil {
		return 0, e
	}
	return st.Size(), nil
}
func (Disk) Commit(partial, final string) error { return os.Rename(partial, final) }
func (Disk) Free(path string) (uint64, error)   { return FreeBytes(path) }
