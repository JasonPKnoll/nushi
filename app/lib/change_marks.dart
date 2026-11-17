import 'dart:math';

import 'package:flutter/material.dart';

import 'package:nushi/hover_card.dart';
import 'package:nushi/line_changes.dart';
import 'package:nushi/theme/theme.dart';

// The change marks in a file's source gutter (ADR 0131), as an editor
// draws them: a bar beside added and changed lines, a wedge where lines
// went, and their card of what the last commit had there.

/// The room a change mark takes between a line's number and its text:
/// [markGap] after the number, the bar [markBar] wide, then the rest of
/// the 14 points that were the number's own padding.
const double markGap = 4;
const double markBar = 3;
const double markRoom = 7;

/// How far a removed run's wedge reaches back from the bar's right edge.
const double markWedge = 7;

/// The most of a mark's old lines its card lists.
const markCardLines = 40;

/// A mark's ink: the diff's own colours (`Tokens.diff`'s inserted and
/// modified words', its gutter's changed text for a removal), solid.
Color markColor(Tokens t, MarkKind kind) => switch (kind) {
  MarkKind.added => t.done,
  MarkKind.changed => t.accentInk,
  MarkKind.removed => t.textSecondary,
};

/// One line's change mark, over its number cell: [cell]'s bar at [barX]
/// from its left, the whole line high, its removed run's wedge on its top
/// or bottom edge; hovered (after a moment) or clicked, its card. Not
/// [painted], it's only where the pointer finds it (the editor paints its
/// marks as it lays its lines out: [paintChangeMark]).
class ChangeMark extends StatelessWidget {
  const new({
    required this.cell,
    required this.barX,
    super.key,
    this.painted = true,
  });

  final MarkCell cell;
  final double barX;
  final bool painted;

  @override
  Widget build(BuildContext context) => HoverCard(
    showOnTap: true,
    above: 300,
    card: (_) => ChangeCard(marks: cell.marks),
    child: MouseRegion(
      cursor: SystemMouseCursors.click,
      child: CustomPaint(
        painter: painted
            ? _MarkPainter(cell, barX, NushiTheme.of(context).t)
            : null,
        child: const SizedBox.expand(),
      ),
    ),
  );
}

/// Paints [cell]'s mark in [box], a line's number cell: its bar [x] in,
/// the box's height, and a removed run's wedge on its top or bottom edge,
/// pointing at the text, its tip at the bar's right edge.
void paintChangeMark(
  Canvas canvas,
  Rect box,
  MarkCell cell,
  double x,
  Tokens t,
) {
  final left = box.left + x;
  if (cell.bar case final bar?) {
    canvas.drawRect(
      Rect.fromLTWH(left, box.top, markBar, box.height),
      Paint()..color = markColor(t, bar.kind),
    );
  }
  // The wedge ends at the bar's right edge, reaching back into the gap
  // after the number: the editor's column ends there, and clips past it.
  final wedge = Paint()..color = markColor(t, MarkKind.removed);
  final tip = left + markBar;
  void at(double y) => canvas.drawPath(
    Path()
      ..moveTo(tip - markWedge, y - 5)
      ..lineTo(tip, y)
      ..lineTo(tip - markWedge, y + 5)
      ..close(),
    wedge,
  );
  if (cell.top != null) at(box.top);
  if (cell.bottom != null) at(box.bottom);
}

class _MarkPainter extends CustomPainter {
  new(this.cell, this.x, this.t);

  final MarkCell cell;
  final double x;
  final Tokens t;

  @override
  void paint(Canvas canvas, Size size) =>
      paintChangeMark(canvas, Offset.zero & size, cell, x, t);

  @override
  bool shouldRepaint(_MarkPainter old) =>
      old.cell != cell || old.x != x || old.t != t;
}

/// A change mark's card: what the last commit had at its lines, read-only
/// (not a diff): "Before" over a changed run's old lines, "N lines
/// removed" over a removed run's, "New lines" for an added run.
class ChangeCard extends StatelessWidget {
  const new({required this.marks, super.key});

  final List<LineMark> marks;

  @override
  Widget build(BuildContext context) {
    final theme = NushiTheme.of(context);
    final t = theme.t;
    final code = theme.code(t.text, size: 12);
    String title(LineMark m) => switch (m.kind) {
      MarkKind.added => m.count == 1 ? 'New line' : 'New lines',
      MarkKind.changed => 'Before',
      MarkKind.removed =>
        m.old.length == 1 ? '1 line removed' : '${m.old.length} lines removed',
    };
    return ConstrainedBox(
      constraints: const BoxConstraints(
        minWidth: 180,
        maxWidth: 560,
        maxHeight: 260,
      ),
      child: Container(
        decoration: floatBox(t, radius: NushiTheme.card),
        padding: const EdgeInsets.fromLTRB(12, 8, 12, 10),
        child: SingleChildScrollView(
          child: SelectionArea(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              spacing: 4,
              children: [
                for (final m in marks) ...[
                  SelectionContainer.disabled(
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      spacing: 6,
                      children: [
                        Container(
                          width: 8,
                          height: 8,
                          decoration: BoxDecoration(
                            color: markColor(t, m.kind),
                            // not a corner: a dot
                            shape: BoxShape.circle,
                          ),
                        ),
                        Text(
                          title(m),
                          style: theme.ui(
                            t.textSecondary,
                            size: 12,
                            weight: FontWeight.w600,
                          ),
                        ),
                      ],
                    ),
                  ),
                  if (m.kind == MarkKind.added)
                    Text(
                      'Not in the last commit',
                      style: theme.ui(t.textMuted, size: 12),
                    )
                  else
                    _OldLines(m.old, style: code),
                ],
              ],
            ),
          ),
        ),
      ),
    );
  }
}

/// A run's old lines in the code font, [markCardLines] at most, then how
/// many more; an empty line shows as one.
class _OldLines extends StatelessWidget {
  const new(this.lines, {required this.style});

  final List<String> lines;
  final TextStyle style;

  @override
  Widget build(BuildContext context) {
    final theme = NushiTheme.of(context);
    final t = theme.t;
    // A tab as four columns: the card is too small for the pane's stops.
    final shown = lines.take(markCardLines).join('\n').replaceAll('\t', '    ');
    final more = lines.length - min(lines.length, markCardLines);
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 6),
      decoration: BoxDecoration(
        color: t.codeBlock,
        borderRadius: BorderRadius.circular(NushiTheme.control),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(shown.isEmpty ? ' ' : shown, style: style, softWrap: false),
          if (more > 0)
            SelectionContainer.disabled(
              child: Text(
                '… $more more ${more == 1 ? 'line' : 'lines'}',
                style: theme.ui(t.textMuted, size: 12),
              ),
            ),
        ],
      ),
    );
  }
}
