import 'package:flutter/material.dart';

import 'package:nushi/app_actions.dart' show fileTreeKey;
import 'package:nushi/chat.dart';
import 'package:nushi/controls.dart';
import 'package:nushi/conventions.dart';
import 'package:nushi/daemon.dart';
import 'package:nushi/file_view.dart';
import 'package:nushi/files_menu.dart';
import 'package:nushi/folder_tree.dart';
import 'package:nushi/keys.dart';
import 'package:nushi/menu.dart';
import 'package:nushi/panes.dart';
import 'package:nushi/preview_path.dart';
import 'package:nushi/settings.dart';
import 'package:nushi/theme/theme.dart';
import 'package:nushi/view_options.dart';

/// The Files pane's second row (ADR 0123), across the whole pane with a
/// hairline under it, as Claude desktop's. At its right end, always there
/// and never moving: Collapse all folders, the tree shown or hidden (lit
/// while it shows, ⇧⌘Y) and the pane's ⋯ ([filesMenu]). Left of them,
/// while a file shows, its path line ([PathLine], a click copies it) over
/// the viewer, the source button, a page's Reload and Open in browser, and
/// Show in Finder; those buttons fold into the ⋯ where the row is narrow.
/// [start] is the room before the path, so it stays over the viewer.
class FilesBar extends StatelessWidget {
  const new({
    required this.chat,
    required this.daemon,
    required this.shown,
    super.key,
    this.start = 0,
  });

  final ChatState chat;
  final Daemon daemon;
  final ShownFile? shown;
  final double start;

  /// Its height, the hairline under it included.
  static const height = 40.0;

  /// The room a file's button takes in the row.
  static const _button = 30.0;

  /// The least the path keeps beside the file's buttons before they fold.
  static const _pathLeast = 160.0;

  /// The size of the right end's icons, the top row's Expand's.
  static const _icon = 18.0;

  /// The right end's width: its three buttons' boxes, side by side as the
  /// top row's Expand and × are, so the tree's button sits under Expand
  /// and the ⋯ under ×, as in Claude desktop.
  static const double ends = 3 * PaneEndButtons.box;

  @override
  Widget build(BuildContext context) {
    final t = NushiTheme.of(context).t;
    final shown = this.shown;
    return Container(
      height: height,
      decoration: BoxDecoration(
        border: Border(bottom: BorderSide(color: t.line)),
      ),
      padding: const EdgeInsets.only(bottom: 1, right: PaneEndButtons.inset),
      child: ListenableBuilder(
        listenable: Listenable.merge([
          for (final o in viewToggles) o.pref,
          for (final c in viewChoices) c.pref,
          filesTree,
          folderTreeOf(chat),
          ?shown?.watch,
          ?shown?.source,
        ]),
        builder: (context, _) => LayoutBuilder(
          builder: (context, box) {
            final room = box.maxWidth - start - ends;
            final folds = shown != null && room < _least(context, shown);
            return Row(
              children: [
                SizedBox(width: start),
                Expanded(
                  child: shown == null
                      ? const SizedBox.shrink()
                      : _file(context, shown, folds: folds),
                ),
                _ends(
                  context,
                  t,
                  folded: [
                    if (shown != null && folds) ..._folded(context, shown),
                  ],
                ),
              ],
            );
          },
        ),
      ),
    );
  }

  /// The room [shown]'s path and buttons need before the buttons fold.
  double _least(BuildContext context, ShownFile shown) {
    // Its page's actions and Show in Finder; the source button stays.
    final count = shown.actions(context).length + 1;
    return _pathLeast + (count + (shown.switches ? 1 : 0)) * _button;
  }

  /// The shown file's path and buttons, but those [folds] puts in the ⋯.
  Widget _file(BuildContext context, ShownFile shown, {required bool folds}) {
    final actions = shown.actions(context);
    return Row(
      children: [
        Expanded(
          child: Align(
            alignment: Alignment.centerLeft,
            child: PathLine(
              folder: shown.chat.folder,
              path: shown.path,
              top: 0,
              end: 8,
            ),
          ),
        ),
        if (shown.switches)
          SourceButton(
            source: shown.source.value,
            onChanged: (on) => shown.showSource(on: on),
          ),
        if (!folds) ...[
          for (final (icon, tooltip, size, onTap) in actions)
            IconTap(
              icon,
              tooltip: tooltip,
              size: size,
              padding: IconTap.header,
              onTap: onTap,
            ),
          IconTap.line(
            LineIcon.folderOpen,
            tooltip: conventions.reveal,
            size: 14,
            padding: IconTap.header,
            onTap: shown.reveal,
          ),
        ],
        const SizedBox(width: 8),
      ],
    );
  }

  /// The file's buttons as the ⋯'s rows, when they fold.
  List<MenuEntry> _folded(BuildContext context, ShownFile shown) => [
    for (final (icon, tooltip, _, onTap) in shown.actions(context))
      MenuEntry(tooltip, icon: icon, onPick: onTap),
    MenuEntry(
      conventions.reveal,
      line: LineIcon.folderOpen,
      onPick: shown.reveal,
    ),
  ];

  /// Collapse all folders, the tree's button and the ⋯, Claude desktop's.
  Widget _ends(
    BuildContext context,
    Tokens t, {
    required List<MenuEntry> folded,
  }) {
    final tree = daemon.daemonApi >= filesApi;
    final on = tree && filesTree.value;
    const pad = (PaneEndButtons.box - _icon) / 2;
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        IconTap.line(
          LineIcon.collapseAll,
          tooltip: 'Collapse all folders',
          size: _icon,
          padding: pad,
          color: t.text,
          onTap: canCollapseFolders(chat, daemon)
              ? folderTreeOf(chat).collapseAll
              : null,
        ),
        IconTap.line(
          LineIcon.fileTree,
          tooltip: withKey(
            on ? 'Hide file tree' : 'Show file tree',
            keyLabel(fileTreeKey),
          ),
          size: _icon,
          padding: pad,
          color: t.text,
          lit: on,
          onTap: tree ? () => toggleFileTree(chat) : null,
        ),
        // Under it, ending under its right edge, as Claude's does.
        TrailingMenus(
          child: MenuIcon.line(
            LineIcon.more,
            tooltip: 'More',
            size: _icon,
            padding: pad,
            color: t.text,
            filter: false,
            heldOpen: true,
            entries: filesMenu(context, chat, daemon, folded: folded),
          ),
        ),
      ],
    );
  }
}
