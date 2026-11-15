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
/// panes (the view's and the rail's; over the view, all of it), each
/// pane's width, and the most each may be dragged to.
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
/// - `over` from [room] and the panes' least widths alone: they lie over
///   the view only once, every one of them at its least, the view would
///   still keep less than [least]. Neither a drag nor a saved width puts
///   them over it or takes them off it.
/// - beside the view, each pane's width and its most from [room], the
///   [live] widths (a drag under way) and the saved ones. Where those leave
///   the view less than [least], the panes narrow, never saved, until it
///   keeps [least]: the widest gives first, down to its least, then the
///   next ([_share]); one being dragged gives last. Its saved width is back
///   as soon as there's room. A pane may be dragged until the view keeps
///   [least] beside the rest as they are; `Resizable` clamps to the same
///   most, so the two agree.
/// - over the view, every pane at its least, its edge stopping there.
/// - the rail from the room the panes leave: it shows while the view keeps
///   [railLeast] beside it (at the line exactly, it shows). It is an input
///   to nothing, so showing or hiding it never moves a pane.
///
/// [railLeast] is at most [least], so the rail has always gone before the
/// panes narrow, and they narrow before they lie over the view.
BesideFit besideFit({
  required double room,
  required List<PaneSpec> panes,
  required Map<String, double> saved,
  required bool wantRail,
  required double least,
  required double railLeast,
  required double rail,
  Map<String, double> live = const {},
}) {
  assert(railLeast <= least, 'the rail goes before the panes narrow');
  var leastSum = 0.0;
  for (final p in panes) {
    leastSum += p.min;
  }
  if (panes.isNotEmpty && room - leastSum < least) {
    final at = {for (final p in panes) p.name: p.min};
    return (over: true, rail: false, left: room, widths: at, most: {...at});
  }
  final avail = room - least;
  double base(PaneSpec p, Map<String, double> live) =>
      (live[p.name] ?? p.saved(saved)).clamp(p.min, p.cap);
  // Each one's most: the room the rest leave, as they'd be with it at its
  // saved width, its own drag aside.
  final most = <String, double>{};
  for (final p in panes) {
    final others = {...live}..remove(p.name);
    final fixed = _share(avail, panes, {
      for (final q in panes) q.name: base(q, others),
    }, others.keys.toSet());
    var rest = 0.0;
    for (final q in panes) {
      if (q != p) rest += fixed[q.name]!;
    }
    most[p.name] = min(p.cap, avail - rest);
  }
  final widths = _share(avail, panes, {
    for (final p in panes)
      p.name: live.containsKey(p.name)
          ? base(p, live).clamp(p.min, max(p.min, most[p.name]!))
          : base(p, live),
  }, live.keys.toSet());
  var used = 0.0;
  for (final w in widths.values) {
    used += w;
  }
  final left = room - used;
  return (
    over: false,
    rail: wantRail && left - rail >= railLeast,
    left: left,
    widths: widths,
    most: most,
  );
}

/// [wants], each pane's width, narrowed until together they take at most
/// [room]: the widest gives first, down to its least, then the next widest
/// (the rightmost first of two alike); those [last] (being dragged) give
/// only after the rest. In the panes' order.
Map<String, double> _share(
  double room,
  List<PaneSpec> panes,
  Map<String, double> wants,
  Set<String> last,
) {
  final widths = {for (final p in panes) p.name: wants[p.name]!};
  var over = -room;
  for (final w in widths.values) {
    over += w;
  }
  if (over <= 0) return widths;
  final order = [for (var i = panes.length - 1; i >= 0; i--) i]
    ..sort((a, b) {
      final pa = panes[a].name;
      final pb = panes[b].name;
      final byLast = (last.contains(pa) ? 1 : 0) - (last.contains(pb) ? 1 : 0);
      if (byLast != 0) return byLast;
      final byWidth = widths[pb]!.compareTo(widths[pa]!);
      return byWidth != 0 ? byWidth : b.compareTo(a);
    });
  for (final i in order) {
    final p = panes[i];
    final give = min(over, widths[p.name]! - p.min);
    widths[p.name] = widths[p.name]! - give;
    over -= give;
    if (over <= 0) break;
  }
  return widths;
}
