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
	procPostMessageO    = user32o.NewProc("PostMessageW")
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

	// wmOverlayPaint asks the overlay's own thread to push the art waiting on
	// o.paints into the layered window. WM_APP is the range Windows reserves
	// for an application's private messages, so it can never collide with a
	// system message this window might receive.
	wmOverlayPaint = 0x8000 + 1 // WM_APP + 1
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

	// showPending defers the reveal to the first Track of an activation, so
	// the window is never shown before it has a position. Without it the very
	// first activation of the session presents the art at (0,0) — the corner
	// the window was created at — for however long it takes the next sample to
	// arrive. internal/app calls Track immediately after Enlarge on the same
	// sample, so the delay is not observable.
	showPending bool

	hwnd    uintptr
	anchorX int
	anchorY int

	// paints hands finished art from Enlarge to the overlay's own thread,
	// which is the only thread allowed to touch a device context (see paint).
	// It is buffered and only ever written by an Enlarge holding o.mu, so the
	// send never blocks, and the overlay thread only ever reads it without
	// blocking. Nothing on either side can wait for the other.
	paints chan *image.RGBA

	once sync.Once
}

// New returns an overlay with no painter, so it does nothing until SetMode
// selects one. The window is created lazily on the first Enlarge with a
// painter set, so config.OverlayOff really does cost nothing.
func New() *Overlay {
	return &Overlay{paints: make(chan *image.RGBA, 1)}
}

// SetMode selects the painter, or none at all for config.OverlayOff. Switching
// to off hides the window immediately. Switching between shapes changes nothing
// on screen until the next Enlarge, which is where the art is drawn.
//
// The design document says selecting off "hides and destroys the window
// immediately". It is deliberately only hidden here. The window is created once
// per process precisely because destroying and recreating it costs a window
// class, a thread and a message pump on every menu click, and the overlay's own
// thread is locked for the life of the process anyway; a hidden layered window
// that is never painted composites nothing and costs nothing. The observable
// behaviour the document cares about — nothing on screen after selecting off —
// is what Restore delivers.
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

// Enlarge draws the art and hands it to the overlay's own thread to push into
// the window. The bitmap is built here, once per activation, so Track only has
// to move a window.
//
// Drawing happens on the calling goroutine because it is pure Go; only the GDI
// half is thread-affine. Windows requires a device context to be released by
// the thread that acquired it, and a memory DC dies with the thread that
// created it, so the GetDC/CreateCompatibleDC work in paint cannot run on the
// polling goroutine — the runtime is free to resume that goroutine on a
// different OS thread across a blocking syscall, and the deferred releases
// would then be issued from the wrong thread. That leaks a DC per activation
// until the process runs out of them and every paint silently fails.
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

	// One critical section, because a Restore that lands anywhere inside it
	// must either be fully overtaken by this activation or fully overtake it.
	// Splitting it would let a Restore hide the window and clear the flags
	// only to have this call set them again, leaving the overlay on screen
	// with nothing able to hide it. PostMessage is safe to hold the lock
	// across: it appends to the target queue and returns, and never waits for
	// the overlay thread.
	o.mu.Lock()
	// Drop art the overlay thread has not picked up yet: it is stale by
	// definition, and this is the only sender, so the send below cannot block.
	select {
	case <-o.paints:
	default:
	}
	o.paints <- img
	posted, _, e := procPostMessageO.Call(hwnd, wmOverlayPaint, 0, 0)
	if posted != 0 {
		o.anchorX, o.anchorY = ax, ay
		o.visible = true
		o.showPending = true
	} else {
		// Take the art back rather than leave it for a message that will never
		// arrive. The receive must not block: an earlier message may already
		// have carried this art away.
		select {
		case <-o.paints:
		default:
		}
	}
	o.mu.Unlock()

	if posted == 0 {
		debugf("posting the paint request failed, overlay stays hidden: %v", e)
	}
	return nil
}

// Restore hides the overlay. The window is kept for reuse.
//
// It hides unconditionally whenever a window exists, rather than only when it
// believes the window is visible: cursor.Enlarger documents that Restore must
// be total, because a stuck enlarged cursor is the worst failure this app can
// produce, and a redundant SW_HIDE costs nothing.
func (o *Overlay) Restore() error {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.visible = false
	// Clearing the latch under the same lock is what stops a reveal that was
	// armed by an Enlarge from landing after this hide and putting the art
	// back on screen with nothing left to take it off again.
	o.showPending = false
	if o.hwnd != 0 {
		procShowWindowO.Call(o.hwnd, swHide)
	}
	return nil
}

// Track centres the window on the pointer, and reveals it on the first call
// after an Enlarge. Nothing is repainted here: at 125 samples per second,
// moving a window is cheap and repainting one is not.
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

	// The window now has a position, so it is safe to show. Testing the latch
	// and showing must happen in one critical section, for the same reason
	// Restore clears the latch inside one: a Restore in between would hide a
	// window this call is about to show, and the show would win.
	o.mu.Lock()
	if o.showPending && o.visible {
		o.showPending = false
		procShowWindowO.Call(hwnd, swShowNA)
	}
	o.mu.Unlock()
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
	// This is the only place the overlay's own thread takes o.mu, and it is
	// safe because it happens before any other thread can reach this window: no
	// caller has the handle yet, and ensureWindow is still blocked on started.
	// Once the pump below is running, nothing on this thread touches o.mu — see
	// overlayWndProc for what depends on that.
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
		// Paint requests are handled here, in the loop, rather than in
		// overlayWndProc. The window procedure also runs on this thread, but it
		// runs for messages SENT from other threads too, and those callers are
		// blocked while it runs; the loop only ever runs for messages this
		// thread has already dequeued, with nobody waiting on it. Servicing the
		// paint here is what lets it touch overlay state at all.
		if m.message == wmOverlayPaint {
			o.servicePaint(hwnd)
			continue
		}
		procTranslateMsgO.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMsgO.Call(uintptr(unsafe.Pointer(&m)))
	}
}

// servicePaint pushes the art Enlarge left on o.paints into the window. It runs
// only on the overlay's own locked thread, which is what makes the device
// contexts in paint legal, and it never takes o.mu — see overlayWndProc.
func (o *Overlay) servicePaint(hwnd uintptr) {
	var img *image.RGBA
	select {
	case img = <-o.paints:
	default:
		// An Enlarge took its art back after posting, or a later Enlarge
		// replaced it and its own message is still queued behind this one.
		return
	}
	if err := o.paint(hwnd, img); err != nil {
		debugf("paint failed: %v", err)
	}
}

// overlayWndProc must never touch o.mu, and neither must anything it calls.
//
// A thread blocked in a cross-thread ShowWindow or SetWindowPos does not sit
// idle: Windows delivers the work to the target window's thread and runs it
// there, inside its window procedure, while the caller waits. Enlarge, Restore
// and Track all make those calls while holding o.mu. If this procedure took
// o.mu it would block on the very caller that is blocked on it, and the two
// threads would wedge for the life of the process — a frozen ring on screen in
// the best case. Work that needs overlay state is posted to the message loop
// instead, where nobody is waiting (see run and servicePaint).
func overlayWndProc(hwnd, msg, wparam, lparam uintptr) uintptr {
	r, _, _ := procDefWindowProcO.Call(hwnd, msg, wparam, lparam)
	return r
}

// paint pushes an image into the layered window with per-pixel alpha.
//
// It must run on the overlay's own locked thread. Windows caches common device
// contexts per thread and requires ReleaseDC from the thread that called GetDC,
// and a CreateCompatibleDC context is destroyed with the thread that created
// it. UpdateLayeredWindow is a blocking cross-thread syscall, so on any
// unlocked goroutine the Go runtime may resume the deferred releases below on a
// different OS thread than the one that acquired the contexts.
func (o *Overlay) paint(hwnd uintptr, img *image.RGBA) error {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 1 || h < 1 {
		return fmt.Errorf("refusing to paint an empty image")
	}
	// The copy loop below walks img.Pix as one tightly packed run of w*h pixels,
	// which only holds for a whole image. A sub-image sharing a larger backing
	// array has Stride > w*4, and the loop would read the wrong pixels and then
	// run off the end. Both painters return whole images today; refusing here
	// turns a future out-of-range panic into a disabled overlay.
	if img.Stride != w*4 || len(img.Pix) < w*h*4 {
		return fmt.Errorf("unexpected image layout: stride %d and %d bytes for %dx%d", img.Stride, len(img.Pix), w, h)
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

	// Keep the window on top: a game going fullscreen-windowed can push it down.
	procSetWindowPosO.Call(hwnd, hwndTopmost, 0, 0, 0, 0, swpNoActivate|swpNoSize|swpNoMove)
	return nil
}
