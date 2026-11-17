import 'dart:math';

import 'package:flutter/foundation.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/widgets.dart';

import 'package:nushi/panes.dart';
import 'package:nushi/toast.dart';

/// The bars of the window (ADR 0128, amended 2026-11-17), for the view
/// in the main pane to put in a row of their own under its header
/// ([BarsRow]): one element wherever they're put, so a bar keeps its state
/// and its slide as the view changes. A chat's `Beside` lays them out
/// itself.
class TopBars extends InheritedWidget {
  const new({required this.bars, required super.child, super.key});

  /// None in a chat, which lays them out itself.
  final Widget? bars;

  @override
  bool updateShouldNotify(TopBars old) => bars != old.bars;
}

/// Under a view's header (Jira's, GitHub's, an empty window's), the bars
/// ([TopBars]) across it, pushing the view down as one slides in and giving
/// the room back as it goes; under them, where the toasts show
/// ([ToastAnchor]), centred, never less than [ToastHost.margin] inside it.
/// Nothing outside a [TopBars].
class BarsRow extends StatelessWidget {
  const new({super.key});

  @override
  Widget build(BuildContext context) {
    final bars = context.dependOnInheritedWidgetOfExactType<TopBars>()?.bars;
    if (bars == null) return const SizedBox.shrink();
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        bars,
        // No height: a toast lies over what's under it for its seconds,
        // [ToastHost.margin] under the bars.
        LayoutBuilder(
          builder: (context, box) => Center(
            child: Transform.translate(
              offset: const Offset(0, ToastHost.margin),
              child: SizedBox(
                width: min(
                  ToastHost.width,
                  max(0, box.maxWidth - 2 * ToastHost.margin),
                ),
                height: 0,
                child: const ToastAnchor(),
              ),
            ),
          ),
        ),
      ],
    );
  }
}

/// What floats over a view that isn't a chat (ADR 0128): the [card] (the
/// newer release's) in its bottom right corner. The bars and the toasts
/// are under its header ([BarsRow]); a chat places all three itself
/// (`Beside`'s notices), the card above its composer ([CardLift]). It
/// covers the main area and takes no pointer but the card's.
class TopNotices extends StatelessWidget {
  const new({required this.card, super.key});

  final Widget card;

  /// The card's room from its column's right and bottom edges: the
  /// details panel's side margin, so it lines up with its cards' text.
  static const cardInset = 16.0;

  /// [card], [bars] and the toasts' anchor (with [toasts]) as the children
  /// of a layout whose delegate has [NoticesPlace], which places the card
  /// and the anchor; the bars are the layout's own to place.
  static List<Widget> slots({
    required Widget card,
    Widget? bars,
    bool toasts = true,
  }) => [
    LayoutId(
      key: const ValueKey(NoticeSlot.card),
      id: NoticeSlot.card,
      child: card,
    ),
    if (bars != null)
      LayoutId(
        key: const ValueKey(NoticeSlot.bars),
        id: NoticeSlot.bars,
        child: bars,
      ),
    if (toasts)
      LayoutId(
        key: const ValueKey(NoticeSlot.toasts),
        id: NoticeSlot.toasts,
        child: const ToastAnchor(),
      ),
  ];

  @override
  Widget build(BuildContext context) => CustomMultiChildLayout(
    delegate: _Place(),
    children: slots(card: card, toasts: false),
  );
}

/// The ids of [TopNotices.slots].
enum NoticeSlot { card, bars, toasts }

/// Places [TopNotices.slots] but the bars (ADR 0128): the toasts' anchor
/// at `top` (under the header and the bars' row), centred over the column
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
    if (!hasChild(NoticeSlot.toasts)) return;

    // A toast is at most the main area less its margins: over the column
    // where it fits, else over the whole area, and never out of it.
    final across = min(ToastHost.width, room);
    layoutChild(NoticeSlot.toasts, BoxConstraints.tight(Size(across, 0)));
    final fits = right - left >= across + 2 * margin;
    final least = margin + across / 2;
    final centre = (fits ? (left + right) / 2 : width / 2)
        .clamp(least, max(least, width - least))
        .toDouble();
    positionChild(NoticeSlot.toasts, Offset(centre - across / 2, top));
  }
}

class _Place extends MultiChildLayoutDelegate with NoticesPlace {
  @override
  void performLayout(Size size) => placeNotices(
    width: size.width,
    top: PaneHeader.height + ToastHost.margin,
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
