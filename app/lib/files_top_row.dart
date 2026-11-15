import 'dart:async';

import 'package:flutter/material.dart';

import 'package:nushi/chat.dart';
import 'package:nushi/controls.dart';
import 'package:nushi/daemon.dart';
import 'package:nushi/edit_bars.dart';
import 'package:nushi/file_view.dart';
import 'package:nushi/files_pane.dart';
import 'package:nushi/pane_tabs.dart';
import 'package:nushi/panes.dart';
import 'package:nushi/preview.dart';
import 'package:nushi/preview_entry.dart';
import 'package:nushi/preview_marks.dart';
import 'package:nushi/theme/theme.dart';
import 'package:nushi/tool_parts.dart' show plainPath;
import 'package:nushi/widgets.dart' show Hover;

/// The Files pane's top row (ADR 0123), level with the chat's header: Files
/// at its left, as Claude desktop's; after it the open file's name and its
/// ✕, or with two or more open their tabs ([PaneTabs], ADR 0070); Live
/// while the daemon keeps the file current; at its right end Expand (while
/// [onExpand] is given) and ×, which hides the pane and keeps its tabs
/// ([PaneEndButtons]). With no file open, Files alone.
class FilesTopRow extends StatelessWidget {
  const new({
    required this.chat,
    required this.daemon,
    required this.shown,
    required this.onClose,
    super.key,
    this.tabsKey,
    this.expanded = false,
    this.onExpand,
  });

  final ChatState chat;
  final Daemon daemon;

  /// The file the viewer shows, if any.
  final ShownFile? shown;
  final VoidCallback onClose;

  /// The key of its [PaneTabs], from outside the pane, so the row keeps
  /// its scroll as the tabs change.
  final Key? tabsKey;
  final bool expanded;
  final VoidCallback? onExpand;

  /// Under this the row has no room for its name beside the file's: the
  /// file's name, or its tabs, alone.
  static const _wordsFrom = 120.0;

  /// The room between Files and the file's name or tabs.
  static const _titleGap = 16.0;

  @override
  Widget build(BuildContext context) {
    final theme = NushiTheme.of(context);
    final t = theme.t;
    final shown = this.shown;
    return PaneHeader(
      // Expanded, it's at the window's top left: with the sidebar hidden
      // it starts past the traffic lights, as a header's does.
      plain: expanded,
      end: PaneEndButtons.inset,
      child: LayoutBuilder(
        builder: (context, box) => ListenableBuilder(
          listenable: Listenable.merge([chat.files.tabs, ?shown?.watch]),
          builder: (context, _) {
            final open = chat.files.tabs.value;
            final watch = shown?.watch;
            // A tab's file on its way keeps Live's room: a pick would
            // otherwise widen the tabs' row for a moment, and move it.
            final coming =
                open.length > 1 &&
                watch != null &&
                watch.file == null &&
                watch.error.isEmpty;
            final live = watch?.live ?? false;
            return Row(
              children: [
                if (box.maxWidth >= _wordsFrom) ...[
                  tooltip(
                    context,
                    plainPath(chat.folder),
                    Text(
                      'Files',
                      maxLines: 1,
                      style: theme.ui(t.text, weight: FontWeight.w500),
                    ),
                  ),
                  const SizedBox(width: _titleGap),
                ],
                if (shown != null) ...[
                  // The shown file's kind, before its name or the tabs, so
                  // the first name stays put as a second file opens.
                  Icon(shown.kind.icon, size: 14, color: t.textSecondary),
                  const SizedBox(width: 6),
                ],
                Expanded(
                  child: switch (shown) {
                    null => const SizedBox.shrink(),
                    final shown when open.length > 1 => PaneTabs(
                      key: tabsKey,
                      chat: chat,
                      tabs: open,
                      current: PreviewFile(shown.path),
                      chips: false,
                    ),
                    final shown => Align(
                      alignment: Alignment.centerLeft,
                      child: _Name(
                        daemon: daemon,
                        chat: chat,
                        path: shown.path,
                        tooltip: watch?.file?.path ?? shown.path,
                      ),
                    ),
                  },
                ),
                if (live || coming)
                  Padding(
                    padding: const EdgeInsets.only(left: 8),
                    child: Visibility.maintain(
                      visible: live,
                      child: LiveMark(version: watch?.file),
                    ),
                  ),
                const SizedBox(width: 8),
                PaneEndButtons(
                  expanded: expanded,
                  onExpand: onExpand,
                  onClose: onClose,
                ),
              ],
            );
          },
        ),
      ),
    );
  }
}

/// The one open file's name, plain, its absolute path ([tooltip]) in a
/// tooltip, and its ✕ (its unsaved edits' dot in its place, ADR 0096): a
/// click on the name shows the file in the tree ([showInFiles]) when it's
/// in the chat's folder, and does nothing otherwise. Its text sits where a
/// tab's would ([PaneTabs]), so a second file opening moves nothing.
class _Name extends StatelessWidget {
  const new({
    required this.daemon,
    required this.chat,
    required this.path,
    required this.tooltip,
  });

  final Daemon daemon;
  final ChatState chat;
  final String path;
  final String tooltip;

  @override
  Widget build(BuildContext context) {
    final theme = NushiTheme.of(context);
    final t = theme.t;
    final inside = inFiles(daemon, chat, path);
    return Hover(
      builder: (context, {required hover}) => Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Flexible(
            child: Pressable(
              onTap: inside == null ? null : () => showInFiles(chat, inside),
              tooltip: plainPath(tooltip),
              // Tinted under the pointer and ringed with the keys on it, as
              // a control is; the text stays where a tab's is.
              builder: (context, look) => Container(
                padding: const EdgeInsets.fromLTRB(previewNameInset, 3, 3, 3),
                decoration: look.hover
                    ? BoxDecoration(
                        color: look.pressed ? t.pressed : t.hover,
                        borderRadius: BorderRadius.circular(NushiTheme.control),
                      )
                    : null,
                foregroundDecoration: focusRing(t, look),
                child: Text(
                  plainPath(baseName(path)),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: theme.code(t.text, size: 12),
                ),
              ),
            ),
          ),
          SizedBox(
            width: 18,
            child: EditedMark(
              chat: chat,
              path: path,
              shown: true,
              hover: hover,
              close: IconTap.line(
                LineIcon.close,
                size: 12,
                padding: 3,
                tooltip: 'Close tab',
                onTap: () =>
                    unawaited(closeTabSaving(context, chat, PreviewFile(path))),
              ),
            ),
          ),
        ],
      ),
    );
  }
}
