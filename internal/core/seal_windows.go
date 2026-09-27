//go:build windows

package core

import (
	"bytes"
	"unsafe"

	"golang.org/x/sys/windows"
)

func seal(plain []byte) ([]byte, error) {
	if len(plain) == 0 {
		return []byte("PLY1"), nil
	}
	in := windows.DataBlob{Size: uint32(len(plain)), Data: &plain[0]}
	var out windows.DataBlob
	if err := windows.CryptProtectData(&in, nil, nil, 0, nil, 0, &out); err != nil {
		return nil, err
	}
	defer freeBlob(&out)
	raw := unsafe.Slice(out.Data, out.Size)
	buf := make([]byte, 4+len(raw))
	copy(buf, "PLY1")
	copy(buf[4:], raw)
	return buf, nil
}

func openSeal(b []byte) ([]byte, error) {
	if !bytes.HasPrefix(b, []byte("PLY1")) {
		return b, nil
	}
	payload := b[4:]
	if len(payload) == 0 {
		return nil, nil
	}
	in := windows.DataBlob{Size: uint32(len(payload)), Data: &payload[0]}
	var out windows.DataBlob
	if err := windows.CryptUnprotectData(&in, nil, nil, 0, nil, 0, &out); err != nil {
		return nil, err
	}
	defer freeBlob(&out)
	raw := unsafe.Slice(out.Data, out.Size)
	buf := make([]byte, len(raw))
	copy(buf, raw)
	return buf, nil
}

func freeBlob(b *windows.DataBlob) {
	if b == nil || b.Data == nil {
		return
	}
	_, _ = windows.LocalFree(windows.Handle(unsafe.Pointer(b.Data)))
	b.Data = nil
}
