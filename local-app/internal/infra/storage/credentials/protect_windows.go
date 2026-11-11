// Package credentials protects configuration using the current Windows user's DPAPI key.
package credentials

import (
	"errors"
	"golang.org/x/sys/windows"
	"unsafe"
)

type Protector struct{}

func transform(b []byte, protect bool) ([]byte, error) {
	if len(b) == 0 {
		return nil, errors.New("empty protected data")
	}
	in := windows.DataBlob{Size: uint32(len(b)), Data: &b[0]}
	var out windows.DataBlob
	var e error
	if protect {
		e = windows.CryptProtectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	} else {
		e = windows.CryptUnprotectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	}
	if e != nil {
		return nil, errors.New("Windows credential protection failed")
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	return append([]byte(nil), unsafe.Slice(out.Data, out.Size)...), nil
}
func (Protector) Protect(b []byte) ([]byte, error)   { return transform(b, true) }
func (Protector) Unprotect(b []byte) ([]byte, error) { return transform(b, false) }
