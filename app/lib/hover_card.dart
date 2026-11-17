import 'dart:async';

import 'package:flutter/material.dart';

/// Shows [card] floating under [child] after the pointer has been on it
/// for 400 ms ([onShow] runs then), held while the pointer is on either.
/// With [showOnTap] a click on [child] shows it at once, and it stays
/// until a second click, or one elsewhere. With [above], it opens over
/// [child] instead when the window has less than that much room under it.
class HoverCard extends StatefulWidget {
  const new({
    required this.child,
    required this.card,
    super.key,
    this.onShow,
    // design: a hover card's wait, as a tooltip's
    this.delay = const Duration(milliseconds: 400),
    this.showOnTap = false,
    this.above,
  });

  final Widget child;
  final WidgetBuilder card;
  final VoidCallback? onShow;
  final Duration delay;
  final bool showOnTap;
  final double? above;

  @override
  State<HoverCard> createState() => _HoverCardState();
}

class _HoverCardState extends State<HoverCard> {
  final _portal = OverlayPortalController();
  final _link = LayerLink();
  Timer? _wait;
  Timer? _leave;

  /// Whether a click showed it: then the pointer leaving doesn't hide it.
  bool _tapped = false;

  void _enter() {
    _leave?.cancel();
    if (_portal.isShowing) return;
    _wait ??= Timer(widget.delay, _show);
  }

  void _show() {
    _wait?.cancel();
    _wait = null;
    if (!mounted || _portal.isShowing) return;
    widget.onShow?.call();
    _portal.show();
  }

  void _hide() {
    _tapped = false;
    if (mounted && _portal.isShowing) _portal.hide();
  }

  void _tap() {
    _leave?.cancel();
    if (_tapped) return _hide();
    _tapped = true;
    _show();
  }

  // A moment's grace, so the pointer can cross the gap to the card.
  void _exit() {
    _wait?.cancel();
    _wait = null;
    _leave?.cancel();
    if (_tapped) return;
    // design: a hover card's linger, as a tooltip's
    _leave = Timer(const Duration(milliseconds: 150), _hide);
  }

  @override
  void dispose() {
    _wait?.cancel();
    _leave?.cancel();
    super.dispose();
  }

  /// Whether it opens over its child: [HoverCard.above] asks, and the
  /// window has less room than that under the child.
  bool _over() {
    final room = widget.above;
    final box = context.findRenderObject();
    if (room == null || box is! RenderBox || !box.hasSize) return false;
    final bottom = box.localToGlobal(Offset(0, box.size.height)).dy;
    return MediaQuery.sizeOf(context).height - bottom < room;
  }

  @override
  Widget build(BuildContext context) {
    Widget child = MouseRegion(
      onEnter: (_) => _enter(),
      onExit: (_) => _exit(),
      child: widget.child,
    );
    if (widget.showOnTap) {
      child = TapRegion(
        groupId: this,
        onTapOutside: (_) => _hide(),
        child: GestureDetector(
          behavior: HitTestBehavior.opaque,
          onTap: _tap,
          child: child,
        ),
      );
    }
    return CompositedTransformTarget(
      link: _link,
      child: OverlayPortal(
        controller: _portal,
        overlayChildBuilder: (context) {
          final over = _over();
          Widget card = MouseRegion(
            onEnter: (_) => _enter(),
            onExit: (_) => _exit(),
            child: widget.card(context),
          );
          if (widget.showOnTap) card = TapRegion(groupId: this, child: card);
          return CompositedTransformFollower(
            link: _link,
            targetAnchor: over ? Alignment.topLeft : Alignment.bottomLeft,
            followerAnchor: over ? Alignment.bottomLeft : Alignment.topLeft,
            offset: Offset(0, over ? -4 : 4),
            child: Align(
              alignment: over ? Alignment.bottomLeft : Alignment.topLeft,
              child: card,
            ),
          );
        },
        child: child,
      ),
    );
  }
}
