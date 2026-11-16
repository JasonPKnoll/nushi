import 'dart:ffi';
import 'dart:io';
import 'dart:typed_data';

import 'package:nushi/app_install.dart';

/// What Windows's shell associates with a scheme or an extension
/// (shlwapi's `AssocQueryStringW`), and starting a program the way the
/// Linux runner does (ADR 0124): with the app's environment less Nushi's
/// own variables, so a browser, editor or file manager started from a
/// development run can't hand its data folder to a terminal it opens.

/// AssocQueryStringW's [AssocWhat] for a program's path and its name.
enum AssocWhat {
  executable(2),
  name(4);

  new(this.code);

  /// ASSOCSTR's value.
  final int code;
}

/// ASSOCF_IS_PROTOCOL: the association is a scheme's, not an extension's.
const _isProtocol = 0x1000;

/// shlwapi, loaded before the first call, so the process's lookup of
/// [_assocQueryString] finds it.
final DynamicLibrary _shlwapi = DynamicLibrary.open('shlwapi.dll');

@Native<
  Int32 Function(
    Uint32,
    Uint32,
    Pointer<Uint16>,
    Pointer<Uint16>,
    Pointer<Uint16>,
    Pointer<Uint32>,
  )
>(symbol: 'AssocQueryStringW', isLeaf: true)
external int _assocQueryString(
  int flags,
  int what,
  Pointer<Uint16> assoc,
  Pointer<Uint16> extra,
  Pointer<Uint16> out,
  Pointer<Uint32> outLength,
);

/// What Windows associates with [assoc] (a scheme such as `https` when
/// [scheme], else an extension such as `.txt`), as [what]: null when
/// nothing is.
String? associated(String assoc, AssocWhat what, {bool scheme = false}) {
  if (!_shlwapi.providesSymbol('AssocQueryStringW')) return null;
  final key = Uint16List.fromList([...assoc.codeUnits, 0]);
  final out = Uint16List(1024);
  final length = Uint32List(1)..[0] = out.length;
  final hr = _assocQueryString(
    scheme ? _isProtocol : 0,
    what.code,
    key.address,
    nullptr,
    out.address,
    length.address,
  );
  if (hr != 0 || length[0] < 2) return null;
  return String.fromCharCodes(out, 0, length[0] - 1); // without its NUL
}

/// The system's own program [name] (`explorer.exe`), by its full path, so
/// `PATH` can't name another.
String systemProgram(String name) =>
    '${Platform.environment['SystemRoot'] ?? r'C:\Windows'}\\$name';

/// Starts [program] with [args] on its own, the app not its parent, with
/// the app's environment less every `NUSHI_*` and `NUSHID_*` variable.
Future<void> startOutside(String program, List<String> args) => Process.start(
  program,
  args,
  environment: withoutNushi(Platform.environment),
  includeParentEnvironment: false,
  mode: ProcessStartMode.detached,
);
