import 'dart:math';

import 'package:flutter/foundation.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/widgets.dart';

import 'package:nushi/panes.dart';
import 'package:nushi/slides.dart';
import 'package:nushi/toast.dart';

/// What floats over a view that isn't a chat (ADR 0111, 0128): at the
/// top, under its header, the [bars], centred and at most
/// [FloatBar.width] wide, and where the toasts show ([ToastAnchor]),
/// centred under them; the [card] (the newer release's) in its bottom
/// right corner. A chat places the same over its transcript (`Beside`'s
/// notices), the card above its composer ([CardLift]). It covers the main
/// area and takes no pointer but theirs.
class TopNotices extends StatelessWidget {
  const new({required this.bars, required this.card, super.key});

  final Widget bars;
  final Widget card;

  /// The card's room from its column's right and bottom edges: the
  /// details panel's side margin, so it lines up with its cards' text.
  static const cardInset = 16.0;

  /// [bars], [card] and the toasts' anchor as the children of a layout
  /// whose delegate has [NoticesPlace].
  static List<Widget> slots({required Widget bars, required Widget card}) => [
    LayoutId(
      key: const ValueKey(NoticeSlot.card),
      id: NoticeSlot.card,
      child: card,
    ),
    LayoutId(
      key: const ValueKey(NoticeSlot.bars),
      id: NoticeSlot.bars,
      child: bars,
    ),
    LayoutId(
      key: const ValueKey(NoticeSlot.toasts),
      id: NoticeSlot.toasts,
      child: const ToastAnchor(),
    ),
  ];

  @override
  Widget build(BuildContext context) => CustomMultiChildLayout(
    delegate: _Place(),
    children: slots(bars: bars, card: card),
  );
}

/// The ids of [TopNotices.slots].
enum NoticeSlot { card, bars, toasts }

/// Places [TopNotices.slots] (ADR 0128): the bars centred across the
/// width at `top`; the toasts' anchor under them, centred over the column
/// from `left` to `right` (the chat's transcript, or the whole width), or
/// over the whole width where a toast doesn't fit the column; the card in
/// the column's bottom right corner, above `bottom`. A toast and the card
/// stay 8 points inside the width, a toast narrowed to fit; the card never
/// above `top`. Where the panes lie over the view from `under` on and the
/// card doesn't fit left of them, it's under them, as the view is, and
/// hidden (`cardShown`): never a sliver of it, never over their rows.
mixin NoticesPlace on MultiChildLayoutDelegate {
  /// Lays the notices out; see [NoticesPlace].
  void placeNotices({
    required double width,
    required double top,
    required double bottom,
    required double left,
    required double right,
    double? under,
    ValueNotifier<bool>? cardShown,
  }) {
    const margin = ToastHost.margin;
    final room = max(0.0, width - 2 * margin);
    final card = layoutChild(NoticeSlot.card, BoxConstraints(maxWidth: room));
    // At the column's right, but never out of the main area.
    var cardLeft = max(
      margin,
      min(right - TopNotices.cardInset, width - margin) - card.width,
    );
    final hidden = under != null && cardLeft + card.width > under;
    if (hidden) cardLeft = max(margin, min(under, width - margin - card.width));
    cardShown?.value = !hidden;
    final cardTop = max(top, bottom - TopNotices.cardInset - card.height);
    positionChild(NoticeSlot.card, Offset(cardLeft, cardTop));

    final most = max(0.0, min(FloatBar.width, width - 32));
    final bars = layoutChild(NoticeSlot.bars, BoxConstraints(maxWidth: most));
    positionChild(NoticeSlot.bars, Offset((width - bars.width) / 2, top));

    // A toast is at most the main area less its margins: over the column
    // where it fits, else over the whole area, and never out of it.
    final across = min(ToastHost.width, room);
    layoutChild(NoticeSlot.toasts, BoxConstraints.tight(Size(across, 0)));
    final fits = right - left >= across + 2 * margin;
    final least = margin + across / 2;
    final centre = (fits ? (left + right) / 2 : width / 2)
        .clamp(least, max(least, width - least))
        .toDouble();
    positionChild(
      NoticeSlot.toasts,
      Offset(centre - across / 2, top + bars.height),
    );
  }
}

class _Place extends MultiChildLayoutDelegate with NoticesPlace {
  @override
  void performLayout(Size size) => placeNotices(
    width: size.width,
    top: PaneHeader.height + FloatBar.inset,
    bottom: size.height,
    left: 0,
    right: size.width,
  );

  @override
  bool shouldRelayout(_Place old) => false;
}

/// Where the chat area's main view tells how tall the foot of its
/// transcript is (its turn status and composer, `OverBottom`), as last
/// laid out, for the update card to stand above it ([CardLift]). A side
/// pane's transcript isn't under it.
class FootRoom extends InheritedWidget {
  const new({required this.room, required super.child, super.key});

  /// The foot's height.
  final ValueNotifier<double> room;

  /// The [FootRoom] [context] is in, if any.
  static ValueNotifier<double>? maybeOf(BuildContext context) =>
      context.dependOnInheritedWidgetOfExactType<FootRoom>()?.room;

  @override
  bool updateShouldNotify(FootRoom old) => room != old.room;
}

/// [child] drawn and hit [by] higher than it's laid out, and only while
/// [shown] says so: the update card over a chat's transcript, above its
/// composer, which grows as it's typed in, and not at all while it's under
/// the panes ([NoticesPlace]). Both are read as it paints, after the
/// composer's and the panes' layout: the card is never a frame behind
/// them, and nothing is laid out again.
class CardLift extends SingleChildRenderObjectWidget {
  const new({this.by, this.shown, super.child, super.key});

  /// How high; not at all while null.
  final ValueListenable<double>? by;

  /// Whether it shows; always while null.
  final ValueListenable<bool>? shown;

  @override
  RenderObject createRenderObject(BuildContext context) => _Lifted(by, shown);

  @override
  void updateRenderObject(BuildContext context, RenderObject renderObject) =>
      (renderObject as _Lifted).follow(by, shown);
}

class _Lifted extends RenderProxyBox {
  new(this._by, this._shown);

  ValueListenable<double>? _by;
  ValueListenable<bool>? _shown;

  void follow(ValueListenable<double>? by, ValueListenable<bool>? shown) {
    if (by == _by && shown == _shown) return;
    if (attached) _listen(false);
    _by = by;
    _shown = shown;
    if (attached) _listen(true);
    _moved();
  }

  void _listen(bool on) {
    for (final l in <Listenable?>[_by, _shown]) {
      on ? l?.addListener(_moved) : l?.removeListener(_moved);
    }
  }

  double get _lift => _by?.value ?? 0;
  bool get _visible => _shown?.value ?? true;

  void _moved() {
    markNeedsPaint();
    markNeedsSemanticsUpdate();
  }

  @override
  void attach(PipelineOwner owner) {
    super.attach(owner);
    _listen(true);
  }

  @override
  void detach() {
    _listen(false);
    super.detach();
  }

  @override
  void paint(PaintingContext context, Offset offset) {
    if (_visible) super.paint(context, offset.translate(0, -_lift));
  }

  // No check of its own box: drawn higher, the card is out of it, and a
  // lifted card's box is out of its parent's (one is in another).
  @override
  bool hitTest(BoxHitTestResult result, {required Offset position}) {
    final card = child;
    if (!_visible || card == null) return false;
    final hit = result.addWithPaintOffset(
      offset: Offset(0, -_lift),
      position: position,
      hitTest: (result, p) => card.hitTest(result, position: p),
    );
    if (hit) result.add(BoxHitTestEntry(this, position));
    return hit;
  }

  @override
  void visitChildrenForSemantics(RenderObjectVisitor visitor) {
    if (_visible) super.visitChildrenForSemantics(visitor);
  }

  @override
  void applyPaintTransform(RenderObject child, Matrix4 transform) =>
      transform.translateByDouble(0, -_lift, 0, 1);
}
