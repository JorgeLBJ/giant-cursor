<p align="center">
  <img src="assets/icon-preview.png" width="128" alt="Giant Cursor">
</p>

<h1 align="center">Giant Cursor</h1>

<p align="center">
  <strong>Lost your mouse pointer? Shake it and it grows.</strong>
</p>

<p align="center">
  <a href="https://github.com/JorgeLBJ/giant-cursor/releases"><img src="https://img.shields.io/github/v/release/JorgeLBJ/giant-cursor?label=download" alt="Latest release"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="MIT License"></a>
  <img src="https://img.shields.io/badge/platform-Windows%2010%2F11-0078d6" alt="Windows 10/11">
</p>

---

Giant Cursor is a tiny, fast Windows utility for people who lose track of the
mouse pointer — especially with low vision. Shake the mouse and the cursor
instantly grows so you can find it, then it shrinks back on its own. Just like
macOS, but on Windows.

It stays out of your way: **no window, negligible CPU, and it keeps working
even when dozens of apps are open.**

## Quick start

1. Download **`GiantCursorSetup.exe`** from the [Releases page](https://github.com/JorgeLBJ/giant-cursor/releases) and run it.
2. Pick your language and tick **"Start with Windows"** if you want it always on.
3. **Shake the mouse.** The cursor grows. Stop shaking and it returns to normal.

Prefer no installer? Download the standalone **`giant-cursor.exe`** and run it — zero dependencies, nothing to configure.

## Using it

| What you want | What to do |
|---|---|
| Find the cursor | Shake the mouse back and forth |
| Back to normal | Just stop shaking (it shrinks after ~1s, even while you keep moving) |
| Configure it | Right-click the tray icon (next to the clock) |
| Quit | Tray icon → **Quit** |
| Cursor stuck large | Run `giant-cursor.exe --restore`, or simply launch the app again |

### Tray menu

Everything is configurable live, no restart:

| Menu | Options |
|---|---|
| **Cursor** | `Crisp arrow` (sharp custom pointer) or `System (zoom)` (your real cursor, enlarged) |
| **Size** | 2x, 3x, **4x** (default), 5x, 6x, 8x |
| **Sensitivity** | Low, **Medium** (default), High — how hard you must shake |
| **Enlarged for** | Short (0.7s), **Normal (1s)**, Long (1.5s) |
| **Language** | English / Español (auto-detected from Windows) |
| **Start with Windows** | Toggle autostart |

## Why not PowerToys "Find My Mouse"?

PowerToys draws a full-screen spotlight (heavy) and relies on a global mouse
hook that Windows can starve when the system is busy — so it sometimes stops
responding right when you need it.

Giant Cursor takes the opposite approach:

| | PowerToys | Giant Cursor |
|---|---|---|
| Detection | Global mouse hook (can be starved) | **Polling** — can't be starved |
| Effect | Full-screen spotlight overlay | **Swaps the system cursor** — no overlay, no per-frame drawing |
| Result | Stutters under load | Reliable and light |

## Command-line options

You normally never need these — the tray covers everything.

| Flag | Default | Description |
|------|---------|-------------|
| `--scale` | `4` | Enlargement factor |
| `--sensitivity` | `medium` | `low`, `medium`, or `high` |
| `--hold-ms` | `1000` | How long it stays enlarged after the last shake |
| `--lang` | auto | `en` or `es` |
| `--install` | — | Enable autostart and save settings |
| `--uninstall` | — | Disable autostart and restore the cursor |
| `--restore` | — | Restore the cursor and exit (panic button) |

Example: `giant-cursor.exe --scale 6 --sensitivity high --install`

Settings live in `%LOCALAPPDATA%\giant-cursor\config.json`.

## How it works

- A background goroutine polls `GetCursorPos` ~120 times/second and flags a
  **shake** as several rapid direction reversals within a short window.
- On a shake it calls `SetSystemCursor` to swap the standard cursors for
  enlarged ones; after ~1s with no further shake it reloads your cursor scheme.
- The shake-detection logic is pure and fully unit-tested; Windows sits behind
  small adapters (hexagonal architecture).

## Build from source

Requires **Go 1.26+**. No CGo, no external runtime — one self-contained `.exe`.

```bash
git clone https://github.com/JorgeLBJ/giant-cursor.git
cd giant-cursor
go build -o giant-cursor.exe ./cmd/giant-cursor
go test ./...
```

Release build (no console window):

```bash
go build -ldflags "-H=windowsgui -s -w" -o giant-cursor.exe ./cmd/giant-cursor
```

Regenerate the icon from `assets/icon-source.png`, and the installer:

```bash
go run ./cmd/genicon                # -> assets/giant-cursor.ico + embedded copy
iscc installer\giant-cursor.iss     # -> installer/Output/GiantCursorSetup.exe
```

## Known limitations

- Only **standard system cursors** are enlarged. Apps with a fully custom
  cursor (some games and design tools) are not affected.
- `System (zoom)` mode looks slightly soft, because it upscales the 32px system
  cursor. Use `Crisp arrow` for a sharp pointer at any size.

## Contributing

Issues and pull requests are welcome — especially accessibility feedback from
people who actually rely on this. Open an
[issue](https://github.com/JorgeLBJ/giant-cursor/issues) to start.

## License

MIT © Jorge Luis Bustamante Jara — see [LICENSE](LICENSE).
