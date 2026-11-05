package filesystem

import (
	"os"

	"golang.org/x/sys/windows"
)

func FreeBytes(path string) (uint64, error) {
	p, e := windows.UTF16PtrFromString(path)
	if e != nil {
		return 0, e
	}
	var free, total, totalFree uint64
	e = windows.GetDiskFreeSpaceEx(p, &free, &total, &totalFree)
	return free, e
}
func Lock(path string) (*os.File, error) {
	f, e := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	var o windows.Overlapped
	e = windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &o)
	if e != nil {
		f.Close()
		return nil, e
	}
	return f, nil
}
