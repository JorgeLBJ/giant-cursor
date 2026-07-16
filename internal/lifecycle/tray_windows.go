//go:build windows

package lifecycle

import (
	"fmt"
	"unsafe"

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
	idQuit            = 900
)

// HoldOption is one entry in the hold-time submenu.
type HoldOption struct {
	Label  string
	Millis int64
}

// TrayCallbacks describe the menu contents and the actions for each item.
type TrayCallbacks struct {
	Scales        []int
	Sensitivities []string
	Holds         []HoldOption

	CurrentScale       func() int
	CurrentSensitivity func() string
	CurrentHold        func() int64
	AutostartOn        func() bool

	OnScale           func(scale int)
	OnSensitivity     func(name string)
	OnHold            func(ms int64)
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

	hicon, _, _ := procLoadIcon.Call(0, idiApplication)

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

func showMenu(hwnd uintptr) {
	menu, _, _ := procCreatePopupMenu.Call()

	scaleMenu, _, _ := procCreatePopupMenu.Call()
	curScale := trayCB.CurrentScale()
	for _, s := range trayCB.Scales {
		flags := uintptr(mfString)
		if s == curScale {
			flags |= mfChecked
		}
		appendMenu(scaleMenu, flags, uintptr(idScaleBase+s), fmt.Sprintf("%dx", s))
	}
	appendMenu(menu, mfString|mfPopup, scaleMenu, "Size")

	sensMenu, _, _ := procCreatePopupMenu.Call()
	curSens := trayCB.CurrentSensitivity()
	for i, name := range trayCB.Sensitivities {
		flags := uintptr(mfString)
		if name == curSens {
			flags |= mfChecked
		}
		appendMenu(sensMenu, flags, uintptr(idSensBase+i), title(name))
	}
	appendMenu(menu, mfString|mfPopup, sensMenu, "Sensitivity")

	holdMenu, _, _ := procCreatePopupMenu.Call()
	curHold := trayCB.CurrentHold()
	for i, h := range trayCB.Holds {
		flags := uintptr(mfString)
		if h.Millis == curHold {
			flags |= mfChecked
		}
		appendMenu(holdMenu, flags, uintptr(idHoldBase+i), h.Label)
	}
	appendMenu(menu, mfString|mfPopup, holdMenu, "Big for")

	autostartFlags := uintptr(mfString)
	if trayCB.AutostartOn() {
		autostartFlags |= mfChecked
	}
	appendMenu(menu, autostartFlags, idToggleAutostart, "Start with Windows")

	appendMenu(menu, mfSeparator, 0, "")
	appendMenu(menu, mfString, idQuit, "Quit Giant Cursor")

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
			trayCB.OnHold(trayCB.Holds[i].Millis)
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

func title(s string) string {
	if s == "" {
		return s
	}
	return string(s[0]-32) + s[1:] // ASCII sensitivity names ("low"/"medium"/"high")
}
