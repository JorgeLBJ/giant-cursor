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

	"giant-cursor/internal/app"
	"giant-cursor/internal/config"
	"giant-cursor/internal/cursor"
	"giant-cursor/internal/input"
	"giant-cursor/internal/lifecycle"
	"giant-cursor/internal/shake"

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

func main() {
	scale := flag.Int("scale", 4, "cursor enlargement factor")
	sens := flag.String("sensitivity", "medium", "shake sensitivity: low|medium|high")
	hold := flag.Int64("hold-ms", 1000, "milliseconds to stay enlarged after the last shake")
	install := flag.Bool("install", false, "enable autostart and save settings, then run")
	uninstall := flag.Bool("uninstall", false, "disable autostart, restore cursors, and exit")
	restore := flag.Bool("restore", false, "restore cursors and exit (panic button)")
	_ = flag.Bool("silent", false, "reserved: run without console output")
	flag.Parse()

	cur := cursor.NewWin32(*scale)

	// Panic button and uninstall must work even if another instance is running.
	if *restore {
		_ = cur.Restore()
		return
	}
	if *uninstall {
		_ = setAutostart(false)
		_ = cur.Restore()
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

	s := loadSettings(settings{Scale: *scale, Sensitivity: *sens, HoldMillis: *hold})
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
	cur = cursor.NewWin32(s.Scale)

	if *install {
		if err := saveSettings(s); err != nil {
			fmt.Fprintln(os.Stderr, "save settings failed:", err)
		}
		if err := setAutostart(true); err != nil {
			fmt.Fprintln(os.Stderr, "autostart failed:", err)
		}
	}

	// SAFETY: clean baseline first, so a previous crash that left the cursor
	// enlarged is fixed simply by launching again.
	_ = cur.Restore()

	det := shake.New(config.ShakeConfig(config.Sensitivity(s.Sensitivity), s.HoldMillis))
	application := app.New(det, cur)
	poller := input.NewWin32Poller()

	done := make(chan struct{})
	cleanup := func() { _ = cur.Restore() }
	lifecycle.OnConsoleClose(cleanup)

	fmt.Printf("Giant Cursor running (scale=%d, sensitivity=%s, hold=%dms). Shake to enlarge. Ctrl+Shift+F12 to quit.\n",
		s.Scale, s.Sensitivity, s.HoldMillis)

	go runLoop(application, poller, done)

	runtime.LockOSThread()
	lifecycle.RunHotkeyLoop(func() {
		close(done)
		cleanup()
	})
}

func runLoop(a *app.App, poller input.PositionSource, done <-chan struct{}) {
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
			_ = a.Step(shake.Sample{Pos: pos, Millis: time.Since(start).Milliseconds()})
		}
	}
}
