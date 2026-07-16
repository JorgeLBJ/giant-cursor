//go:build windows

package cursor

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// debug prints diagnostics to stderr when GIANTCURSOR_DEBUG is set.
var debug = os.Getenv("GIANTCURSOR_DEBUG") != ""

func dbg(format string, a ...any) {
	if debug {
		fmt.Fprintf(os.Stderr, "[giant-cursor] "+format+"\n", a...)
	}
}

var (
	user32              = windows.NewLazySystemDLL("user32.dll")
	procLoadImageW      = user32.NewProc("LoadImageW")
	procCopyImage       = user32.NewProc("CopyImage")
	procSetSystemCursor = user32.NewProc("SetSystemCursor")
	procSystemParamInfo = user32.NewProc("SystemParametersInfoW")
	procGetSystemMetric = user32.NewProc("GetSystemMetrics")
	procCreateIconIndir = user32.NewProc("CreateIconIndirect")
	procSendMsgTimeout  = user32.NewProc("SendMessageTimeoutW")

	gdi32             = windows.NewLazySystemDLL("gdi32.dll")
	procCreateDIBSect = gdi32.NewProc("CreateDIBSection")
	procCreateBitmap  = gdi32.NewProc("CreateBitmap")
	procDeleteObject  = gdi32.NewProc("DeleteObject")
)

const (
	imageCursor    = 2      // IMAGE_CURSOR
	lrDefaultColor = 0x0000 // LR_DEFAULTCOLOR
	lrDefaultSize  = 0x0040 // LR_DEFAULTSIZE
	lrShared       = 0x8000 // LR_SHARED

	smCXCursor = 13 // SM_CXCURSOR
	smCYCursor = 14 // SM_CYCURSOR

	spiSetCursors  = 0x0057 // SPI_SETCURSORS
	spifUpdateIni  = 0x0001 // SPIF_UPDATEINIFILE
	spifSendChange = 0x0002 // SPIF_SENDCHANGE

	wmSettingChange = 0x001A // WM_SETTINGCHANGE
	hwndBroadcast   = 0xFFFF // HWND_BROADCAST
	smtoAbortIfHung = 0x0002 // SMTO_ABORTIFHUNG

	cursorsKeyPath = `Control Panel\Cursors`
	baseSizeValue  = "CursorBaseSize"

	ocrNormal = 32512 // OCR_NORMAL (the arrow)
)

type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

type iconInfo struct {
	fIcon    int32
	xHotspot uint32
	yHotspot uint32
	hbmMask  uintptr
	hbmColor uintptr
}

// Standard system cursor ids (OCR_*). Any id the system does not provide is
// skipped at runtime.
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

// CurrentBaseSize reads the user's cursor base size in pixels, defaulting to 32.
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

// Win32 enlarges the cursor. The mechanism depends on the chosen Style:
// crisp/system use SetSystemCursor (reliable live apply); native uses the
// Windows cursor base size (crisp where the OS applies it live).
type Win32 struct {
	scale  int
	normal int // user's normal cursor base size, restored in native mode
	style  Style
}

// NewWin32 returns a Win32 enlarger for the given scale, the user's normal
// cursor base size (used by native mode), and the enlargement style.
func NewWin32(scale, normalBaseSize int, style Style) *Win32 {
	if scale < 1 {
		scale = 1
	}
	if normalBaseSize < 1 {
		normalBaseSize = DefaultBaseSize
	}
	if style == "" {
		style = StyleCrisp
	}
	return &Win32{scale: scale, normal: normalBaseSize, style: style}
}

func metric(index uintptr) int {
	r, _, _ := procGetSystemMetric.Call(index)
	if r == 0 {
		return DefaultBaseSize
	}
	return int(r)
}

// Enlarge enlarges the cursor according to the configured style.
func (w *Win32) Enlarge() error {
	if w.style == StyleNative {
		return applyNativeSize(EnlargedSize(w.normal, w.scale))
	}
	cx := EnlargedSize(metric(smCXCursor), w.scale)
	cy := EnlargedSize(metric(smCYCursor), w.scale)
	dbg("Enlarge style=%s scale=%d -> %dx%d", w.style, w.scale, cx, cy)
	n := 0
	for _, id := range systemCursorIDs {
		var h uintptr
		if w.style == StyleCrisp && id == ocrNormal {
			h = makeArrowCursor(cx) // custom crisp high-contrast arrow
		} else {
			h = loadBigCursor(id, cx, cy) // upscaled system cursor
		}
		if h == 0 {
			continue
		}
		procSetSystemCursor.Call(h, id) // takes ownership of h
		n++
	}
	dbg("SetSystemCursor applied to %d cursors", n)
	return nil
}

// applyNativeSize writes CursorBaseSize and asks Windows to re-apply it live,
// via SystemParametersInfo and a WM_SETTINGCHANGE broadcast.
func applyNativeSize(px int) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, cursorsKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if err := k.SetDWordValue(baseSizeValue, uint32(px)); err != nil {
		return err
	}
	r1, _, _ := procSystemParamInfo.Call(spiSetCursors, uintptr(px), 0, spifUpdateIni|spifSendChange)
	section, _ := windows.UTF16PtrFromString("Cursors")
	var res uintptr
	r2, _, _ := procSendMsgTimeout.Call(hwndBroadcast, wmSettingChange, 0,
		uintptr(unsafe.Pointer(section)), smtoAbortIfHung, 500, uintptr(unsafe.Pointer(&res)))
	dbg("native size px=%d spi=%d bcast=%d readback=%dpx", px, r1, r2, CurrentBaseSize())
	return nil
}

// makeArrowCursor builds a crisp, high-contrast arrow HCURSOR at the given size
// from a self-rendered bitmap (hotspot at the tip). Returns 0 on failure.
func makeArrowCursor(size int) uintptr {
	bgra := rasterizeArrow(size)

	bi := bitmapInfoHeader{
		Size:     40,
		Width:    int32(size),
		Height:   -int32(size), // negative = top-down
		Planes:   1,
		BitCount: 32,
	}
	var bits unsafe.Pointer
	dib, _, _ := procCreateDIBSect.Call(0, uintptr(unsafe.Pointer(&bi)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if dib == 0 || bits == nil {
		dbg("CreateDIBSection failed")
		return 0
	}
	copy(unsafe.Slice((*byte)(bits), len(bgra)), bgra)

	maskStride := ((size + 15) / 16) * 2
	maskBits := make([]byte, maskStride*size) // all zero = fully opaque (alpha rules)
	mask, _, _ := procCreateBitmap.Call(uintptr(size), uintptr(size), 1, 1, uintptr(unsafe.Pointer(&maskBits[0])))

	ii := iconInfo{fIcon: 0, hbmMask: mask, hbmColor: dib}
	hcur, _, _ := procCreateIconIndir.Call(uintptr(unsafe.Pointer(&ii)))

	procDeleteObject.Call(dib)
	procDeleteObject.Call(mask)
	if hcur == 0 {
		dbg("CreateIconIndirect failed")
	}
	return hcur
}

// loadBigCursor returns an owned enlarged cursor handle. It first tries to load
// the system cursor directly at the target size (crisper when Windows has a
// high-resolution source), and falls back to upscaling the default size.
func loadBigCursor(id uintptr, cx, cy int) uintptr {
	if h, _, _ := procLoadImageW.Call(0, id, imageCursor, uintptr(cx), uintptr(cy), lrDefaultColor); h != 0 {
		return h
	}
	shared, _, _ := procLoadImageW.Call(0, id, imageCursor, 0, 0, lrShared|lrDefaultSize)
	if shared == 0 {
		return 0
	}
	big, _, _ := procCopyImage.Call(shared, imageCursor, uintptr(cx), uintptr(cy), 0)
	return big
}

// Restore returns the cursor to normal. Native mode resets the base size;
// crisp/system reload the user's cursor scheme to undo the SetSystemCursor swap.
func (w *Win32) Restore() error {
	if w.style == StyleNative {
		return applyNativeSize(w.normal)
	}
	r, _, callErr := procSystemParamInfo.Call(spiSetCursors, 0, 0, spifSendChange)
	dbg("Restore SPI_SETCURSORS ret=%d err=%v", r, callErr)
	// SPI_SETCURSORS reloads the scheme even when it reports 0, so do not fail.
	return nil
}
