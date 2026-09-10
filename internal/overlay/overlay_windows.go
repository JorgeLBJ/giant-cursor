//go:build windows

package overlay

import (
	"fmt"
	"image"
	"os"
	"runtime"
	"sync"
	"unsafe"

	"github.com/JorgeLBJ/giant-cursor/internal/config"
	"golang.org/x/sys/windows"
)

var (
	user32o   = windows.NewLazySystemDLL("user32.dll")
	kernel32o = windows.NewLazySystemDLL("kernel32.dll")
	gdi32o    = windows.NewLazySystemDLL("gdi32.dll")

	procRegisterClassO  = user32o.NewProc("RegisterClassW")
	procCreateWindowExO = user32o.NewProc("CreateWindowExW")
	procDefWindowProcO  = user32o.NewProc("DefWindowProcW")
	procGetMessageO     = user32o.NewProc("GetMessageW")
	procTranslateMsgO   = user32o.NewProc("TranslateMessage")
	procDispatchMsgO    = user32o.NewProc("DispatchMessageW")
	procShowWindowO     = user32o.NewProc("ShowWindow")
	procSetWindowPosO   = user32o.NewProc("SetWindowPos")
	procUpdateLayeredW  = user32o.NewProc("UpdateLayeredWindow")
	procGetDCO          = user32o.NewProc("GetDC")
	procReleaseDCO      = user32o.NewProc("ReleaseDC")

	procGetModuleHandleO = kernel32o.NewProc("GetModuleHandleW")

	procCreateCompatDCO = gdi32o.NewProc("CreateCompatibleDC")
	procDeleteDCO       = gdi32o.NewProc("DeleteDC")
	procCreateDIBSectO  = gdi32o.NewProc("CreateDIBSection")
	procSelectObjectO   = gdi32o.NewProc("SelectObject")
	procDeleteObjectO   = gdi32o.NewProc("DeleteObject")
)

const (
	// The extended styles are what make the window safe to draw over a game:
	// layered gives per-pixel alpha, transparent lets clicks fall through to
	// whatever is underneath, topmost keeps it above the game, tool-window
	// keeps it out of the Alt+Tab list, and no-activate stops it from ever
	// becoming the foreground window. None of them is optional.
	wsExLayered     = 0x00080000
	wsExTransparent = 0x00000020
	wsExTopmost     = 0x00000008
	wsExToolWindow  = 0x00000080
	wsExNoActivate  = 0x08000000
	wsPopup         = 0x80000000

	swHide   = 0
	swShowNA = 8 // show without activating: never steal focus from the game

	hwndTopmost = ^uintptr(0) // (HWND)-1

	swpNoSize     = 0x0001
	swpNoMove     = 0x0002
	swpNoZOrder   = 0x0004
	swpNoActivate = 0x0010

	ulwAlpha     = 0x00000002
	acSrcOver    = 0x00
	acSrcAlpha   = 0x01
	biRGB        = 0
	dibRGBColors = 0
)

// debugf reports overlay failures when GIANTCURSOR_DEBUG is set. The overlay is
// an enhancement: it must never take the app down with it.
func debugf(format string, a ...any) {
	if os.Getenv("GIANTCURSOR_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[giant-cursor overlay] "+format+"\n", a...)
	}
}

type wndClassO struct {
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
}

type msgO struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ X, Y int32 }
}

type bitmapInfoHeaderO struct {
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

type blendFunction struct {
	BlendOp             byte
	BlendFlags          byte
	SourceConstantAlpha byte
	AlphaFormat         byte
}

type pointO struct{ X, Y int32 }
type sizeO struct{ CX, CY int32 }

// Overlay is a click-through, always-on-top window that follows the pointer.
// It implements cursor.Enlarger and cursor.Tracker.
//
// It is created ONCE for the process. The controller rebuilds the cursor
// effector on every settings change, so an overlay created there would leak a
// window per menu click; mode and scale changes swap the painter instead.
type Overlay struct {
	mu      sync.Mutex
	painter Painter
	size    int // enlarged cursor size in pixels
	visible bool
	failed  bool // a creation failure disables the overlay for the session

	hwnd    uintptr
	edge    int // current window edge length
	anchorX int
	anchorY int

	ready chan struct{}
	once  sync.Once
}

// New returns an overlay with no painter, so it does nothing until SetMode
// selects one. The window is created lazily on the first Enlarge with a
// painter set, so config.OverlayOff really does cost nothing.
func New() *Overlay {
	return &Overlay{ready: make(chan struct{})}
}

// SetMode selects the painter, or none at all for config.OverlayOff. Switching
// to off hides the window immediately; switching between shapes discards the
// cached bitmap, which is redrawn on the next Enlarge.
func (o *Overlay) SetMode(mode config.OverlayMode) {
	o.mu.Lock()
	o.painter = PainterFor(mode)
	painter := o.painter
	o.mu.Unlock()
	if painter == nil {
		_ = o.Restore()
	}
}

// SetScale sets the enlarged cursor size the painters draw for.
func (o *Overlay) SetScale(cursorSize int) {
	o.mu.Lock()
	o.size = cursorSize
	o.mu.Unlock()
}

// Enlarge paints the overlay and shows it. The bitmap is built here, once per
// activation, so Track only has to move a window.
func (o *Overlay) Enlarge() error {
	o.mu.Lock()
	painter, size, failed := o.painter, o.size, o.failed
	o.mu.Unlock()

	if painter == nil || failed {
		return nil
	}
	if err := o.ensureWindow(); err != nil {
		o.mu.Lock()
		o.failed = true
		o.mu.Unlock()
		debugf("window creation failed, overlay disabled for this session: %v", err)
		return nil
	}

	hwnd := o.handle()
	img, ax, ay := painter.Draw(size)
	if err := o.paint(hwnd, img, ax, ay); err != nil {
		debugf("paint failed: %v", err)
		return nil
	}

	o.mu.Lock()
	o.visible = true
	o.mu.Unlock()
	procShowWindowO.Call(hwnd, swShowNA)
	return nil
}

// Restore hides the overlay. The window and its bitmap are kept for reuse.
func (o *Overlay) Restore() error {
	o.mu.Lock()
	hwnd, visible := o.hwnd, o.visible
	o.visible = false
	o.mu.Unlock()

	if hwnd != 0 && visible {
		procShowWindowO.Call(hwnd, swHide)
	}
	return nil
}

// Track centres the window on the pointer. Nothing is repainted here: at 125
// samples per second, moving a window is cheap and repainting one is not.
func (o *Overlay) Track(x, y int) {
	o.mu.Lock()
	hwnd, visible, ax, ay := o.hwnd, o.visible, o.anchorX, o.anchorY
	o.mu.Unlock()

	if hwnd == 0 || !visible {
		return
	}
	procSetWindowPosO.Call(
		hwnd, 0,
		uintptr(int32(x-ax)), uintptr(int32(y-ay)), 0, 0,
		swpNoActivate|swpNoZOrder|swpNoSize,
	)
}

// handle returns the window handle, or 0 before the window exists. Every read
// of o.hwnd goes through the mutex that guards its one write.
func (o *Overlay) handle() uintptr {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.hwnd
}

// ensureWindow starts the overlay's own thread and message pump on first use.
// Win32 windows are thread-affine and the tray owns the only other pump, so the
// overlay keeps its own; that way its lifetime never entangles with the tray's.
func (o *Overlay) ensureWindow() error {
	var err error
	o.once.Do(func() {
		started := make(chan error, 1)
		go o.run(started)
		err = <-started
	})
	if err != nil {
		return err
	}
	if o.handle() == 0 {
		return fmt.Errorf("overlay window was not created")
	}
	return nil
}

func (o *Overlay) run(started chan<- error) {
	// The window and its message pump must stay on one OS thread for the life
	// of the process, so this goroutine never returns the thread to the runtime.
	runtime.LockOSThread()

	hinst, _, _ := procGetModuleHandleO.Call(0)
	className, _ := windows.UTF16PtrFromString("GiantCursorOverlayWnd")

	wc := wndClassO{
		lpfnWndProc:   windows.NewCallback(overlayWndProc),
		hInstance:     hinst,
		lpszClassName: className,
	}
	if atom, _, e := procRegisterClassO.Call(uintptr(unsafe.Pointer(&wc))); atom == 0 {
		started <- fmt.Errorf("RegisterClass failed: %w", e)
		return
	}

	hwnd, _, e := procCreateWindowExO.Call(
		wsExLayered|wsExTransparent|wsExTopmost|wsExToolWindow|wsExNoActivate,
		uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(className)),
		wsPopup,
		0, 0, 1, 1,
		0, 0, hinst, 0,
	)
	if hwnd == 0 {
		started <- fmt.Errorf("CreateWindowEx failed: %w", e)
		return
	}
	o.mu.Lock()
	o.hwnd = hwnd
	o.mu.Unlock()
	started <- nil

	var m msgO
	for {
		r, _, _ := procGetMessageO.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			return
		}
		procTranslateMsgO.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMsgO.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func overlayWndProc(hwnd, msg, wparam, lparam uintptr) uintptr {
	r, _, _ := procDefWindowProcO.Call(hwnd, msg, wparam, lparam)
	return r
}

// paint pushes an image into the layered window with per-pixel alpha.
func (o *Overlay) paint(hwnd uintptr, img *image.RGBA, anchorX, anchorY int) error {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 1 || h < 1 {
		return fmt.Errorf("refusing to paint an empty image")
	}

	screenDC, _, _ := procGetDCO.Call(0)
	if screenDC == 0 {
		return fmt.Errorf("GetDC failed")
	}
	defer procReleaseDCO.Call(0, screenDC)

	memDC, _, _ := procCreateCompatDCO.Call(screenDC)
	if memDC == 0 {
		return fmt.Errorf("CreateCompatibleDC failed")
	}
	defer procDeleteDCO.Call(memDC)

	// Negative height makes the DIB top-down, matching image.RGBA's row order.
	bi := bitmapInfoHeaderO{
		Size:        uint32(unsafe.Sizeof(bitmapInfoHeaderO{})),
		Width:       int32(w),
		Height:      int32(-h),
		Planes:      1,
		BitCount:    32,
		Compression: biRGB,
	}
	var bits unsafe.Pointer
	dib, _, _ := procCreateDIBSectO.Call(
		memDC, uintptr(unsafe.Pointer(&bi)), dibRGBColors,
		uintptr(unsafe.Pointer(&bits)), 0, 0,
	)
	if dib == 0 || bits == nil {
		return fmt.Errorf("CreateDIBSection failed")
	}
	defer procDeleteObjectO.Call(dib)

	// image.RGBA is premultiplied RGBA; Windows wants premultiplied BGRA.
	dst := unsafe.Slice((*byte)(bits), w*h*4)
	for i := 0; i < w*h; i++ {
		dst[i*4+0] = img.Pix[i*4+2] // B
		dst[i*4+1] = img.Pix[i*4+1] // G
		dst[i*4+2] = img.Pix[i*4+0] // R
		dst[i*4+3] = img.Pix[i*4+3] // A
	}

	old, _, _ := procSelectObjectO.Call(memDC, dib)
	defer procSelectObjectO.Call(memDC, old)

	size := sizeO{CX: int32(w), CY: int32(h)}
	src := pointO{}
	blend := blendFunction{BlendOp: acSrcOver, SourceConstantAlpha: 255, AlphaFormat: acSrcAlpha}

	r, _, e := procUpdateLayeredW.Call(
		hwnd, screenDC, 0,
		uintptr(unsafe.Pointer(&size)),
		memDC, uintptr(unsafe.Pointer(&src)),
		0, uintptr(unsafe.Pointer(&blend)), ulwAlpha,
	)
	if r == 0 {
		return fmt.Errorf("UpdateLayeredWindow failed: %w", e)
	}

	o.mu.Lock()
	o.edge, o.anchorX, o.anchorY = w, anchorX, anchorY
	o.mu.Unlock()
	// Keep the window on top: a game going fullscreen-windowed can push it down.
	procSetWindowPosO.Call(hwnd, hwndTopmost, 0, 0, 0, 0, swpNoActivate|swpNoSize|swpNoMove)
	return nil
}
