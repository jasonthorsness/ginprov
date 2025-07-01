package oshelper

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"syscall"
	"unsafe"
)

var ErrRenameError = errors.New("rename in root error")

// RenameInRoot can be deleted once we upgrade to Go 1.25 which has root.Rename.
func RenameInRoot(root *os.Root, from, to string) error {
	if from == to {
		return nil
	}

	fd, err := getRootFD(root)
	if err != nil {
		return err
	}

	fromBytes, err := syscall.BytePtrFromString(from)
	if err != nil {
		return fmt.Errorf("BytePtrFromString failed: %w", err)
	}

	toBytes, err := syscall.BytePtrFromString(to)
	if err != nil {
		return fmt.Errorf("BytePtrFromString failed: %w", err)
	}

	_, _, errno := syscall.Syscall6(
		syscall.SYS_RENAMEAT,
		uintptr(fd), uintptr(unsafe.Pointer(fromBytes)), //nolint:gosec
		uintptr(fd), uintptr(unsafe.Pointer(toBytes)), //nolint:gosec
		0, 0,
	)
	if errno != 0 {
		return fmt.Errorf("failed to rename %s to %s: %w", from, to, errno)
	}

	return nil
}

func getRootFD(root *os.Root) (int, error) {
	rootVal := reflect.ValueOf(root)
	if rootVal.Kind() != reflect.Ptr || rootVal.Elem().Kind() != reflect.Struct {
		return 0, fmt.Errorf("%w: root must be pointer-to-struct, got %T", ErrRenameError, root)
	}

	structVal := rootVal.Elem()

	innerPtr := structVal.FieldByName("root")
	if !innerPtr.IsValid() {
		return 0, fmt.Errorf("%w: no inner root field on %T", ErrRenameError, root)
	}

	if innerPtr.IsNil() {
		return 0, fmt.Errorf("%w: inner root pointer is nil", ErrRenameError)
	}

	innerStruct := innerPtr.Elem()

	fdField := innerStruct.FieldByName("fd")
	if !fdField.IsValid() {
		return 0, fmt.Errorf("%w: no fd field on inner root %T", ErrRenameError, innerStruct.Interface())
	}

	if fdField.Kind() != reflect.Int {
		return 0, fmt.Errorf("%w: fd field is not an int, got %s", ErrRenameError, fdField.Kind())
	}

	return int(fdField.Int()), nil
}
