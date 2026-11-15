import 'dart:math';

/// A pane that can open beside a view: its [name] (its remembered
/// width's), the width it opens at until dragged, and its least and most.
class PaneSpec {
  const new(
    this.name, {
    required this.initial,
    required this.min,
    required this.cap,
  });

  final String name;
  final double initial;
  final double min;
  final double cap;

  /// Its width as remembered in [widths] (`paneWidths`), or [initial],
  /// within its least and most.
  double saved(Map<String, double> widths) =>
      (widths[name] ?? initial).clamp(min, cap);
}

/// What [besideFit] works out: whether the panes lie `over` the view or
/// beside it, whether the details `rail` shows, the room `left` of the
/// panes (the view's and the rail's), each pane's width, and the most each
/// may be dragged to.
typedef BesideFit = ({
  bool over,
  bool rail,
  double left,
  Map<String, double> widths,
  Map<String, double> most,
});

/// How a chat's room is shared (ADR 0123): left to right, the view, the
/// details rail ([rail] wide, while [wantRail]) and [panes].
///
/// Each step reads only what the ones before it gave, so nothing it decides
/// feeds back into itself:
/// - `over` from [room] and the panes' saved widths alone, never their
///   [live] ones: a drag never puts the panes over the view or takes them
///   off it until it's let go, and so the saved width it leaves decides.
/// - each pane's width and its most from [room], the [live] widths (a drag
///   under way), the saved ones and `over`. Beside the view a pane may be
///   dragged until the view keeps [least]; over it, until a [strip] of it
///   is left. These are `Resizable`'s own clamps, so the two agree.
/// - the rail from the room the panes leave: it shows while the view keeps
///   [railLeast] beside it (at the line exactly, it shows). It is an input
///   to nothing, so showing or hiding it never moves a pane.
///
/// [railLeast] is at most [least], so the rail has always gone before the
/// panes lie over the view.
BesideFit besideFit({
  required double room,
  required List<PaneSpec> panes,
  required Map<String, double> saved,
  required bool wantRail,
  required double least,
  required double railLeast,
  required double rail,
  Map<String, double> live = const {},
  double strip = 48,
}) {
  assert(railLeast <= least, 'the rail goes before the panes lie over');
  var savedSum = 0.0;
  for (final p in panes) {
    savedSum += p.saved(saved);
  }
  final over = panes.isNotEmpty && room - savedSum < least;
  final reserve = over ? strip : least;
  double base(PaneSpec p) =>
      (live[p.name] ?? p.saved(saved)).clamp(p.min, p.cap);
  var baseSum = 0.0;
  for (final p in panes) {
    baseSum += base(p);
  }
  final widths = <String, double>{};
  final most = <String, double>{};
  var used = 0.0;
  for (final p in panes) {
    final m = min(p.cap, room - (baseSum - base(p)) - reserve);
    most[p.name] = m;
    used += widths[p.name] = base(p).clamp(p.min, max(p.min, m));
  }
  final left = over ? room : room - used;
  return (
    over: over,
    rail: wantRail && !over && left - rail >= railLeast,
    left: left,
    widths: widths,
    most: most,
  );
}
