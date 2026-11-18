import 'dart:math';

import 'package:flutter/material.dart';

import 'package:nushi/hover_card.dart';
import 'package:nushi/line_changes.dart';
import 'package:nushi/run_state.dart';
import 'package:nushi/theme/theme.dart';

// The change marks in a file's source gutter (ADR 0131), as an editor
// draws them: a bar beside added and changed lines, a wedge where lines
// went, and their card of what was there before the chat's branch (or the
// last commit, with no branch base).

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

/// A mark's ink, solid: the diff's inserted and modified words' colours,
/// and for a removal red (`crashed`), as an editor's gutter has it: a
/// grey wedge that small was easy to miss.
Color markColor(Tokens t, MarkKind kind) => switch (kind) {
  MarkKind.added => t.done,
  MarkKind.changed => t.accentInk,
  MarkKind.removed => t.crashed,
};

/// One line's change mark, over its number cell: [cell]'s bar at [barX]
/// from its left, the whole line high, its removed run's wedge on its top
/// or bottom edge; hovered (after a moment) or clicked, its card. Not
/// [painted], it's only where the pointer finds it (the editor paints its
/// marks as it lays its lines out: [paintChangeMark]). The card keeps
/// clear of its whole run's lines on screen, so what they say now stays
/// in sight beside what they were.
class ChangeMark extends StatefulWidget {
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
  State<ChangeMark> createState() => _ChangeMarkState();
}

class _ChangeMarkState extends State<ChangeMark> {
  /// The marks on screen, to find a run's other lines by: a run's lines
  /// share its one [LineMark] (`markCells`), each view its own.
  static final Set<_ChangeMarkState> _shown = runSet();

  @override
  void initState() {
    super.initState();
    _shown.add(this);
  }

  @override
  void dispose() {
    _shown.remove(this);
    super.dispose();
  }

  Rect? _box() {
    final box = context.findRenderObject();
    if (box is! RenderBox || !box.attached || !box.hasSize) return null;
    return box.localToGlobal(Offset.zero) & box.size;
  }

  /// This line's box and its run's other lines' on screen (a removal's
  /// wedge is its line's alone).
  Rect _around() {
    final own = _box() ?? Rect.zero;
    final run = widget.cell.bar;
    if (run == null) return own;
    var all = own;
    for (final other in _shown) {
      if (!identical(other.widget.cell.bar, run)) continue;
      if (other._box() case final box?) all = all.expandToInclude(box);
    }
    return all;
  }

  @override
  Widget build(BuildContext context) {
    final branch = MarksAgainst.branchOf(context);
    return HoverCard(
      showOnTap: true,
      above: 300,
      around: _around,
      card: (_) => ChangeCard(marks: widget.cell.marks, branch: branch),
      child: MouseRegion(
        cursor: SystemMouseCursors.click,
        child: CustomPaint(
          painter: widget.painted
              ? _MarkPainter(widget.cell, widget.barX, NushiTheme.of(context).t)
              : null,
          child: const SizedBox.expand(),
        ),
      ),
    );
  }
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

/// What a file's change marks below it count from (ADR 0131): the text
/// before the chat's branch ([branch]), or the last commit's.
class MarksAgainst extends InheritedWidget {
  const new({required this.branch, required super.child, super.key});

  final bool branch;

  /// Whether the marks at [context] count from before the branch; false
  /// with none above.
  static bool branchOf(BuildContext context) =>
      context.dependOnInheritedWidgetOfExactType<MarksAgainst>()?.branch ??
      false;

  @override
  bool updateShouldNotify(MarksAgainst old) => old.branch != branch;
}

/// A change mark's card: what the base had at its lines, read-only (not a
/// diff): "Before this branch" ([branch]) or "Before" (the last commit)
/// over a changed run's old lines, "N lines removed" over a removed run's,
/// "New lines" for an added run.
class ChangeCard extends StatelessWidget {
  const new({required this.marks, super.key, this.branch = false});

  final List<LineMark> marks;
  final bool branch;

  @override
  Widget build(BuildContext context) {
    final theme = NushiTheme.of(context);
    final t = theme.t;
    final code = theme.code(t.text, size: 12);
    String title(LineMark m) => switch (m.kind) {
      MarkKind.added => m.count == 1 ? 'New line' : 'New lines',
      MarkKind.changed => branch ? 'Before this branch' : 'Before',
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
                      branch
                          ? 'Added on this branch'
                          : 'Not in the last commit',
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
          // Wrapped: a long line is read whole, not cut at the card's edge.
          Text(shown.isEmpty ? ' ' : shown, style: style),
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
