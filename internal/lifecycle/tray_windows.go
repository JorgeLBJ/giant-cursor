//go:build windows

package lifecycle

import (
	"encoding/binary"
	"fmt"
	"unsafe"

	"github.com/JorgeLBJ/giant-cursor/internal/i18n"
	"golang.org/x/sys/windows"
)

var (
	user32t  = windows.NewLazySystemDLL("user32.dll")
	kernel32t = windows.NewLazySystemDLL("kernel32.dll")
	shell32t = windows.NewLazySystemDLL("shell32.dll")

	procRegisterClass    = user32t.NewProc("RegisterClassW")
	procCreateWindowEx   = user32t.NewProc("CreateWindowExW")
	procDefWindowProc    = user32t.NewProc("DefWindowProcW")
	procDestroyWindow    = user32t.NewProc("DestroyWindow")
	procLoadIcon         = user32t.NewProc("LoadIconW")
	procCreateIconFromRes = user32t.NewProc("CreateIconFromResourceEx")
	procGetModuleHandle  = kernel32t.NewProc("GetModuleHandleW")
	procCreatePopupMenu  = user32t.NewProc("CreatePopupMenu")
	procAppendMenu       = user32t.NewProc("AppendMenuW")
	procTrackPopupMenu   = user32t.NewProc("TrackPopupMenu")
	procDestroyMenu      = user32t.NewProc("DestroyMenu")
	procSetForeground    = user32t.NewProc("SetForegroundWindow")
	procGetCursorPosT    = user32t.NewProc("GetCursorPos")
	procPostMessage      = user32t.NewProc("PostMessageW")
	procPostQuitMessage  = user32t.NewProc("PostQuitMessage")
	procGetMessageT      = user32t.NewProc("GetMessageW")
	procTranslateMsgT    = user32t.NewProc("TranslateMessage")
	procDispatchMsgT     = user32t.NewProc("DispatchMessageW")
	procShellNotifyIcon  = shell32t.NewProc("Shell_NotifyIconW")
)

const (
	wmDestroy      = 0x0002
	wmTrayCallback = 0x8001 // WM_APP + 1
	wmRButtonUp    = 0x0205
	wmLButtonUp    = 0x0202
	wmContextMenu  = 0x007B

	mfString    = 0x0000
	mfPopup     = 0x0010
	mfChecked   = 0x0008
	mfSeparator = 0x0800

	tpmRightButton = 0x0002
	tpmReturnCmd   = 0x0100
	tpmNonNotify   = 0x0080

	nimAdd     = 0x0000
	nimDelete  = 0x0002
	nifMessage = 0x0001
	nifIcon    = 0x0002
	nifTip     = 0x0004

	idiApplication = 32512

	// Menu command id ranges.
	idScaleBase       = 100 // idScaleBase + scale
	idSensBase        = 200 // idSensBase + index
	idHoldBase        = 300 // idHoldBase + index
	idToggleAutostart = 400
	idStyleBase       = 500 // idStyleBase + index
	idLangBase        = 600 // idLangBase + index
	idQuit            = 900
)

// TrayCallbacks describe the menu contents and the actions for each item. The
// value lists carry the stable option values; labels come from Strings so the
// menu can be shown in the user's language.
type TrayCallbacks struct {
	Strings func() i18n.Strings
	IconICO []byte // raw .ico bytes for the tray icon (falls back to a stock icon)

	Styles        []string // e.g. ["crisp","system"]
	Scales        []int    // e.g. [2,3,4,5,6,8]
	Sensitivities []string // e.g. ["low","medium","high"]
	Holds         []int64  // e.g. [700,1000,1500]
	Langs         []string // e.g. ["en","es"]

	CurrentStyle       func() string
	CurrentScale       func() int
	CurrentSensitivity func() string
	CurrentHold        func() int64
	CurrentLang        func() string
	AutostartOn        func() bool

	OnStyle           func(value string)
	OnScale           func(scale int)
	OnSensitivity     func(name string)
	OnHold            func(ms int64)
	OnLang            func(code string)
	OnToggleAutostart func()
	OnQuit            func()
}

type wndClass struct {
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

type notifyIconData struct {
	cbSize            uint32
	hWnd              uintptr
	uID               uint32
	uFlags            uint32
	uCallbackMessage  uint32
	hIcon             uintptr
	szTip             [128]uint16
	dwState           uint32
	dwStateMask       uint32
	szInfo            [256]uint16
	uVersionOrTimeout uint32
	szInfoTitle       [64]uint16
	dwInfoFlags       uint32
	guidItem          windows.GUID
	hBalloonIcon      uintptr
}

type trayMsg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ X, Y int32 }
}

// Package-level state; RunTray is called exactly once (single instance).
var (
	trayCB   TrayCallbacks
	trayHwnd uintptr
	trayNID  notifyIconData
)

// RunTray creates the tray icon and runs the message loop until the user quits.
// Call runtime.LockOSThread() on the calling goroutine first.
func RunTray(cb TrayCallbacks) error {
	trayCB = cb

	hinst, _, _ := procGetModuleHandle.Call(0)
	className, _ := windows.UTF16PtrFromString("GiantCursorTrayWnd")

	wc := wndClass{
		lpfnWndProc:   windows.NewCallback(wndProc),
		hInstance:     hinst,
		lpszClassName: className,
	}
	if atom, _, err := procRegisterClass.Call(uintptr(unsafe.Pointer(&wc))); atom == 0 {
		return fmt.Errorf("RegisterClass failed: %w", err)
	}

	hwnd, _, err := procCreateWindowEx.Call(
		0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(className)),
		0, 0, 0, 0, 0, 0, 0, hinst, 0,
	)
	if hwnd == 0 {
		return fmt.Errorf("CreateWindowEx failed: %w", err)
	}
	trayHwnd = hwnd

	hicon := iconFromICO(trayCB.IconICO)
	if hicon == 0 {
		hicon, _, _ = procLoadIcon.Call(0, idiApplication)
	}

	trayNID = notifyIconData{
		hWnd:             hwnd,
		uID:              1,
		uFlags:           nifMessage | nifIcon | nifTip,
		uCallbackMessage: wmTrayCallback,
		hIcon:            hicon,
	}
	trayNID.cbSize = uint32(unsafe.Sizeof(trayNID))
	copyUTF16(trayNID.szTip[:], "Giant Cursor — shake the mouse to enlarge")
	procShellNotifyIcon.Call(nimAdd, uintptr(unsafe.Pointer(&trayNID)))

	var m trayMsg
	for {
		r, _, _ := procGetMessageT.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		procTranslateMsgT.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMsgT.Call(uintptr(unsafe.Pointer(&m)))
	}

	procShellNotifyIcon.Call(nimDelete, uintptr(unsafe.Pointer(&trayNID)))
	return nil
}

// iconFromICO creates an HICON from raw .ico bytes, picking the entry closest
// to a small-icon size. Returns 0 on any problem so the caller can fall back.
func iconFromICO(data []byte) uintptr {
	const want = 32
	if len(data) < 6 {
		return 0
	}
	count := int(binary.LittleEndian.Uint16(data[4:]))
	best, bestDiff := -1, 1<<30
	for i := 0; i < count; i++ {
		e := 6 + i*16
		if e+16 > len(data) {
			break
		}
		w := int(data[e])
		if w == 0 {
			w = 256
		}
		if d := abs(w - want); d < bestDiff {
			bestDiff, best = d, i
		}
	}
	if best < 0 {
		return 0
	}
	e := 6 + best*16
	size := binary.LittleEndian.Uint32(data[e+8:])
	offset := binary.LittleEndian.Uint32(data[e+12:])
	if offset == 0 || size == 0 || int(offset+size) > len(data) {
		return 0
	}
	img := data[offset : offset+size]
	h, _, _ := procCreateIconFromRes.Call(
		uintptr(unsafe.Pointer(&img[0])), uintptr(len(img)),
		1, 0x00030000, want, want, 0,
	)
	return h
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func wndProc(hwnd, msg, wparam, lparam uintptr) uintptr {
	switch msg {
	case wmTrayCallback:
		switch lparam & 0xffff {
		case wmRButtonUp, wmLButtonUp, wmContextMenu:
			showMenu(hwnd)
		}
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProc.Call(hwnd, msg, wparam, lparam)
	return r
}

func createSub() uintptr {
	m, _, _ := procCreatePopupMenu.Call()
	return m
}

func checkFlag(on bool) uintptr {
	if on {
		return mfString | mfChecked
	}
	return mfString
}

func labelAt(labels []string, i int, fallback string) string {
	if i >= 0 && i < len(labels) {
		return labels[i]
	}
	return fallback
}

func showMenu(hwnd uintptr) {
	s := trayCB.Strings()
	menu, _, _ := procCreatePopupMenu.Call()

	styleMenu := createSub()
	for i, v := range trayCB.Styles {
		appendMenu(styleMenu, checkFlag(v == trayCB.CurrentStyle()), uintptr(idStyleBase+i), labelAt(s.Styles, i, v))
	}
	appendMenu(menu, mfString|mfPopup, styleMenu, s.MenuCursor)

	scaleMenu := createSub()
	curScale := trayCB.CurrentScale()
	for _, sc := range trayCB.Scales {
		appendMenu(scaleMenu, checkFlag(sc == curScale), uintptr(idScaleBase+sc), fmt.Sprintf("%dx", sc))
	}
	appendMenu(menu, mfString|mfPopup, scaleMenu, s.MenuSize)

	sensMenu := createSub()
	for i, v := range trayCB.Sensitivities {
		appendMenu(sensMenu, checkFlag(v == trayCB.CurrentSensitivity()), uintptr(idSensBase+i), labelAt(s.Sensitivities, i, v))
	}
	appendMenu(menu, mfString|mfPopup, sensMenu, s.MenuSensitivity)

	holdMenu := createSub()
	curHold := trayCB.CurrentHold()
	for i, v := range trayCB.Holds {
		appendMenu(holdMenu, checkFlag(v == curHold), uintptr(idHoldBase+i), labelAt(s.Holds, i, ""))
	}
	appendMenu(menu, mfString|mfPopup, holdMenu, s.MenuHold)

	langMenu := createSub()
	for i, v := range trayCB.Langs {
		appendMenu(langMenu, checkFlag(v == trayCB.CurrentLang()), uintptr(idLangBase+i), labelAt(s.Langs, i, v))
	}
	appendMenu(menu, mfString|mfPopup, langMenu, s.MenuLanguage)

	appendMenu(menu, checkFlag(trayCB.AutostartOn()), idToggleAutostart, s.Autostart)
	appendMenu(menu, mfSeparator, 0, "")
	appendMenu(menu, mfString, idQuit, s.Quit)

	var pt struct{ X, Y int32 }
	procGetCursorPosT.Call(uintptr(unsafe.Pointer(&pt)))
	procSetForeground.Call(hwnd)
	cmd, _, _ := procTrackPopupMenu.Call(
		menu, tpmRightButton|tpmReturnCmd|tpmNonNotify,
		uintptr(pt.X), uintptr(pt.Y), 0, hwnd, 0,
	)
	procPostMessage.Call(hwnd, 0, 0, 0) // WM_NULL: classic popup-dismiss fix
	procDestroyMenu.Call(menu)

	dispatch(int(cmd))
}

func dispatch(id int) {
	switch {
	case id >= idStyleBase && id < idLangBase:
		i := id - idStyleBase
		if i >= 0 && i < len(trayCB.Styles) {
			trayCB.OnStyle(trayCB.Styles[i])
		}
	case id >= idLangBase && id < idLangBase+100:
		i := id - idLangBase
		if i >= 0 && i < len(trayCB.Langs) {
			trayCB.OnLang(trayCB.Langs[i])
		}
	case id >= idScaleBase && id < idSensBase:
		trayCB.OnScale(id - idScaleBase)
	case id >= idSensBase && id < idHoldBase:
		i := id - idSensBase
		if i >= 0 && i < len(trayCB.Sensitivities) {
			trayCB.OnSensitivity(trayCB.Sensitivities[i])
		}
	case id >= idHoldBase && id < idToggleAutostart:
		i := id - idHoldBase
		if i >= 0 && i < len(trayCB.Holds) {
			trayCB.OnHold(trayCB.Holds[i])
		}
	case id == idToggleAutostart:
		trayCB.OnToggleAutostart()
	case id == idQuit:
		trayCB.OnQuit()
		procDestroyWindow.Call(trayHwnd)
	}
}

func appendMenu(menu, flags, item uintptr, text string) {
	var p *uint16
	if text != "" {
		p, _ = windows.UTF16PtrFromString(text)
	}
	procAppendMenu.Call(menu, flags, item, uintptr(unsafe.Pointer(p)))
}

func copyUTF16(dst []uint16, s string) {
	src := windows.StringToUTF16(s)
	n := copy(dst, src)
	if n < len(dst) {
		dst[n-1] = 0
	}
}
