// A chat's one name for a file, however a tool row, a show call or the
// Artifacts widget named it.

/// [path] (relative to [folder], or absolute) as an absolute path.
String absoluteIn(String folder, String path) =>
    path.startsWith('/') ? path : '$folder/$path';

/// [path] (relative to [folder], or absolute) as the daemon names a tool
/// call's file (chats' `rel`): relative to [folder] when inside it, else
/// absolute; lexically, links not followed. A show call's path is the
/// agent's own, often absolute, and one file must have one name wherever
/// the chat's artifacts are compared.
String relativeIn(String folder, String path) {
  if (!folder.startsWith('/')) return path;
  final file = _cleaned(absoluteIn(folder, path));
  final root = _cleaned(folder);
  final inside = root == '/' ? '/' : '$root/';
  return file.length > inside.length && file.startsWith(inside)
      ? file.substring(inside.length)
      : file;
}

/// An absolute [path] without its empty, `.` and `..` parts, as Go's
/// `filepath.Clean` makes it.
String _cleaned(String path) {
  final parts = <String>[];
  for (final p in path.split('/')) {
    if (p == '..') {
      if (parts.isNotEmpty) parts.removeLast();
    } else if (p.isNotEmpty && p != '.') {
      parts.add(p);
    }
  }
  return '/${parts.join('/')}';
}
