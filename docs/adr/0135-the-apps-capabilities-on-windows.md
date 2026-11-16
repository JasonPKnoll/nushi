# 0135. The app's capabilities on Windows

Status: accepted, 2026-11-17. Builds on [0125](0125-the-app-runs-on-windows.md) (the app on Windows), whose stand-ins it replaces one capability at a time; amends it.

## Context
ADR 0125 got the app running on Windows with most capabilities off: each a stand-in in its `<capability>_windows.dart`. What each needs from Windows is a call through `dart:ffi` (user32, shell32, COM), so the runner stays without a channel of its own where it can.

## Decision
- **The clipboard**: read in-process through user32, opened and closed in one synchronous call. Files are `CF_HDROP`'s paths (`DragQueryFileW`). A picture, only when the clipboard holds no text (as on the Mac), is the PNG an app registered (`PNG`, a browser's copied image) as it is, else the bitmap (`CF_DIB`, which a screenshot is and Windows makes of any other) drawn as PNG by the engine: 24- and 32-bit pixels are read in Dart (a 32-bit one whose alpha is 0 everywhere is opaque, as apps that draw without alpha leave it), any other kind given to the engine as a BMP file.
- **Presenting the window**: the app's window is the runner's class's that this process made (`FindWindowExW`); it's restored if minimized and `SetForegroundWindow`ed. Windows grants the foreground only to a process that may take it (the user's last input, a notification's click), so where it refuses, the taskbar's button flashes until the window is chosen (`FlashWindowEx`), as a Linux desktop marks a window as wanting attention.
- **Spell check**: Windows's Spell Checking API (`ISpellChecker`, `spellcheck.h`), the checker its own text boxes use, through COM by FFI (each method by its vtable slot; no COM package), on the app's thread, whose apartment the runner set up. Its language is the user's locale's (`GetUserDefaultLocaleName`) among those it supports, else the first of that language; none, and there's no spell check. Ignore is its session's `Ignore`, Learn its `Add` (the user's dictionary for that language, shared by every app that checks with it).
- **Stopping the daemon**: a Windows Unix socket doesn't name the process at its other end, and a process with no console window has no SIGTERM, so the daemon says its pid in `Ping` (on the socket only, API 122) and makes an event, `Local\nushid-<pid>-stop`; set, it stops as on SIGTERM (agents, terminals and dev servers closed first). The app sets the event named by the pid its last ping gave (`AppInstall.stop`'s `pid`), with only `EVENT_MODIFY_STATE`; a daemon too old to say its pid isn't stopped, and the restart says to end it in Task Manager. An event of that name that exists before the daemon makes it is another process's, and the daemon doesn't watch it.

## Consequences
- Ctrl+V attaches Explorer's copied files and a pasted screenshot, as ⌘V does on the Mac.
- The composer checks spelling on Windows as on the Mac and Linux.
- Restart daemon is offered on Windows (`AppInstall.canStop`), as on the Mac and Linux, so a newer bundled daemon replaces the running one.
- A notification's click (once Windows has notifications) can bring the window up through `AppWindow.present`, as on Linux.

## Alternatives considered
- `CF_DIBV5` first: it carries alpha masks, but Windows synthesizes it from `CF_DIB` for most apps, and the engine's BMP reader takes either; `CF_DIB` is what every app's bitmap becomes.
- A runner channel in C++ (GDI+ to encode PNG; the spell checker's COM): more native code to build and test for what FFI and the engine's encoder do.
- `TerminateProcess` by the pid: simpler, but the daemon would end without closing its agents' sessions and terminals; a console control event needs the app to attach to the daemon's hidden console, which its agents share too.
- `package:win32`'s COM bindings: a large dependency for the five methods the checker needs.
