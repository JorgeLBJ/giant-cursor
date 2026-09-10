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

Prefer no installer? Download **`giant-cursor-portable.exe`** and run it — zero dependencies, nothing to configure.

> **"Windows protected your PC"?** That warning is expected — see
> [Why does Windows warn about this?](#why-does-windows-warn-about-this) below.

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
| **Cursor** | `Crisp arrow` — sharp high-resolution pointer and hand — or `System (zoom)` — your real cursors, enlarged |
| **Size** | 2x, 3x, **4x** (default), 5x, 6x, 8x |
| **Sensitivity** | Low, **Medium** (default), High — how hard you must shake |
| **Enlarged for** | Short (0.7s), **Normal (1s)**, Long (1.5s) |
| **Language** | English / Español (auto-detected from Windows) |
| **Start with Windows** | Toggle autostart |

### Game overlay (beta)

Games such as World of Warcraft ship their own cursor artwork, so enlarging the
Windows cursor does not reach them. The overlay draws a separate always-on-top
marker that follows the pointer instead.

Pick a mode from the tray menu, or start with `-overlay halo`:

- **Halo ring** draws a ring around the pointer. Recommended.
- **Large pointer** draws a big arrow. You will see two cursors, yours and the
  game's, because the game keeps drawing its own.

Two things to know:

- **Play in windowed or borderless mode.** Exclusive fullscreen bypasses the
  desktop compositor, so nothing drawn on top of it is visible.
- **The overlay never touches the game.** It is an ordinary window: nothing is
  injected into the game process, nothing is hooked, and no game memory is read.

## Why does Windows warn about this?

When you run the download, Windows may show **"Windows protected your PC — unknown
publisher"**. That is expected, and it does not mean anything is wrong with the file.

Giant Cursor isn't **code-signed**. A signing certificate costs hundreds of dollars a
year, and this is free software. Windows shows that warning for *any* unsigned program
downloaded from the internet — Go, Electron, C++, it makes no difference.

**To run it:** click **More info** → **Run anyway**. You only do this once per file.

**To verify the download is authentic:** GitHub shows a SHA-256 for every release asset.
Compare it with your copy:

```powershell
Get-FileHash .\GiantCursorSetup.exe -Algorithm SHA256
```

If the hashes match, your file is byte-for-byte what the [release workflow](.github/workflows/release.yml)
built from this source code.

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
- `Crisp arrow` mode replaces the pointer and the hand with **high-resolution
  artwork scaled down** to the chosen size. Downscaling stays sharp — the
  blurriness of other tools comes from upscaling the 32px system cursors.
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

Regenerate the assets and the installer:

```bash
go run ./cmd/genicon                # app icon  <- assets/icon-source.png
go run ./cmd/genarrow               # cursors   <- assets/cursor-raw/*.png
iscc installer\giant-cursor.iss     # installer -> installer/Output/GiantCursorSetup.exe
```

To restyle a cursor, replace its art in `assets/cursor-raw/` (white shape with a
black outline on a blue background — the blue is keyed out automatically) and
re-run `go run ./cmd/genarrow`. No code changes needed.

## Known limitations

- Only **standard system cursors** are enlarged. Apps with a fully custom
  cursor (some games and design tools) are not affected.
- In `Crisp arrow` mode only the **pointer and the hand** use high-resolution
  artwork — the ones you see almost all the time. The remaining cursors (text
  caret, resize handles, busy…) are upscaled and therefore softer.
- `System (zoom)` mode is soft across the board, because Windows only ships its
  cursors at 32px. It is there for when you prefer your exact system cursors.

## Contributing

Issues and pull requests are welcome — especially accessibility feedback from
people who actually rely on this. Open an
[issue](https://github.com/JorgeLBJ/giant-cursor/issues) to start.

## License

MIT © Jorge Luis Bustamante Jara — see [LICENSE](LICENSE).
