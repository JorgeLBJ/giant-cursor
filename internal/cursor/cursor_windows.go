//go:build windows

package cursor

import (
	"fmt"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var (
	user32              = windows.NewLazySystemDLL("user32.dll")
	procSystemParamInfo = user32.NewProc("SystemParametersInfoW")
)

const (
	spiSetCursors  = 0x0057 // SPI_SETCURSORS
	spifUpdateIni  = 0x0001 // SPIF_UPDATEINIFILE
	spifSendChange = 0x0002 // SPIF_SENDCHANGE

	cursorsKeyPath = `Control Panel\Cursors`
	baseSizeValue  = "CursorBaseSize"
)

// CurrentBaseSize reads the user's current cursor base size in pixels. It
// defaults to DefaultBaseSize (32) when the value is missing.
func CurrentBaseSize() int {
	k, err := registry.OpenKey(registry.CURRENT_USER, cursorsKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return DefaultBaseSize
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue(baseSizeValue)
	if err != nil || v == 0 {
		return DefaultBaseSize
	}
	return int(v)
}

// Win32 enlarges the cursor using the native Windows cursor-size mechanism
// (the same one the accessibility "pointer size" slider uses), which renders
// crisply at any size instead of upscaling a small bitmap.
type Win32 struct {
	scale  int
	normal int // the user's normal base size, restored on Restore
}

// NewWin32 returns a Win32 enlarger. normalBaseSize is the size to return to on
// Restore (the user's own pointer size).
func NewWin32(scale, normalBaseSize int) *Win32 {
	if scale < 1 {
		scale = 1
	}
	if normalBaseSize < 1 {
		normalBaseSize = DefaultBaseSize
	}
	return &Win32{scale: scale, normal: normalBaseSize}
}

// Enlarge sets the cursor base size to normal*scale (clamped).
func (w *Win32) Enlarge() error {
	return applyBaseSize(EnlargedSize(w.normal, w.scale))
}

// Restore sets the cursor base size back to the user's normal size.
func (w *Win32) Restore() error {
	return applyBaseSize(w.normal)
}

// applyBaseSize writes CursorBaseSize and reloads cursors so the change takes
// effect immediately.
func applyBaseSize(px int) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, cursorsKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if err := k.SetDWordValue(baseSizeValue, uint32(px)); err != nil {
		return err
	}
	r, _, callErr := procSystemParamInfo.Call(spiSetCursors, 0, 0, spifUpdateIni|spifSendChange)
	if r == 0 {
		return fmt.Errorf("SPI_SETCURSORS failed: %w", callErr)
	}
	return nil
}
