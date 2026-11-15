import 'dart:math';

import 'package:flutter/rendering.dart';
import 'package:flutter/widgets.dart';

import 'package:nushi/source_spans.dart';
import 'package:nushi/whitespace_marks.dart';
import 'package:re_editor/re_editor.dart';

// Indent guides (the Files pane's ⋯, Claude desktop's and VS Code's): a
// faint line down each indent level's first column, in the source and in
// the editor alike. Only drawn: the text is as it was.

/// The lines a blank line looks through, each way, for the indent it sits
/// in: past that it has none.
const _look = 200;

/// Each of [lines]' indent level: its leading spaces and tabs in columns
/// (a tab to its stop, every [size]), in steps of [size]. A blank line
/// (only spaces and tabs) takes the lesser of the nearest lines' around
/// it with text, within [_look] lines each way (none past the file's
/// start or end, or the look: 0), so a block's guides run on through it.
/// One pass each way.
List<int> indentLevels(List<String> lines, int size) {
  final n = lines.length;
  final own = List<int?>.filled(n, null);
  for (var i = 0; i < n; i++) {
    final line = lines[i];
    var column = 0;
    var at = 0;
    for (; at < line.length; at++) {
      final c = line.codeUnitAt(at);
      if (c == 0x20) {
        column++;
      } else if (c == 0x09) {
        column += tabColumns(column, size);
      } else {
        break;
      }
    }
    if (at < line.length) own[i] = column ~/ size;
  }
  final before = List<int>.filled(n, 0);
  var last = -1;
  for (var i = 0; i < n; i++) {
    if (own[i] != null) {
      last = i;
    } else if (last >= 0 && i - last <= _look) {
      before[i] = own[last]!;
    }
  }
  final levels = List<int>.filled(n, 0);
  var next = -1;
  for (var i = n - 1; i >= 0; i--) {
    if (own[i] case final level?) {
      next = i;
      levels[i] = level;
    } else if (next >= 0 && next - i <= _look) {
      levels[i] = min(before[i], own[next]!);
    }
  }
  return levels;
}

/// The guides' x, from a line's start: each level's first column, [step]
/// (a tab's columns' width) apart, [level] of them.
Iterable<double> guidesAt(int level, double step) sync* {
  for (var k = 0; k < level; k++) {
    yield k * step;
  }
}

/// Paints [level] guides down [height] from [at], a line's text's start.
void paintGuides(
  Canvas canvas,
  Offset at,
  int level,
  double step,
  double height,
  Color color,
) {
  if (level <= 0) return;
  final paint = Paint()
    ..color = color
    ..strokeWidth = 1;
  for (final x in guidesAt(level, step)) {
    // On a whole pixel, so it's one sharp line; the one past the line's
    // start, so a gutter laid over the start doesn't cover it.
    final dx = (at.dx + x).ceilToDouble() + 0.5;
    canvas.drawLine(Offset(dx, at.dy), Offset(dx, at.dy + height), paint);
  }
}

/// A line of the read-only source with its indent guides down its whole
/// height (wrapped, it's several rows high): [level] of them, [step]
/// apart, from where its child's paragraph (wherever below) starts.
class IndentGuides extends SingleChildRenderObjectWidget {
  const new({
    required this.level,
    required this.step,
    required this.color,
    super.key,
    super.child,
  });

  final int level;
  final double step;
  final Color color;

  @override
  RenderObject createRenderObject(BuildContext context) =>
      _RenderGuides(level, step, color);

  @override
  void updateRenderObject(BuildContext context, RenderObject renderObject) =>
      (renderObject as _RenderGuides)
        ..level = level
        ..step = step
        ..color = color;
}

class _RenderGuides extends RenderProxyBox {
  new(this._level, this._step, this._color);

  int _level;
  double _step;
  Color _color;

  int get level => _level;
  set level(int v) {
    if (_level == v) return;
    _level = v;
    markNeedsPaint();
  }

  double get step => _step;
  set step(double v) {
    if (_step == v) return;
    _step = v;
    markNeedsPaint();
  }

  Color get color => _color;
  set color(Color v) {
    if (_color == v) return;
    _color = v;
    markNeedsPaint();
  }

  @override
  void paint(PaintingContext context, Offset offset) {
    // Under the text and its selection: a guide is the faintest thing.
    if (_level > 0) {
      final p = paragraphIn(child);
      if (p != null && p.hasSize) {
        final at = MatrixUtils.transformPoint(
          p.getTransformTo(this),
          Offset.zero,
        );
        paintGuides(
          context.canvas,
          offset + Offset(at.dx, 0),
          _level,
          _step,
          size.height,
          _color,
        );
      }
    }
    super.paint(context, offset);
  }
}

/// The editor's indent guides over it, where re_editor laid its lines out
/// ([lines]), as [WhitespaceOverlay]'s marks: its text area starts at the
/// right edge of [gutter]; the lines' text is [controller]'s, each level
/// [step] wide. Only drawn: clicks go through to the editor.
class IndentGuideOverlay extends StatefulWidget {
  const new({
    required this.lines,
    required this.controller,
    required this.gutter,
    required this.step,
    required this.size,
    required this.color,
    super.key,
  });

  final EditorLines lines;
  final CodeLineEditingController controller;
  final GlobalKey gutter;
  final double step;

  /// The tab size the levels are counted in.
  final int size;
  final Color color;

  @override
  State<IndentGuideOverlay> createState() => _IndentGuideOverlayState();
}

class _IndentGuideOverlayState extends State<IndentGuideOverlay> {
  final GlobalKey _area = GlobalKey();

  /// The levels as last counted, for the lines and the tab size they were
  /// counted from: a scroll repaints without counting again.
  (CodeLines, int, List<int>)? _counted;

  List<int> _levels() {
    final code = widget.controller.codeLines;
    if (_counted case (final c, final s, final levels)
        when identical(c, code) && s == widget.size) {
      return levels;
    }
    final levels = indentLevels([
      for (var i = 0; i < code.length; i++) code[i].text,
    ], widget.size);
    _counted = (code, widget.size, levels);
    return levels;
  }

  @override
  Widget build(BuildContext context) => IgnorePointer(
    child: CustomPaint(
      key: _area,
      size: Size.infinite,
      painter: _EditorGuides(
        lines: widget.lines,
        controller: widget.controller,
        gutter: widget.gutter,
        area: _area,
        step: widget.step,
        size: widget.size,
        color: widget.color,
        levels: _levels,
      ),
    ),
  );
}

class _EditorGuides extends CustomPainter {
  new({
    required this.lines,
    required this.controller,
    required this.gutter,
    required this.area,
    required this.step,
    required this.size,
    required this.color,
    required this.levels,
  }) : super(repaint: Listenable.merge([lines, controller]));

  final EditorLines lines;
  final CodeLineEditingController controller;
  final GlobalKey gutter;
  final GlobalKey area;
  final double step;
  final int size;
  final Color color;

  /// Each line's level, counted once for the lines as they are.
  final List<int> Function() levels;

  @override
  void paint(Canvas canvas, Size box) {
    final laidOut = lines.value;
    final g = gutter.currentContext?.findRenderObject();
    final me = area.currentContext?.findRenderObject();
    if (laidOut == null || g is! RenderBox || me is! RenderBox) return;
    if (!g.hasSize) return;
    final origin = MatrixUtils.transformPoint(
      g.getTransformTo(me),
      Offset(g.size.width, 0),
    );
    final clip = Rect.fromLTRB(origin.dx, origin.dy, box.width, box.height);
    if (clip.isEmpty) return;
    final levels = this.levels();
    canvas
      ..save()
      ..clipRect(clip);
    for (final p in laidOut.paragraphs) {
      if (p.index >= levels.length) continue;
      final at = origin + p.offset + Offset(lines.moved, 0);
      paintGuides(canvas, at, levels[p.index], step, p.height, color);
    }
    canvas.restore();
  }

  @override
  bool shouldRepaint(_EditorGuides old) =>
      old.color != color ||
      old.step != step ||
      old.size != size ||
      !identical(old.lines, lines) ||
      !identical(old.controller, controller);
}
