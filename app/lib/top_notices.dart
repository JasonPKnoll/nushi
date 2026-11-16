import 'dart:math';

import 'package:flutter/widgets.dart';

import 'package:nushi/panes.dart';
import 'package:nushi/slides.dart';
import 'package:nushi/toast.dart';

/// What floats at the top of a view that isn't a chat, under its header
/// (ADR 0111, 0128): the [bars], centred and at most [FloatBar.width]
/// wide; the [card] (the newer release's) at the right, clear of the
/// header's buttons; and where the toasts show ([ToastAnchor]), centred
/// under the bars. A chat places the same over its transcript (`Beside`'s
/// notices). It covers the main area and takes no pointer but theirs.
class TopNotices extends StatelessWidget {
  const new({required this.bars, required this.card, super.key});

  final Widget bars;
  final Widget card;

  /// The card's room from its column's right edge: the details panel's
  /// side margin, so it lines up with its cards' text.
  static const cardInset = 16.0;

  /// The room between the card and what goes under it.
  static const gap = 8.0;

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
/// width, the top given down; the card at the right; the toasts' anchor
/// centred over the column from its left to its right (the chat's
/// transcript, or the whole width), under the bars. A bar or the
/// toasts that would meet the card, in a narrow window, go under it.
mixin NoticesPlace on MultiChildLayoutDelegate {
  /// Lays the notices out; see [NoticesPlace].
  void placeNotices({
    required double width,
    required double top,
    required double left,
    required double right,
  }) {
    final card = layoutChild(
      NoticeSlot.card,
      BoxConstraints(maxWidth: width),
    );
    final cardLeft = width - TopNotices.cardInset - card.width;
    positionChild(NoticeSlot.card, Offset(cardLeft, top));
    final under = top + card.height + TopNotices.gap;
    // Whether something centred at [centre], [across] wide, meets the card.
    bool meets(double centre, double across) =>
        !card.isEmpty && centre + across / 2 > cardLeft - TopNotices.gap;

    final most = max(0.0, min(FloatBar.width, width - 32));
    final bars = layoutChild(NoticeSlot.bars, BoxConstraints(maxWidth: most));
    final barsTop = !bars.isEmpty && meets(width / 2, bars.width) ? under : top;
    positionChild(NoticeSlot.bars, Offset((width - bars.width) / 2, barsTop));

    layoutChild(NoticeSlot.toasts, BoxConstraints.tight(Size.zero));
    final centre = (left + right) / 2;
    var toasts = bars.isEmpty ? top : max(top, barsTop + bars.height);
    if (meets(centre, ToastHost.width)) toasts = max(toasts, under);
    positionChild(NoticeSlot.toasts, Offset(centre, toasts));
  }
}

class _Place extends MultiChildLayoutDelegate with NoticesPlace {
  @override
  void performLayout(Size size) => placeNotices(
    width: size.width,
    top: PaneHeader.height + FloatBar.inset,
    left: 0,
    right: size.width,
  );

  @override
  bool shouldRelayout(_Place old) => false;
}
