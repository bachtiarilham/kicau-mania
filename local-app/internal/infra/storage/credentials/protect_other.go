//go:build !windows

package credentials

import "errors"

// Headless Linux remains available; portable setup credential storage targets Windows.
type Protector struct{}

func (Protector) Protect([]byte) ([]byte, error) {
	return nil, errors.New("portable setup requires Windows DPAPI")
}
func (Protector) Unprotect([]byte) ([]byte, error) {
	return nil, errors.New("portable setup requires Windows DPAPI")
}
