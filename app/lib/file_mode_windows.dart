import 'package:nushi/file_mode.dart';

/// Windows has no mode bits: a file the app writes in its data folder
/// takes that folder's access list, which the user's profile already
/// limits to the user, SYSTEM and the administrators (ADR 0123, 0124), so
/// there's nothing to set.
class WindowsFileModes implements FileModes {
  const new();

  @override
  void setMode(String path, int mode) {}
}
