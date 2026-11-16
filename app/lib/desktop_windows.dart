import 'dart:ffi';
import 'dart:io';
import 'dart:isolate';
import 'dart:typed_data';

import 'package:nushi/app_log.dart';
import 'package:nushi/desktop.dart';
import 'package:nushi/shell_windows.dart';

/// Windows's file manager and default apps (ADR 0124): Explorer shows a
/// file selected (the shell's `SHOpenFolderAndSelectItems`) and opens a
/// folder; a file opens as text with the program Windows has for `.txt`,
/// whatever the file is (else Notepad). Each program starts with none of
/// Nushi's variables. As on the Mac and Linux, a failure opens nothing and
/// throws nothing: it's logged.
class WindowsDesktop implements Desktop {
  const new();

  /// Through the shell's own call, which takes the path whole:
  /// `explorer.exe /select,<path>` splits it at a comma, so
  /// `notes,v2.txt` opened the wrong place. In an isolate of its own, as
  /// Explorer may take a moment to answer.
  @override
  Future<void> reveal(String path) async {
    try {
      final hr = await Isolate.run(() => _select(path));
      if (hr != 0) _failed('reveal', hr);
    } on Object catch (e) {
      appLog.warn('the desktop opened nothing', {
        'call': 'reveal',
        'error': e.runtimeType.toString(),
      });
    }
  }

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
      _failed(call, e.errorCode);
    }
  }

  static void _failed(String call, int code) =>
      appLog.warn('the desktop opened nothing', {
        'call': call,
        'code': '$code',
      });
}

/// shell32 and ole32, loaded before the first call, so the process's
/// lookup of the calls below finds them.
final DynamicLibrary _shell32 = DynamicLibrary.open('shell32.dll');
final DynamicLibrary _ole32 = DynamicLibrary.open('ole32.dll');

@Native<Int32 Function(Pointer<Void>, Uint32)>(
  symbol: 'CoInitializeEx',
  isLeaf: true,
)
external int _coInitializeEx(Pointer<Void> reserved, int model);

@Native<Void Function()>(symbol: 'CoUninitialize', isLeaf: true)
external void _coUninitialize();

@Native<Pointer<Void> Function(Pointer<Uint16>)>(
  symbol: 'ILCreateFromPathW',
  isLeaf: true,
)
external Pointer<Void> _itemOf(Pointer<Uint16> path);

@Native<Void Function(Pointer<Void>)>(symbol: 'ILFree', isLeaf: true)
external void _freeItem(Pointer<Void> item);

@Native<Int32 Function(Pointer<Void>, Uint32, Pointer<Void>, Uint32)>(
  symbol: 'SHOpenFolderAndSelectItems',
)
external int _openFolderAndSelect(
  Pointer<Void> item,
  int count,
  Pointer<Void> items,
  int flags,
);

/// COINIT_APARTMENTTHREADED, as the shell's calls want.
const _apartment = 0x2;

/// E_FAIL, for what fails before the shell is asked.
const _failure = -0x7fffbffb;

/// Shows [path] selected in its folder's Explorer window, on this thread's
/// COM: an item list of the one file, whose parent opens. The HRESULT, 0
/// when it did.
int _select(String path) {
  if (!_shell32.providesSymbol('SHOpenFolderAndSelectItems') ||
      !_ole32.providesSymbol('CoInitializeEx')) {
    return _failure;
  }
  final com = _coInitializeEx(nullptr, _apartment);
  try {
    // Windows names take no `/`: one is a separator the shell may not read.
    final wide = Uint16List.fromList([
      ...path.replaceAll('/', r'\').codeUnits,
      0,
    ]);
    final item = _itemOf(wide.address);
    if (item == nullptr) return _failure; // no such file
    try {
      return _openFolderAndSelect(item, 0, nullptr, 0);
    } finally {
      _freeItem(item);
    }
  } finally {
    // S_OK or S_FALSE are this call's to undo; RPC_E_CHANGED_MODE (another
    // model already, negative) isn't.
    if (com >= 0) _coUninitialize();
  }
}
