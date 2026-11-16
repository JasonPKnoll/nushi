# 0135. The app's capabilities on Windows

Status: accepted, 2026-11-17. Builds on [0125](0125-the-app-runs-on-windows.md) (the app on Windows), whose stand-ins it replaces one capability at a time; amends it.

## Context
ADR 0125 got the app running on Windows with most capabilities off: each a stand-in in its `<capability>_windows.dart`. What each needs from Windows is a call through `dart:ffi` (user32, shell32, COM), so the runner stays without a channel of its own where it can.

## Decision
- **The clipboard**: read in-process through user32, opened and closed in one synchronous call. Files are `CF_HDROP`'s paths (`DragQueryFileW`). A picture, only when the clipboard holds no text (as on the Mac), is the PNG an app registered (`PNG`, a browser's copied image) as it is, else the bitmap (`CF_DIB`, which a screenshot is and Windows makes of any other) drawn as PNG by the engine: 24- and 32-bit pixels are read in Dart (a 32-bit one whose alpha is 0 everywhere is opaque, as apps that draw without alpha leave it), any other kind given to the engine as a BMP file.
- **Presenting the window**: the app's window is the runner's class's that this process made (`FindWindowExW`); it's restored if minimized and `SetForegroundWindow`ed. Windows grants the foreground only to a process that may take it (the user's last input, a notification's click), so where it refuses, the taskbar's button flashes until the window is chosen (`FlashWindowEx`), as a Linux desktop marks a window as wanting attention.

## Consequences
- Ctrl+V attaches Explorer's copied files and a pasted screenshot, as ⌘V does on the Mac.
- A notification's click (once Windows has notifications) can bring the window up through `AppWindow.present`, as on Linux.

## Alternatives considered
- `CF_DIBV5` first: it carries alpha masks, but Windows synthesizes it from `CF_DIB` for most apps, and the engine's BMP reader takes either; `CF_DIB` is what every app's bitmap becomes.
- A runner channel in C++ (GDI+ to encode PNG): more native code to build and test for what FFI and the engine's encoder do.
