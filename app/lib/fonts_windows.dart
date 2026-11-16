import 'dart:io';

import 'package:nushi/fonts.dart';

/// Runs a program with arguments, as `Process.run` does.
typedef RunProcess = Future<ProcessResult> Function(
  String program,
  List<String> arguments,
);

/// The monospaced families Windows ships or installs often, by the file
/// each comes in.
const _known = {
  'CascadiaMono.ttf': 'Cascadia Mono',
  'CascadiaCode.ttf': 'Cascadia Code',
  'consola.ttf': 'Consolas',
  'cour.ttf': 'Courier New',
  'lucon.ttf': 'Lucida Console',
};

/// Where Windows lists its installed fonts: the machine's, then the
/// user's own.
const _fontKeys = [
  r'HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion\Fonts',
  r'HKCU\SOFTWARE\Microsoft\Windows NT\CurrentVersion\Fonts',
];

/// Windows's installed fonts (ADR 0125): every family as the registry's
/// font list names them (`reg query`), and the monospaced ones among
/// [_known] whose file is in the system's or the user's fonts folder.
/// Telling every monospaced family needs DirectWrite, which comes later.
class WindowsFonts implements Fonts {
  /// Running `reg` with [run].
  const new({this.run = Process.run});

  final RunProcess run;

  @override
  Future<List<String>?> monospaced() async {
    final env = Platform.environment;
    final folders = [
      '${env['SystemRoot'] ?? r'C:\Windows'}\\Fonts',
      if (env['LOCALAPPDATA'] case final local?)
        '$local\\Microsoft\\Windows\\Fonts',
    ];
    return [
      for (final MapEntry(key: file, value: family) in _known.entries)
        if (folders.any((f) => File('$f\\$file').existsSync())) family,
    ];
  }

  /// Both keys' families, each once; no answer when neither could be
  /// read (the user's key is missing until they install a font).
  @override
  Future<List<String>?> families() async {
    final families = <String>{};
    var read = false;
    for (final key in _fontKeys) {
      final ProcessResult r;
      try {
        r = await run(_reg, ['query', key]);
      } on ProcessException {
        continue;
      }
      if (r.exitCode != 0) continue;
      read = true;
      families.addAll(registryFamilies('${r.stdout}'));
    }
    return read ? (families.toList()..sort()) : null;
  }

  /// No one name a family list holds: the interface font is Segoe UI's
  /// variable face, which Flutter's engine picks itself.
  @override
  String? get system => null;
}

/// The system's own `reg.exe`, by its full path, so `PATH` can't name
/// another.
String get _reg =>
    '${Platform.environment['SystemRoot'] ?? r'C:\Windows'}\\System32\\reg.exe';

/// What a value's name says after the family: the font's format.
final _format = RegExp(r'\s*\((TrueType|OpenType|All res)\)$');

/// The style words Windows names a family's bold and italic faces by,
/// which are the family's own (a weight such as Light or Black is often a
/// family of its own, so it stays).
final _style = RegExp(r'(\s+(Bold|Italic|Oblique|Regular))+$');

/// The families in `reg query`'s [output] of a font key, sorted, each
/// once: a value's name is a font's (`Arial Bold (TrueType)`), or a
/// collection's, its fonts joined by ` & `.
List<String> registryFamilies(String output) {
  final families = <String>{};
  for (final line in output.split('\n')) {
    final at = line.indexOf('    REG_');
    if (!line.startsWith('    ') || at < 0) continue;
    final name = line.substring(4, at).replaceFirst(_format, '');
    for (final font in name.split(' & ')) {
      final family = font.trim().replaceFirst(_style, '');
      if (family.isNotEmpty) families.add(family);
    }
  }
  return families.toList()..sort();
}
