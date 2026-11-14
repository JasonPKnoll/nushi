import 'dart:io';

import 'package:nushi/app_log.dart';
import 'package:nushi/desktop.dart';
import 'package:nushi/shell_windows.dart';

/// Windows's file manager and default apps (ADR 0124): Explorer shows a
/// file selected (`/select,`) and opens a folder; a file opens as text
/// with the program Windows has for `.txt`, whatever the file is (else
/// Notepad). Each starts with none of Nushi's variables. As on the Mac and
/// Linux, a failure opens nothing and throws nothing: it's logged.
class WindowsDesktop implements Desktop {
  const new();

  @override
  Future<void> reveal(String path) =>
      _start('reveal', systemProgram('explorer.exe'), ['/select,$path']);

  @override
  Future<void> openFolder(String folder) =>
      _start('openFolder', systemProgram('explorer.exe'), [folder]);

  @override
  Future<void> openAsText(String path) => _start(
    'openAsText',
    associated('.txt', AssocWhat.executable) ?? systemProgram('notepad.exe'),
    [path],
  );

  /// Starts [program]; a failure is logged by [call] alone, never the
  /// path, which may name the user's work.
  static Future<void> _start(
    String call,
    String program,
    List<String> args,
  ) async {
    try {
      await startOutside(program, args);
    } on ProcessException catch (e) {
      appLog.warn('the desktop opened nothing', {
        'call': call,
        'code': '${e.errorCode}',
      });
    }
  }
}
