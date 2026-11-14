import 'package:nushi/alerts_windows.dart';
import 'package:nushi/app_install_windows.dart';
import 'package:nushi/back_swipe_windows.dart';
import 'package:nushi/clipboard_files_windows.dart';
import 'package:nushi/conventions.dart';
import 'package:nushi/desktop_windows.dart';
import 'package:nushi/file_mode_windows.dart';
import 'package:nushi/fonts_windows.dart';
import 'package:nushi/login_item_windows.dart';
import 'package:nushi/mermaid_windows.dart';
import 'package:nushi/native.dart';
import 'package:nushi/web_views_windows.dart';
import 'package:nushi/window_drag_windows.dart';

/// Windows's implementation of each native capability (ADR 0124): Explorer
/// and the shell's associations through FFI, no runner channel of its own
/// yet; notifications, the clipboard's files, Start at login, web views
/// and spell check do nothing for now.
Native windowsNative() => Native(
  alerts: const WindowsAlertCenter(),
  clipboard: const WindowsClipboardFiles(),
  conventions: windowsConventions,
  desktop: const WindowsDesktop(),
  diagrams: const WindowsDiagrams(),
  fileModes: const WindowsFileModes(),
  fonts: const WindowsFonts(),
  install: WindowsAppInstall(),
  loginItem: const WindowsLoginItem(),
  swipes: const WindowsPageSwipes(),
  web: const WindowsWebViews(),
  window: const WindowsWindow(),
);
