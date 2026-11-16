import 'dart:typed_data';

import 'package:nushi/clipboard_files.dart';

/// Windows's clipboard gives Ctrl+V no files or picture yet (ADR 0125;
/// `CF_HDROP` and `CF_DIB` later): only its text, which Flutter reads.
class WindowsClipboardFiles implements ClipboardFiles {
  const new();

  @override
  Future<List<String>> files() async => const [];

  @override
  Future<Uint8List?> image() async => null;
}
