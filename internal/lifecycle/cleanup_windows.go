//go:build windows

package lifecycle

import "golang.org/x/sys/windows"

var (
	kernel32             = windows.NewLazySystemDLL("kernel32.dll")
	procSetConsoleCtrlHK = kernel32.NewProc("SetConsoleCtrlHandler")
)

// OnConsoleClose runs handler when the console receives Ctrl+C / Ctrl+Break /
// close / logoff / shutdown. No-op effect if the process has no console (e.g.
// a -H=windowsgui release build); the hotkey and startup-reset paths remain.
func OnConsoleClose(handler func()) {
	cb := windows.NewCallback(func(ctrlType uint32) uintptr {
		handler()
		return 1
	})
	procSetConsoleCtrlHK.Call(cb, 1)
}
