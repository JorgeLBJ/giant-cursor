// Command giant-cursor enlarges the Windows cursor when the mouse is shaken.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"giant-cursor/internal/control"
	"giant-cursor/internal/cursor"
	"giant-cursor/internal/i18n"
	"giant-cursor/internal/input"
	"giant-cursor/internal/lifecycle"
	"giant-cursor/internal/shake"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	appName      = "GiantCursor"
	mutexName    = "Global\\GiantCursorSingleInstance"
	runKeyPath   = `Software\Microsoft\Windows\CurrentVersion\Run`
	pollInterval = 8 * time.Millisecond
)

type settings struct {
	Scale       int    `json:"scale"`
	Sensitivity string `json:"sensitivity"`
	HoldMillis  int64  `json:"hold_ms"`
	Style       string `json:"style"`
	Lang        string `json:"lang"`
	BaseSize    int    `json:"base_cursor_size"` // user's normal cursor size (px)
}

// defaultLang returns "es" when the Windows UI language is Spanish, else "en".
func defaultLang() string {
	proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetUserDefaultUILanguage")
	r, _, _ := proc.Call()
	if r&0x3FF == 0x0A { // LANG_SPANISH
		return "es"
	}
	return "en"
}

func configPath() string {
	dir := filepath.Join(os.Getenv("LOCALAPPDATA"), "giant-cursor")
	return filepath.Join(dir, "config.json")
}

func loadSettings(def settings) settings {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return def
	}
	var s settings
	if json.Unmarshal(data, &s) != nil {
		return def
	}
	if s.Scale < 1 {
		s.Scale = def.Scale
	}
	if s.Sensitivity == "" {
		s.Sensitivity = def.Sensitivity
	}
	if s.HoldMillis <= 0 {
		s.HoldMillis = def.HoldMillis
	}
	if s.Style == "" {
		s.Style = def.Style
	}
	if s.Lang == "" {
		s.Lang = def.Lang
	}
	return s
}

func saveSettings(s settings) error {
	if err := os.MkdirAll(filepath.Dir(configPath()), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), data, 0o644)
}

func setAutostart(enabled bool) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if !enabled {
		err := k.DeleteValue(appName)
		if err == registry.ErrNotExist {
			return nil
		}
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return k.SetStringValue(appName, fmt.Sprintf("%q --silent", exe))
}

func autostartEnabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	_, _, err = k.GetStringValue(appName)
	return err == nil
}

func main() {
	scale := flag.Int("scale", 4, "cursor enlargement factor")
	sens := flag.String("sensitivity", "medium", "shake sensitivity: low|medium|high")
	hold := flag.Int64("hold-ms", 1000, "milliseconds to stay enlarged after the last shake")
	install := flag.Bool("install", false, "enable autostart and save settings, then run")
	uninstall := flag.Bool("uninstall", false, "disable autostart, restore cursors, and exit")
	restore := flag.Bool("restore", false, "restore cursors and exit (panic button)")
	_ = flag.Bool("silent", false, "reserved: run without console output")
	flag.Parse()

	// Panic button and uninstall must work even if another instance is running.
	// Restore to the saved normal size (fall back to the OS default).
	if *restore || *uninstall {
		if *uninstall {
			_ = setAutostart(false)
		}
		normal := loadSettings(settings{}).BaseSize
		if normal <= 0 {
			normal = cursor.DefaultBaseSize
		}
		// Native restore resets the base size and reloads the scheme, undoing
		// whichever style was active.
		_ = cursor.NewWin32(1, normal, cursor.StyleNative).Restore()
		return
	}

	release, already, err := lifecycle.AcquireSingleInstance(mutexName)
	if err != nil {
		fmt.Fprintln(os.Stderr, "single-instance check failed:", err)
		return
	}
	if already {
		fmt.Println("Giant Cursor is already running.")
		return
	}
	defer release()

	s := loadSettings(settings{Scale: *scale, Sensitivity: *sens, HoldMillis: *hold, Style: string(cursor.StyleCrisp), Lang: defaultLang()})
	// Explicit flags override the config file.
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "scale":
			s.Scale = *scale
		case "sensitivity":
			s.Sensitivity = *sens
		case "hold-ms":
			s.HoldMillis = *hold
		}
	})

	// Capture the user's normal cursor size once, then persist it so a crash
	// that left the cursor enlarged can always be undone on the next launch.
	if s.BaseSize <= 0 {
		s.BaseSize = cursor.CurrentBaseSize()
	}
	if err := saveSettings(s); err != nil {
		fmt.Fprintln(os.Stderr, "save settings failed:", err)
	}
	if *install {
		if err := setAutostart(true); err != nil {
			fmt.Fprintln(os.Stderr, "autostart failed:", err)
		}
	}

	normal := s.BaseSize
	newEnlarger := func(scale int, style string) cursor.Enlarger {
		return cursor.NewWin32(scale, normal, cursor.Style(style))
	}

	ctrl := control.New(
		control.Settings{Scale: s.Scale, Sensitivity: s.Sensitivity, HoldMillis: s.HoldMillis, Style: s.Style},
		newEnlarger,
		func(ns control.Settings) {
			s.Scale, s.Sensitivity, s.HoldMillis, s.Style = ns.Scale, ns.Sensitivity, ns.HoldMillis, ns.Style
			_ = saveSettings(s)
		},
	)

	// SAFETY: clean baseline first, so a previous crash that left the cursor
	// enlarged is fixed simply by launching again.
	_ = ctrl.Restore()

	poller := input.NewWin32Poller()
	done := make(chan struct{})
	go runLoop(ctrl, poller, done)

	lifecycle.OnConsoleClose(func() { _ = ctrl.Restore() })

	fmt.Printf("Giant Cursor running (scale=%d, sensitivity=%s, hold=%dms). "+
		"Shake to enlarge. Right-click the tray icon to configure or quit.\n",
		s.Scale, s.Sensitivity, s.HoldMillis)

	cb := lifecycle.TrayCallbacks{
		Strings:       func() i18n.Strings { return i18n.For(s.Lang) },
		Styles:        []string{string(cursor.StyleCrisp), string(cursor.StyleSystem)},
		Scales:        []int{2, 3, 4, 5, 6, 8},
		Sensitivities: []string{"low", "medium", "high"},
		Holds:         []int64{700, 1000, 1500},
		Langs:         []string{"en", "es"},
		CurrentStyle:       func() string { return ctrl.Get().Style },
		CurrentScale:       func() int { return ctrl.Get().Scale },
		CurrentSensitivity: func() string { return ctrl.Get().Sensitivity },
		CurrentHold:        func() int64 { return ctrl.Get().HoldMillis },
		CurrentLang:        func() string { return s.Lang },
		AutostartOn:        autostartEnabled,
		OnStyle:            ctrl.SetStyle,
		OnScale:            ctrl.SetScale,
		OnSensitivity:      ctrl.SetSensitivity,
		OnHold:             ctrl.SetHold,
		OnLang: func(code string) {
			s.Lang = code
			_ = saveSettings(s)
		},
		OnToggleAutostart: func() {
			_ = setAutostart(!autostartEnabled())
		},
		OnQuit: func() {
			close(done)
			_ = ctrl.Restore()
		},
	}

	runtime.LockOSThread()
	if err := lifecycle.RunTray(cb); err != nil {
		fmt.Fprintln(os.Stderr, "tray failed:", err)
		_ = ctrl.Restore()
	}
}

func runLoop(ctrl *control.Controller, poller input.PositionSource, done <-chan struct{}) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	start := time.Now()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			pos, err := poller.Poll()
			if err != nil {
				continue
			}
			_ = ctrl.Step(shake.Sample{Pos: pos, Millis: time.Since(start).Milliseconds()})
		}
	}
}
