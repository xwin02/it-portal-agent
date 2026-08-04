package security

import (
	"fmt"
	"syscall"
	"unsafe"
)

const cryptProtectUIForbidden = 0x1

var (
	crypt32            = syscall.NewLazyDLL("crypt32.dll")
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	cryptProtectData   = crypt32.NewProc("CryptProtectData")
	cryptUnprotectData = crypt32.NewProc("CryptUnprotectData")
	localFree          = kernel32.NewProc("LocalFree")
)

type dataBlob struct {
	Size uint32
	Data *byte
}

// ProtectSecret encrypts data for the current Windows account with DPAPI.
func ProtectSecret(value []byte) ([]byte, error) { return transform(value, cryptProtectData) }

// UnprotectSecret decrypts data encrypted by ProtectSecret for this account.
func UnprotectSecret(value []byte) ([]byte, error) { return transform(value, cryptUnprotectData) }

func transform(value []byte, procedure *syscall.LazyProc) ([]byte, error) {
	if len(value) == 0 {
		return nil, fmt.Errorf("secret is empty")
	}
	in := dataBlob{Size: uint32(len(value)), Data: &value[0]}
	var out dataBlob
	r1, _, callErr := procedure.Call(uintptr(unsafe.Pointer(&in)), 0, 0, 0, 0, cryptProtectUIForbidden, uintptr(unsafe.Pointer(&out)))
	if r1 == 0 {
		return nil, fmt.Errorf("Windows data protection operation failed: %w", callErr)
	}
	defer localFree.Call(uintptr(unsafe.Pointer(out.Data)))
	return append([]byte(nil), unsafe.Slice(out.Data, out.Size)...), nil
}
