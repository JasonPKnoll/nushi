import 'dart:async';

import 'package:flutter/material.dart';

/// Shows [card] floating under [child] after the pointer has been on it
/// for 400 ms ([onShow] runs then), held while the pointer is on either.
class HoverCard extends StatefulWidget {
  const new({
    required this.child,
    required this.card,
    super.key,
    this.onShow,
    // design: a hover card's wait, as a tooltip's
    this.delay = const Duration(milliseconds: 400),
  });

  final Widget child;
  final WidgetBuilder card;
  final VoidCallback? onShow;
  final Duration delay;

  @override
  State<HoverCard> createState() => _HoverCardState();
}

class _HoverCardState extends State<HoverCard> {
  final _portal = OverlayPortalController();
  final _link = LayerLink();
  Timer? _wait;
  Timer? _leave;

  void _enter() {
    _leave?.cancel();
    if (_portal.isShowing) return;
    _wait ??= Timer(widget.delay, () {
      _wait = null;
      if (!mounted) return;
      widget.onShow?.call();
      _portal.show();
    });
  }

  // A moment's grace, so the pointer can cross the gap to the card.
  void _exit() {
    _wait?.cancel();
    _wait = null;
    _leave?.cancel();
    // design: a hover card's linger, as a tooltip's
    _leave = Timer(const Duration(milliseconds: 150), () {
      if (mounted && _portal.isShowing) _portal.hide();
    });
  }

  @override
  void dispose() {
    _wait?.cancel();
    _leave?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => CompositedTransformTarget(
    link: _link,
    child: OverlayPortal(
      controller: _portal,
      overlayChildBuilder: (context) => CompositedTransformFollower(
        link: _link,
        targetAnchor: Alignment.bottomLeft,
        offset: const Offset(0, 4),
        child: Align(
          alignment: Alignment.topLeft,
          child: MouseRegion(
            onEnter: (_) => _enter(),
            onExit: (_) => _exit(),
            child: widget.card(context),
          ),
        ),
      ),
      child: MouseRegion(
        onEnter: (_) => _enter(),
        onExit: (_) => _exit(),
        child: widget.child,
      ),
    ),
  );
}
