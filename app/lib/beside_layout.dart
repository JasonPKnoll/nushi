part of 'beside.dart';

/// A pane beside a view, as its [PaneSpec] says, and what it shows.
class SidePane extends PaneSpec {
  new(
    super.name, {
    required super.initial,
    required super.min,
    required super.cap,
    required this.child,
  }) : fixed = false;

  /// One of [spec]'s sizes, showing [child].
  new of(PaneSpec spec, {required this.child})
    : fixed = false,
      super(
        spec.name,
        initial: spec.initial,
        min: spec.min,
        cap: spec.cap,
        wide: spec.wide,
        squeezed: spec.squeezed,
      );

  /// A pane of one [width], with no edge to drag: the details panel
  /// (ADR 0092), which shows or hides only.
  new fixed(super.name, {required double width, required this.child})
    : fixed = true,
      super(initial: width, min: width, cap: width);

  final bool fixed;
  final Widget child;

  double get width => fixed ? initial : saved(paneWidths.value);
}

/// What [Beside] tells its [Beside.head].
class _BesideScope extends InheritedWidget {
  const new({
    required this.glide,
    required this.scene,
    required this.held,
    required super.child,
  });

  final Object glide;
  final Object? scene;
  final bool held;

  @override
  bool updateShouldNotify(_BesideScope old) =>
      glide != old.glide || scene != old.scene || held != old.held;
}

enum _Slot { main, head, foot }

/// Lays [Beside]'s parts out as its build decided ([besideFit]), measuring
/// the panes as laid out, so a pane being resized moves the rest with it;
/// it decides nothing itself.
///
/// The panes run its full height at its right, right to left; left of them
/// the header across the view and the rail, the terminal under them, and
/// between the two the view and, at its right, the rail. Over the main
/// view, those keep [block] (the view's least) and lie under the panes,
/// which cover their right: the same parts in the same places, the panes
/// only further left. Those [going] stay where they were ([at], which it
/// keeps up to date), laid out as they were; so does the rail while it's
/// tucked away.
class _Beside extends MultiChildLayoutDelegate with NoticesPlace {
  new({
    required this.panes,
    required this.rail,
    required this.railShown,
    required this.railWidth,
    required this.squeezed,
    required this.going,
    required this.at,
    required this.widths,
    this.block,
    this.cover,
  });

  final List<String> panes; // left to right
  final String? rail; // the rail's slot, while it's in the tree
  final bool railShown;
  final double railWidth;
  final double? block; // the view's width under the panes, over it
  final Map<String, double> squeezed; // the most each takes, too narrow
  final List<String> going;
  final Map<String, Rect> at;
  final Map<String, double> widths; // each pane's, as last laid out beside
  final String? cover; // the pane over all, the whole of it

  @override
  void performLayout(Size size) {
    final width = size.width;
    final height = size.height;
    final head = hasChild(_Slot.head) ? PaneHeader.height : 0.0;
    double foot(double across) {
      if (!hasChild(_Slot.foot)) return 0;
      final h = layoutChild(
        _Slot.foot,
        BoxConstraints(
          minWidth: across,
          maxWidth: across,
          maxHeight: max(0, height - head),
        ),
      ).height;
      positionChild(_Slot.foot, Offset(0, height - h));
      return h;
    }

    void place(String slot, Rect r) {
      layoutChild(slot, BoxConstraints.tight(r.size));
      positionChild(slot, r.topLeft);
    }

    // The panes run the full height, beside the view or over it.
    var x = width;
    for (final p in panes.reversed) {
      if (p == cover) {
        // It fills it all; the rest keep the room they had beside it, so
        // nothing under it is laid out anew.
        place(p, Offset.zero & size);
        x -= widths[p] ?? 0;
        continue;
      }
      final w = layoutChild(
        p,
        BoxConstraints(
          maxWidth: squeezed[p] ?? width,
          minHeight: height,
          maxHeight: height,
        ),
      ).width;
      widths[p] = w;
      x -= w;
      positionChild(p, Offset(x, 0));
      at[p] = Rect.fromLTWH(x, 0, w, height);
    }
    // The view's header, terminal and rail: beside the panes, the room
    // they leave; over the view, its least, under them.
    final across = block ?? max(0.0, x);
    final bottom = height - foot(across);
    if (head > 0) {
      layoutChild(_Slot.head, BoxConstraints.tight(Size(across, head)));
      positionChild(_Slot.head, Offset.zero);
    }
    final band = max(0.0, bottom - head);
    var main = across;
    if (rail case final r?) {
      if (railShown) {
        main = max(0, across - railWidth);
        place(r, at[r] = Rect.fromLTWH(main, head, railWidth, band));
      } else {
        // Going, or tucked away: where it was, as it was.
        place(
          r,
          at[r] ??
              Rect.fromLTWH(max(0, across - railWidth), head, railWidth, band),
        );
      }
    }
    for (final g in going) {
      place(g, at[g] ?? Rect.fromLTWH(width, 0, 0, height));
    }
    layoutChild(_Slot.main, BoxConstraints.tight(Size(main, band)));
    positionChild(_Slot.main, Offset(0, head));
    // The notices over it all; the card and the toasts over the
    // transcript, what the rail and the panes leave of it (ADR 0128).
    if (hasChild(NoticeSlot.card)) {
      placeNotices(
        width: width,
        top: head + FloatBar.inset,
        left: 0,
        right: cover != null ? width : min(main, x),
      );
    }
  }

  @override
  bool shouldRelayout(_Beside old) => true; // cheap: a few panes
}
