//go:build windows

package lifecycle

import "golang.org/x/sys/windows"

// AcquireSingleInstance creates a named mutex. If it already exists, already
// is true and the caller should exit. Otherwise release must be called on exit.
func AcquireSingleInstance(name string) (release func(), already bool, err error) {
	ptr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, false, err
	}
	h, err := windows.CreateMutex(nil, false, ptr)
	if h == 0 {
		return nil, false, err
	}
	if err == windows.ERROR_ALREADY_EXISTS {
		windows.CloseHandle(h)
		return nil, true, nil
	}
	return func() { windows.CloseHandle(h) }, false, nil
}
