import 'dart:io';

import 'package:nushi/fonts.dart';

/// The monospaced families Windows ships or installs often, by the file
/// each comes in.
const _known = {
  'CascadiaMono.ttf': 'Cascadia Mono',
  'CascadiaCode.ttf': 'Cascadia Code',
  'consola.ttf': 'Consolas',
  'cour.ttf': 'Courier New',
  'lucon.ttf': 'Lucida Console',
};

/// Windows's monospaced fonts (ADR 0124): those of [_known] whose file is
/// in the system's or the user's fonts folder. Listing every family needs
/// DirectWrite, which comes later.
class WindowsFonts implements Fonts {
  const new();

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
}
