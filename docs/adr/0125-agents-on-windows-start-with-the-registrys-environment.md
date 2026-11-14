# 0125. Agents on Windows start with the registry's environment

Status: accepted, 2026-11-14. Amends [0048](0048-agents-start-with-the-login-shells-environment.md) (the login shell's environment) and [0123](0123-the-daemon-builds-on-windows.md) (the daemon on Windows).

## Context
Agents start with the environment the user's login shell gives a terminal, which the daemon reads by running that shell (ADR 0048), whatever started the daemon. Windows has no login shell: a terminal there, PowerShell or cmd, starts with the environment Windows builds for every new process from the registry, the system's variables and the user's (System Properties' Environment Variables, and what an installer adds to Path). The daemon ran `/bin/sh`, which isn't there, so agents got the daemon's own environment: what the app had when it started the daemon, missing any Path an installer changed since. And `LookPath` couldn't find a program there at all: it read `PATH` by its Unix name (Windows's is `Path`, the same variable whatever its case), wanted a mode bit Windows doesn't have, and didn't try `PATHEXT`'s extensions (`claude` is `claude.exe`, npm's `claude.cmd`).

## Decision
- **The environment** on Windows is the registry's as Windows builds it: `HKLM\…\Session Manager\Environment`, then `HKCU\Environment` over it, except Path, which is the system's then the user's; a `REG_EXPAND_SZ` value's `%NAME%`s are expanded against what's built so far, then the daemon's (the per-session ones, such as `USERPROFILE`, are only there), and one named nowhere stays as written. It goes over the daemon's own, without Nushi's variables, as the shell's does. Nothing runs, so it takes milliseconds and can't hang; Reload in Settings › Agents reads it again, and Settings names it Windows ("Agents start with Windows's environment").
- **Names** compare whatever their case on Windows (`shellenv`'s `Get` and `Merge`), as Windows and `os/exec` compare them.
- **`LookPath`** on Windows tries each folder of Path in order, a name with each of `PATHEXT`'s extensions (`.COM;.EXE;.BAT;.CMD` when unset), or the name itself when it has one; a regular file is a program. A folder that isn't absolute is passed over, as `exec.LookPath` refuses the current folder.
- Combining the registry's values (`registry.go`) is apart from reading them (`read_windows.go`), so CI's Mac and Linux jobs test it.

## Consequences
- An agent installed after the daemon started, its folder added to Path by the installer, is found after a Reload, without restarting anything.
- What a PowerShell profile sets isn't in it: a profile runs in that shell only, and the registry is what every program the user starts gets.
- The values are never logged or kept anywhere but in memory, as ADR 0048's.

## Alternatives considered
- PowerShell with its profile printing its environment, as a login shell does on Unix: closest to "what a terminal has" for someone whose profile sets things, but a second or more to start, at the mercy of the execution policy and whatever the profile prints or asks, and still not what cmd, Explorer or an IDE give.
- The daemon's own environment: no reading at all, but it's the app's at the daemon's start, so a Path changed since never reaches an agent until both restart.
