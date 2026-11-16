import 'dart:math';

import 'package:flutter/widgets.dart';

import 'package:nushi/panes.dart';
import 'package:nushi/slides.dart';
import 'package:nushi/toast.dart';

/// What floats at the top of the main area, under its header (ADR 0111,
/// 0128): the [bars], centred and at most [FloatBar.width] wide; the
/// [card] (the newer release's) at the right, [cardInset] in, clear of the
/// header's buttons; and where the toasts show ([ToastAnchor]), centred
/// under the bars. One that would meet the card, in a narrow window, goes
/// under it instead. It covers the main area and takes no pointer but
/// theirs.
class TopNotices extends StatelessWidget {
  const new({required this.bars, required this.card, super.key});

  final Widget bars;
  final Widget card;

  /// The card's room from the main area's right edge: the details panel's
  /// side margin, so it lines up with its cards' text.
  static const cardInset = 16.0;

  /// The room between the card and what goes under it.
  static const gap = 8.0;

  @override
  Widget build(BuildContext context) => CustomMultiChildLayout(
    delegate: _Place(),
    children: [
      LayoutId(id: _Slot.card, child: card),
      LayoutId(id: _Slot.bars, child: bars),
      LayoutId(id: _Slot.toasts, child: const ToastAnchor()),
    ],
  );
}

enum _Slot { card, bars, toasts }

class _Place extends MultiChildLayoutDelegate {
  @override
  void performLayout(Size size) {
    const top = PaneHeader.height + FloatBar.inset;
    final card = layoutChild(_Slot.card, BoxConstraints.loose(size));
    final cardLeft = size.width - TopNotices.cardInset - card.width;
    positionChild(_Slot.card, Offset(cardLeft, top));
    final under = top + card.height + TopNotices.gap;
    // Whether something centred, [width] wide, would meet the card.
    bool meets(double width) =>
        !card.isEmpty && (size.width + width) / 2 > cardLeft - TopNotices.gap;

    final most = max(0.0, min(FloatBar.width, size.width - 32));
    final bars = layoutChild(_Slot.bars, BoxConstraints(maxWidth: most));
    final barsTop = !bars.isEmpty && meets(bars.width) ? under : top;
    positionChild(_Slot.bars, Offset((size.width - bars.width) / 2, barsTop));

    layoutChild(_Slot.toasts, BoxConstraints.tight(Size.zero));
    var toasts = bars.isEmpty ? top : barsTop + bars.height;
    if (meets(ToastHost.width)) toasts = max(toasts, under);
    positionChild(_Slot.toasts, Offset(size.width / 2, toasts));
  }

  @override
  bool shouldRelayout(_Place old) => false;
}
