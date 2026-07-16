//go:build windows

package cursor

import (
	"fmt"

	"golang.org/x/sys/windows"
)

var (
	user32              = windows.NewLazySystemDLL("user32.dll")
	procLoadImageW      = user32.NewProc("LoadImageW")
	procCopyImage       = user32.NewProc("CopyImage")
	procSetSystemCursor = user32.NewProc("SetSystemCursor")
	procSystemParamInfo = user32.NewProc("SystemParametersInfoW")
	procGetSystemMetric = user32.NewProc("GetSystemMetrics")
)

const (
	imageCursor   = 2      // IMAGE_CURSOR
	lrDefaultSize = 0x0040 // LR_DEFAULTSIZE
	lrShared      = 0x8000 // LR_SHARED
	smCXCursor    = 13     // SM_CXCURSOR
	smCYCursor    = 14     // SM_CYCURSOR
	spiSetCursors = 0x0057 // SPI_SETCURSORS
)

// Standard system cursor ids (OCR_*). Covers the common shapes; any id the
// system does not provide is skipped at runtime.
var systemCursorIDs = []uintptr{
	32512, // OCR_NORMAL
	32513, // OCR_IBEAM
	32514, // OCR_WAIT
	32515, // OCR_CROSS
	32516, // OCR_UP
	32642, // OCR_SIZENWSE
	32643, // OCR_SIZENESW
	32644, // OCR_SIZEWE
	32645, // OCR_SIZENS
	32646, // OCR_SIZEALL
	32648, // OCR_NO
	32649, // OCR_HAND
	32650, // OCR_APPSTARTING
	32651, // OCR_HELP
}

// Win32 enlarges the standard system cursors and restores the user's scheme.
type Win32 struct{ scale int }

// NewWin32 returns a Win32 enlarger with the given scale factor (e.g. 4).
func NewWin32(scale int) *Win32 {
	if scale < 1 {
		scale = 1
	}
	return &Win32{scale: scale}
}

func metric(index uintptr) int {
	r, _, _ := procGetSystemMetric.Call(index)
	if r == 0 {
		return 32
	}
	return int(r)
}

// Enlarge swaps every standard system cursor for a scaled copy. It loads the
// shared system cursor, makes an owned scaled copy with CopyImage, and hands
// that copy to SetSystemCursor (which takes ownership).
func (w *Win32) Enlarge() error {
	cx := uintptr(metric(smCXCursor) * w.scale)
	cy := uintptr(metric(smCYCursor) * w.scale)
	for _, id := range systemCursorIDs {
		hShared, _, _ := procLoadImageW.Call(0, id, imageCursor, 0, 0, lrShared|lrDefaultSize)
		if hShared == 0 {
			continue
		}
		hBig, _, _ := procCopyImage.Call(hShared, imageCursor, cx, cy, 0)
		if hBig == 0 {
			continue
		}
		procSetSystemCursor.Call(hBig, id)
	}
	return nil
}

// Restore reloads the user's configured cursor scheme.
func (w *Win32) Restore() error {
	r, _, err := procSystemParamInfo.Call(spiSetCursors, 0, 0, 0)
	if r == 0 {
		return fmt.Errorf("SPI_SETCURSORS failed: %w", err)
	}
	return nil
}
