import 'package:flutter/material.dart';

import 'package:nushi/controls.dart';
import 'package:nushi/conventions.dart';
import 'package:nushi/file_view.dart';
import 'package:nushi/menu.dart';
import 'package:nushi/panes.dart';
import 'package:nushi/preview_marks.dart';
import 'package:nushi/preview_path.dart';
import 'package:nushi/settings.dart';
import 'package:nushi/theme/theme.dart';

/// The Files pane's second row (ADR 0123), across the whole pane with a
/// hairline under it: while a file shows, its path line ([PathLine], a
/// click copies it) over the viewer, then wrapping while a source shows,
/// the source button, a page's Reload and Open in browser, and Show in
/// Finder;
/// folded into a ⋯ where the row is narrow. [start] is the room before the
/// path, so it stays over the viewer.
class FilesBar extends StatelessWidget {
  const new({required this.shown, super.key, this.start = 0});

  final ShownFile? shown;
  final double start;

  /// Its height, the hairline under it included.
  static const height = 40.0;

  /// The room a button takes in the row.
  static const _button = 30.0;

  /// The least the path keeps beside the buttons before they fold.
  static const _pathLeast = 160.0;

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
      child: shown == null
          ? null
          : Padding(
              padding: EdgeInsets.only(left: start),
              child: ListenableBuilder(
                listenable: Listenable.merge([
                  shown.watch,
                  shown.source,
                  wrapLines,
                ]),
                builder: (context, _) => LayoutBuilder(
                  builder: (context, box) => _row(context, shown, box.maxWidth),
                ),
              ),
            ),
    );
  }

  Widget _row(BuildContext context, ShownFile shown, double width) {
    final actions = shown.actions(context);
    final wraps = shown.showsSource;
    final count = actions.length + (wraps ? 1 : 0) + 1;
    final switches = shown.switches;
    final folds = width < _pathLeast + (count + (switches ? 1 : 0)) * _button;
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
        // Wrapping, while a source shows, comes and goes left of the
        // source button, which never moves for it.
        if (wraps && !folds) const WrapButton(),
        if (switches)
          SourceButton(
            source: shown.source.value,
            onChanged: (on) => shown.showSource(on: on),
          ),
        if (folds)
          MenuIcon.line(
            LineIcon.more,
            tooltip: 'More',
            padding: IconTap.header,
            filter: false,
            entries: [
              if (wraps)
                MenuEntry(
                  wrapLabel(wrap: wrapLines.value),
                  icon: Icons.wrap_text,
                  onPick: toggleWrap,
                ),
              for (final (icon, tooltip, _, onTap) in actions)
                MenuEntry(tooltip, icon: icon, onPick: onTap),
              MenuEntry(
                conventions.reveal,
                line: LineIcon.folderOpen,
                onPick: shown.reveal,
              ),
            ],
          )
        else ...[
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
      ],
    );
  }
}
