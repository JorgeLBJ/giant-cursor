//go:build windows

package lifecycle

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32u          = windows.NewLazySystemDLL("user32.dll")
	procRegisterHK   = user32u.NewProc("RegisterHotKey")
	procGetMessageW  = user32u.NewProc("GetMessageW")
	procTranslateMsg = user32u.NewProc("TranslateMessage")
	procDispatchMsgW = user32u.NewProc("DispatchMessageW")
)

const (
	modAlt     = 0x0001 // MOD_ALT
	modControl = 0x0002 // MOD_CONTROL
	wmHotkey   = 0x0312 // WM_HOTKEY
	vkQ        = 0x51   // 'Q'
)

type winMsg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ X, Y int32 }
}

// RunHotkeyLoop registers Ctrl+Alt+Q and blocks, pumping messages, until the
// hotkey fires (then it calls onExit and returns) or GetMessage fails. Call
// runtime.LockOSThread() on the calling goroutine first: the message queue is
// thread-specific.
func RunHotkeyLoop(onExit func()) {
	procRegisterHK.Call(0, 1, modControl|modAlt, vkQ)
	var m winMsg
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 { // 0 = WM_QUIT, -1 = error
			break
		}
		if m.message == wmHotkey {
			onExit()
			return
		}
		procTranslateMsg.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMsgW.Call(uintptr(unsafe.Pointer(&m)))
	}
}
