import 'package:flutter/material.dart';

import 'package:nushi/app_actions.dart';
import 'package:nushi/chat.dart';
import 'package:nushi/controls.dart';
import 'package:nushi/daemon.dart';
import 'package:nushi/edit_bars.dart' show FileTabsFocus;
import 'package:nushi/file_view.dart';
import 'package:nushi/files_bar.dart';
import 'package:nushi/files_open.dart';
import 'package:nushi/files_split.dart';
import 'package:nushi/files_top_row.dart';
import 'package:nushi/files_tree.dart';
import 'package:nushi/folder_tree.dart';
import 'package:nushi/keys.dart';
import 'package:nushi/open_file.dart' show openFileKey;
import 'package:nushi/panes.dart';
import 'package:nushi/preview_entry.dart';
import 'package:nushi/settings.dart';
import 'package:nushi/theme/theme.dart';

export 'package:nushi/files_open.dart';

/// The Files pane (ADR 0123), as Claude desktop's: its top row
/// ([FilesTopRow]: Files, the open files' name or tabs, ×), its bar
/// ([FilesBar]: the shown file's path and buttons), then the viewer of the
/// open files ([FileView]), blank with none ([FilesBlank]), and the tree of
/// the chat's folder at the pane's right ([FilesTree]), its edge dragged.
/// Opening a file never moves the tree. The viewer gives its width first
/// as the pane narrows, then the tree ([treeWidth]). With a daemon older
/// than the tree (`filesApi`) the viewer is alone.
class FilesPane extends StatefulWidget {
  const new({
    required this.chat,
    required this.daemon,
    required this.onClose,
    super.key,
    this.tabsKey,
    this.expanded = false,
    this.onExpand,
  });

  final ChatState chat;
  final Daemon daemon;

  /// Hides the pane; its tabs stay.
  final VoidCallback onClose;

  /// The key of its tabs' row ([FilesTopRow.tabsKey]).
  final Key? tabsKey;

  /// Whether it covers the chat's whole pane (ADR 0094's Expand).
  final bool expanded;
  final VoidCallback? onExpand;

  @override
  State<FilesPane> createState() => _FilesPaneState();
}

class _FilesPaneState extends State<FilesPane> with FileTabsFocus {
  ShownFile? _shown;

  ChatState get _chat => widget.chat;

  @override
  void initState() {
    super.initState();
    _shown = _make(_chat.files.shown.value);
    _chat.files.shown.addListener(_showFile);
  }

  @override
  void dispose() {
    _chat.files.shown.removeListener(_showFile);
    // The viewer, under this, has gone already.
    _shown?.dispose();
    super.dispose();
  }

  ShownFile? _make(PreviewFile? file) =>
      file == null ? null : ShownFile(_chat, widget.daemon, file.path);

  /// Another tab shows: its file's parts are made, and the last one's let
  /// go once its view has gone.
  void _showFile() {
    final file = _chat.files.shown.value;
    if (file?.path == _shown?.path) return;
    final old = _shown;
    setState(() => _shown = _make(file));
    if (old != null) {
      WidgetsBinding.instance.addPostFrameCallback((_) => old.dispose());
    }
  }

  @override
  Widget build(BuildContext context) {
    final t = NushiTheme.of(context).t;
    final tree = widget.daemon.daemonApi >= filesApi;
    final shown = _shown;
    return LayoutBuilder(
      builder: (context, box) {
        final w = box.maxWidth;
        // Under the least of both, the two narrow alike (treeWidth).
        final squeezed = w < filesTreeLeast + filesViewerLeast;
        final most = squeezed ? treeWidth(w, filesTreeInitial) : treeMost(w);
        return Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            FilesTopRow(
              chat: _chat,
              daemon: widget.daemon,
              shown: shown,
              tabsKey: widget.tabsKey,
              expanded: widget.expanded,
              onExpand: widget.onExpand,
              onClose: widget.onClose,
            ),
            FilesBar(shown: shown),
            Expanded(
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Expanded(
                    child: shown == null
                        ? FilesBlank(tree: tree)
                        : FileView(key: ObjectKey(shown), shown: shown),
                  ),
                  if (tree)
                    Resizable(
                      name: 'filesTree',
                      initial: filesTreeInitial,
                      min: squeezed ? most : filesTreeLeast,
                      max: most,
                      left: true,
                      locked: squeezed,
                      // Its own hairline, which a locked edge keeps.
                      line: false,
                      child: DecoratedBox(
                        decoration: BoxDecoration(
                          border: Border(left: BorderSide(color: t.line)),
                        ),
                        child: FilesTree(chat: _chat, daemon: widget.daemon),
                      ),
                    ),
                ],
              ),
            ),
          ],
        );
      },
    );
  }
}

/// The viewer with no file open, as Claude desktop's: a folder, "Open
/// files appear here" and how to open one; without the [tree] (a daemon
/// older than it), the way that's there, Go to file's key.
class FilesBlank extends StatelessWidget {
  const new({required this.tree, super.key});

  final bool tree;

  @override
  Widget build(BuildContext context) {
    final theme = NushiTheme.of(context);
    final t = theme.t;
    return ColoredBox(
      color: t.base,
      child: Center(
        child: Padding(
          padding: const EdgeInsets.all(NushiTheme.gutter),
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 280),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                LineIcon.folder.view(size: 20, color: t.textMuted),
                const SizedBox(height: 12),
                Text(
                  'Open files appear here',
                  textAlign: TextAlign.center,
                  style: theme.ui(t.text),
                ),
                const SizedBox(height: 4),
                Text(
                  tree
                      ? 'Pick a file in the tree, or click a file path in '
                            'the conversation'
                      : 'Go to a file with ${keyLabel(openFileKey)}, or '
                            'click a file path in the conversation',
                  textAlign: TextAlign.center,
                  style: theme.ui(t.textMuted, size: 12),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

/// [path] (as the preview names it) as [chat]'s Files pane does, relative
/// to the chat's folder; null for a file outside it, or a daemon without
/// the pane.
String? inFiles(Daemon daemon, ChatState chat, String path) {
  if (daemon.daemonApi < filesApi) return null;
  final folder = '${chat.folder}/';
  final inside = path.startsWith('/')
      ? (path.startsWith(folder) ? path.substring(folder.length) : null)
      : path.replaceFirst(RegExp(r'^(\./)+'), '');
  return inside == null || inside.isEmpty ? null : inside;
}

/// Shows [inFiles]' file in [chat]'s Files pane: the pane opens on the Chat
/// tab, the folders above it open, its row selected and scrolled to.
void showInFiles(ChatState chat, String inside) {
  folderTreeOf(chat).reveal(inside);
  chat.tab.value = ChatTab.chat;
  if (!filesOpen(chat)) toggleFiles(chat);
}

/// The chat header's Files button, lit while the pane shows, as the
/// terminal's and the details' are.
class FilesButton extends StatelessWidget {
  const new({required this.chat, super.key});

  final ChatState chat;

  /// Its tooltip: Show files or Hide files, and the key.
  static String tooltip({required bool shown}) =>
      withKey('${shown ? 'Hide' : 'Show'} files', keyLabel(filesKey));

  @override
  Widget build(BuildContext context) => ListenableBuilder(
    listenable: Listenable.merge([filesChats, chat.tab]),
    builder: (context, _) {
      final shown = filesShown(chat);
      return IconTap.line(
        LineIcon.folder,
        tooltip: tooltip(shown: shown),
        padding: IconTap.header,
        lit: shown,
        onTap: () => showOrHideFiles(chat),
      );
    },
  );
}
