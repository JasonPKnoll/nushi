import 'dart:math';

import 'package:diff_view/diff_view.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/scheduler.dart';

import 'package:nushi/controls.dart';
import 'package:nushi/theme/theme.dart';

/// A diff that takes comments ([child], a `UnifiedDiff` or a
/// `SideBySideDiff` of [controller]) with a small accent "+" in the gutter
/// of the line under the pointer, as GitHub's diff has: it shows that a
/// line takes a comment. It sits in the change marker's column, right of
/// the numbers (where the package draws its own +), so it covers no digit
/// at any width and stays put from line to line; on a changed line it
/// stands in for the marker, whose tint still shows. It moves nothing; a
/// press on it goes through to the gutter below, so a click opens the
/// comment box a click on the number opens and a drag from it selects
/// lines as one over the numbers does. In the side by side view it is on
/// the pane under the pointer, the side a click on a number there
/// comments on. Only with a mouse: nothing hovers on touch.
class DiffCommentButton extends StatefulWidget {
  const new({
    required this.controller,
    required this.unified,
    required this.theme,
    required this.child,
    super.key,
  });

  final DiffViewController controller;

  /// Whether [child] is the unified view (else side by side).
  final bool unified;

  /// The diff's theme: the side-by-side view's divider width.
  final DiffThemeData theme;

  final Widget child;

  /// The button's side.
  static const size = 16.0;

  @override
  State<DiffCommentButton> createState() => _DiffCommentButtonState();
}

class _DiffCommentButtonState extends State<DiffCommentButton> {
  /// The pointer, in this box; null when it's elsewhere.
  final _pointer = ValueNotifier<Offset?>(null);

  /// Bumped when the rows move under a still pointer: the diff scrolled,
  /// or a comment box opened above it.
  final _moved = ValueNotifier<int>(0);

  Listenable? _watched;

  @override
  void initState() {
    super.initState();
    _watch();
  }

  @override
  void didUpdateWidget(DiffCommentButton old) {
    super.didUpdateWidget(old);
    if (old.controller != widget.controller || old.unified != widget.unified) {
      _watch();
    }
  }

  void _watch() {
    _watched?.removeListener(_rowsMoved);
    final c = widget.controller;
    _watched = Listenable.merge([
      c.geometryChanges,
      if (widget.unified)
        c.unifiedScroll
      else ...[
        c.scroll(PaneSide.old),
        c.scroll(PaneSide.neu),
      ],
    ])..addListener(_rowsMoved);
  }

  /// The diff measures its inlays while it lays out, and says so then:
  /// the button follows after that frame.
  void _rowsMoved() {
    if (_pointer.value == null) return;
    if (SchedulerBinding.instance.schedulerPhase == SchedulerPhase.idle) {
      _moved.value++;
      return;
    }
    SchedulerBinding.instance.addPostFrameCallback((_) {
      if (mounted) _moved.value++;
    });
  }

  @override
  void dispose() {
    _watched?.removeListener(_rowsMoved);
    _pointer.dispose();
    _moved.dispose();
    super.dispose();
  }

  /// The pane under [at]: its side, its left edge in this box.
  (PaneSide, double) _pane(Offset at, double width) {
    if (widget.unified) return (PaneSide.neu, 0);
    // Two equal panes either side of the divider (`SideBySideDiff`).
    final pane = (width - widget.theme.dividerWidth) / 2;
    if (at.dx < pane) return (PaneSide.old, 0);
    if (at.dx >= width - pane) return (PaneSide.neu, width - pane);
    return (PaneSide.neu, double.nan); // the divider
  }

  /// Where the row under the pointer at [at] starts: its pane's left
  /// edge and its top, or null off a line (an inlay, a fold, the divider).
  Offset? _place(Offset at, double width) {
    final c = widget.controller;
    final (side, left) = _pane(at, width);
    if (left.isNaN) return null;
    final g = widget.unified ? c.unifiedGeometry : c.geometry(side);
    final s = widget.unified ? c.unifiedScroll : c.scroll(side);
    if (!s.hasClients || g.rowCount == 0) return null;
    final pixels = s.position.pixels;
    final hit = g.index.hit(pixels + at.dy);
    final row = hit.slot;
    if (row >= g.rowCount ||
        !g.isRowVisible(row) ||
        hit.dy < g.index.extraAbove(row) ||
        hit.dy >= g.index.extraAbove(row) + g.lineHeight) {
      return null;
    }
    return Offset(left, g.topOfRow(row) - pixels);
  }

  @override
  Widget build(BuildContext context) {
    final c = widget.controller;
    final t = NushiTheme.of(context).t;
    return MouseRegion(
      opaque: false,
      onHover: (e) => _pointer.value = e.localPosition,
      onExit: (_) => _pointer.value = null,
      child: LayoutBuilder(
        builder: (context, box) => Stack(
          fit: StackFit.expand,
          children: [
            widget.child,
            ListenableBuilder(
              listenable: Listenable.merge([_pointer, _moved]),
              builder: (context, _) {
                final at = _pointer.value;
                final place = at == null ? null : _place(at, box.maxWidth);
                if (place == null) return const SizedBox.shrink();
                final res = c.resources;
                final line = res.lineHeight;
                final size = min(DiffCommentButton.size, line);
                // Centred on the marker's column, inside the gutter: a
                // press anywhere on it is one on the gutter.
                final m = widget.unified ? res.unifiedGutter : res.gutter;
                final x = min(
                  m.markerX + (res.advance - size) / 2,
                  m.gutterWidth - size,
                );
                return Positioned(
                  left: place.dx + x,
                  top: place.dy + (line - size) / 2,
                  child: _SeeThrough(
                    child: tooltip(
                      context,
                      'Comment on this line',
                      MouseRegion(
                        cursor: SystemMouseCursors.click,
                        child: IgnorePointer(
                          child: Container(
                            key: const ValueKey('diff-comment-button'),
                            width: size,
                            height: size,
                            decoration: BoxDecoration(
                              color: t.accent,
                              borderRadius: BorderRadius.circular(
                                NushiTheme.tag,
                              ),
                            ),
                            child: Icon(
                              Icons.add,
                              size: size - 4,
                              color: t.onAccent,
                            ),
                          ),
                        ),
                      ),
                    ),
                  ),
                );
              },
            ),
          ],
        ),
      ),
    );
  }
}

/// Its child is hit (hovered: the tooltip, the cursor) and so is what's
/// under it: a press reaches the line numbers below.
class _SeeThrough extends SingleChildRenderObjectWidget {
  const new({required super.child});

  @override
  RenderObject createRenderObject(BuildContext context) => _RenderSeeThrough();
}

class _RenderSeeThrough extends RenderProxyBox {
  @override
  bool hitTest(BoxHitTestResult result, {required Offset position}) {
    super.hitTest(result, position: position);
    return false;
  }
}
