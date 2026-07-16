//go:build windows

package input

import (
	"fmt"
	"unsafe"

	"giant-cursor/internal/shake"
	"golang.org/x/sys/windows"
)

var (
	user32           = windows.NewLazySystemDLL("user32.dll")
	procGetCursorPos = user32.NewProc("GetCursorPos")
)

type winPoint struct{ X, Y int32 }

// Win32Poller reads the cursor position via GetCursorPos.
type Win32Poller struct{}

// NewWin32Poller returns a GetCursorPos-based position source.
func NewWin32Poller() *Win32Poller { return &Win32Poller{} }

// Poll returns the current cursor position in screen coordinates.
func (p *Win32Poller) Poll() (shake.Point, error) {
	var pt winPoint
	r, _, err := procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	if r == 0 {
		return shake.Point{}, fmt.Errorf("GetCursorPos failed: %w", err)
	}
	return shake.Point{X: pt.X, Y: pt.Y}, nil
}
