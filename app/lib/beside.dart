import 'dart:math';

import 'package:flutter/material.dart';

import 'package:nushi/beside_fit.dart';
import 'package:nushi/panes.dart';
import 'package:nushi/settings.dart';
import 'package:nushi/sidebar_button.dart';
import 'package:nushi/slides.dart';
import 'package:nushi/theme/theme.dart';

part 'beside_layout.dart';

/// A chat's room, left to right (ADR 0123): [main], the [details] rail
/// while it's wanted (a [SidePane.fixed], a hairline at its left), and
/// [panes], each resizable; [head] over [main] and the rail,
/// [PaneHeader.height] high, and [foot] under them (the chat's header and
/// its terminal). The panes run its full height, their own top rows level
/// with [head].
///
/// How the room is shared is [besideFit]'s: when [main] would keep less
/// than [railLeast] beside the rail, the rail tucks itself away (kept,
/// not drawn; never saved, as it's still wanted), and when [main] would
/// keep less than [least] beside the panes alone, they lie over it, under
/// [head], [head] and [foot] then across all of it. A pane's edge, dragged
/// while the panes are beside [main], stops where [main] keeps [least];
/// the rail tucks and comes back as the edge moves.
///
/// A pane slides its full width in from the right as it opens and out as
/// it closes ([Presence], [paneSlide]), and so does the rail as it's shown,
/// hidden, tucked or brought back; one going keeps its place until it's
/// gone, taking no pointer or focus, while the rest are laid out at once
/// where they go. [main] is laid out once at its new width and drawn
/// gliding there ([Glide]) from where its text was, [columnSides] saying
/// where that text's column lies in it (the chat's); so do the panes that
/// stay but move, and [head]'s trailing buttons ([Beside.glideOf]). The
/// rail tucked with the focus in it gives the focus to [keep].
///
/// A new [scene] (another of the chat's tabs) is a new set of [main], its
/// panes and details: they're there at once, with no slide, while [head]
/// and [foot] stay the same widgets (a terminal keeps its shell).
///
/// The pane named [cover] takes the whole of it, over the rest, [head] and
/// [foot] too (an expanded browser tab, ADR 0094): they stay laid out where
/// they were, keeping their state, but draw nothing and take no pointer or
/// focus, so the pane is the same widget throughout and its page doesn't
/// load again.
class Beside extends StatefulWidget {
  const new({
    required this.main,
    required this.least,
    required this.panes,
    super.key,
    this.head,
    this.foot,
    this.details,
    double? railLeast,
    this.columnSides,
    this.cover,
    this.scene,
    this.keep,
  }) : railLeast = railLeast ?? least;

  final Widget main;
  final Widget? head;
  final Widget? foot;
  final double least;
  final double railLeast;
  final List<SidePane> panes;
  final SidePane? details;
  final EdgeInsets Function(double width)? columnSides;
  final String? cover;
  final Object? scene;
  final FocusNode? keep;

  /// What [head]'s trailing part glides by: it changes as the panes beside
  /// it come and go (and the rail and the sidebar), so a [Glide] given it
  /// slides with them; null outside a [Beside].
  static Object? glideOf(BuildContext context) => _scope(context)?.glide;

  /// The [Beside.scene] [head] is in: a [Glide] given it jumps when it
  /// changes, as the rest do.
  static Object? sceneOf(BuildContext context) => _scope(context)?.scene;

  /// Whether the rail is wanted but tucked away for the panes' room.
  static bool railHeldOf(BuildContext context) =>
      _scope(context)?.held ?? false;

  static _BesideScope? _scope(BuildContext context) =>
      context.dependOnInheritedWidgetOfExactType<_BesideScope>();

  @override
  State<Beside> createState() => _BesideState();
}

class _BesideState extends State<Beside> {
  /// Every pane it has shown, so one closing keeps its place in the
  /// children (and its state) as it goes; a gone one is nothing. The
  /// rail's name is in it once it was wanted.
  final _seen = <String>{};

  /// What showed in the last build.
  var _shown = <String>{};

  /// Whether it was built before: what it shows at first doesn't slide.
  bool _built = false;

  /// Each pane's rect, and the rail's, as last laid out: where one going
  /// stays, laid out as it was.
  final _at = <String, Rect>{};

  /// Each pane's width as last laid out beside [Beside.main]: what one
  /// over all ([Beside.cover]) leaves the rest.
  final _widths = <String, double>{};

  /// Each pane's [Glide] token, changed only while it stays shown: a pane
  /// coming in slides, it doesn't glide.
  final _tokens = <String, Object>{};

  /// The widths of the panes being dragged, as their edges say: only open
  /// ones not over all. A pane closed, or covering it all, mid-drag never
  /// says its drag ended (its edge is gone), so what it said last would
  /// otherwise stay and decide for it when it's back at its saved width.
  final _live = <String, double>{};

  /// Whether [name] is a pane whose drag decides: open, and not over all.
  bool _drags(String name) =>
      name != widget.cover && widget.panes.any((p) => p.name == name);

  /// The room as last laid out, and whether the rail showed in it.
  double _room = 0;
  bool _rail = false;

  /// Holds the focus while it's in the rail.
  final _railFocus = FocusNode(skipTraversal: true, canRequestFocus: false);

  @override
  void didUpdateWidget(Beside old) {
    super.didUpdateWidget(old);
    if (widget.scene == old.scene) return;
    // Another scene: what it shows is there at once, as at the first build.
    _seen.clear();
    _shown = {};
    _tokens.clear();
    _at.clear();
    _widths.clear();
    _live.clear();
    _built = false;
  }

  @override
  void dispose() {
    _railFocus.dispose();
    super.dispose();
  }

  BesideFit _fit(double room) => besideFit(
    room: room,
    panes: widget.panes,
    saved: paneWidths.value,
    live: _live,
    wantRail: widget.details != null,
    least: widget.least,
    railLeast: widget.railLeast,
    rail: widget.details?.width ?? 0,
  );

  /// A pane's edge is dragged to [width] (null: let go): the rail shows or
  /// tucks as the room it leaves says, and nothing else is built again.
  void _dragged(String name, double? width) {
    if (width == null || !_drags(name)) {
      _live.remove(name);
    } else {
      _live[name] = width;
    }
    if (mounted && _fit(_room).rail != _rail) setState(() {});
  }

  @override
  Widget build(BuildContext context) => LayoutBuilder(
    builder: (context, box) => ValueListenableBuilder(
      valueListenable: paneWidths,
      builder: (context, _, _) {
        final room = _room = box.maxWidth;
        _live.removeWhere((name, _) => !_drags(name));
        final fit = _fit(room);
        final scene = widget.scene;
        final details = widget.details;
        final panes = widget.panes;
        final over = fit.over;
        final rail = fit.rail;
        // The rail going with the focus in it: the keys stay with [keep].
        if (_rail && !rail && _railFocus.hasFocus) {
          WidgetsBinding.instance.addPostFrameCallback((_) {
            if (mounted) widget.keep?.requestFocus();
          });
        }
        _rail = rail;
        final t = NushiTheme.of(context).t;
        final now = {for (final p in panes) p.name, if (rail) ?details?.name};
        // What glides: what's open beside [main], whether the panes lie
        // over it, and the sidebar, which moves everything sideways.
        final token = [...now, if (over) 'over', sidebarShut].join(',');
        for (final slot in now) {
          if (_shown.contains(slot)) _tokens[slot] = token;
          _tokens.putIfAbsent(slot, () => token);
        }
        final built = _built;
        _built = true;
        _seen.addAll(now);
        if (details != null) _seen.add(details.name);
        _shown = now;
        final cover = panes.any((p) => p.name == widget.cover)
            ? widget.cover
            : null;
        // Under the pane over all: kept, but not drawn or reached.
        Widget under(Object slot, Widget child) => ExcludeFocus(
          excluding: cover != null && slot != cover,
          child: Offstage(
            offstage: cover != null && slot != cover,
            child: child,
          ),
        );
        Widget slide(String slot, Widget? child, {bool shown = true}) => under(
          slot,
          FocusTraversalGroup(
            child: Glide(
              token: _tokens[slot],
              child: Presence(
                // What showed as the view came is there; what opens
                // after slides in.
                arrives: built,
                shown: shown,
                child: child,
                builder: (_, run, c) => paneSlide(run, c),
              ),
            ),
          ),
        );
        Widget pane(String slot, Widget? child) => LayoutId(
          key: ValueKey((scene, slot)),
          id: slot,
          child: slide(slot, child),
        );
        Widget backed(Widget child, {required bool shadow}) => DecoratedBox(
          decoration: BoxDecoration(
            color: t.base,
            boxShadow: [
              if (shadow)
                BoxShadow(
                  color: t.shadow.withValues(
                    alpha: t.brightness == Brightness.dark ? 0.4 : 0.12,
                  ),
                  blurRadius: 12,
                ),
            ],
          ),
          child: child,
        );
        final railName = details?.name ?? _BesideState._railName;
        final children = [
          // First in the tree, as they were above and below the rest: they
          // never lie over what's after them.
          if (widget.head case final head?)
            LayoutId(
              key: const ValueKey(_Slot.head),
              id: _Slot.head,
              child: under(
                _Slot.head,
                FocusTraversalGroup(
                  child: _BesideScope(
                    glide: token,
                    scene: scene,
                    held: details != null && !rail,
                    child: head,
                  ),
                ),
              ),
            ),
          if (widget.foot case final foot?)
            LayoutId(
              key: const ValueKey(_Slot.foot),
              id: _Slot.foot,
              child: under(_Slot.foot, FocusTraversalGroup(child: foot)),
            ),
          LayoutId(
            key: ValueKey((scene, _Slot.main)),
            id: _Slot.main,
            child: under(
              _Slot.main,
              FocusTraversalGroup(
                child: Glide(
                  token: token,
                  sides: widget.columnSides,
                  // Its foot is its full width: cut at its left as it
                  // glides, it doesn't lie over the sidebar.
                  clip: true,
                  child: widget.main,
                ),
              ),
            ),
          ),
          // The rail: kept while it's tucked away for room, gone only once
          // it's not wanted. Under the panes, which it slides out under.
          if (_seen.contains(railName))
            LayoutId(
              key: ValueKey((scene, railName)),
              id: railName,
              child: Focus(
                focusNode: _railFocus,
                child: slide(
                  railName,
                  shown: rail,
                  details == null
                      ? null
                      : SizedBox(
                          key: ValueKey(details.name),
                          width: details.width,
                          // Over the rail, whose own fill would hide it.
                          child: DecoratedBox(
                            position: DecorationPosition.foreground,
                            decoration: BoxDecoration(
                              border: Border(left: BorderSide(color: t.line)),
                            ),
                            child: details.child,
                          ),
                        ),
                ),
              ),
            ),
          // Those closing: over the main view and the rail, under the rest.
          for (final s in _seen)
            if (!now.contains(s) && s != railName) pane(s, null),
          // Beside, a pane leaves [main] its least width; over it, a
          // strip of it. Over it, the first one's shadow falls on it.
          for (final (i, p) in panes.indexed)
            pane(
              p.name,
              backed(
                Resizable(
                  key: ValueKey(p.name),
                  name: p.name,
                  initial: p.initial,
                  min: p.min,
                  max: fit.most[p.name]!,
                  left: true,
                  locked: p.name == cover,
                  onDrag: (w) => _dragged(p.name, w),
                  child: p.child,
                ),
                shadow: over && i == 0,
              ),
            ),
        ];
        // The pane over all is drawn last; keyed, it keeps its state as it
        // moves in the list.
        if (cover != null) {
          final top = children.indexWhere(
            (c) => c.key == ValueKey((scene, cover)),
          );
          children.add(children.removeAt(top));
        }
        return CustomMultiChildLayout(
          delegate: _Beside(
            panes: [for (final p in panes) p.name],
            rail: _seen.contains(railName) ? railName : null,
            railShown: rail,
            railWidth: details?.width ?? 0,
            over: over,
            going: [
              for (final s in _seen)
                if (!now.contains(s) && s != railName) s,
            ],
            at: _at,
            widths: _widths,
            cover: cover,
          ),
          children: children,
        );
      },
    ),
  );

  /// The rail's slot while no rail is wanted: its last name.
  static const _railName = 'details';
}
