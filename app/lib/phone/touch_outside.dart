import 'package:flutter/gestures.dart';
import 'package:flutter/widgets.dart';

/// What a touch outside a focused text field does on the phone: a tap
/// (the finger lifted where it went down) lets go of the field, and the
/// keyboard closes; a drag (a scroll of the transcript) keeps it, so the
/// chat scrolls with the keyboard up. A mouse's or a pen's press lets go
/// at once, as the framework does.
///
/// The framework's own rule, in a mobile browser, lets go on a touch's
/// press (EditableText's `_EditableTextTapOutsideAction`, under `kIsWeb`),
/// before it can tell a scroll from a tap; this overrides its intents for
/// every field below it.
class TouchOutside extends StatefulWidget {
  const new({required this.child, super.key});

  final Widget child;

  @override
  State<TouchOutside> createState() => _TouchOutsideState();
}

class _TouchOutsideState extends State<TouchOutside> {
  /// The touch that went down outside the focused field, and where.
  ({int pointer, Offset at})? _down;

  void _pressed(EditableTextTapOutsideIntent i) {
    final e = i.pointerDownEvent;
    if (e.kind != PointerDeviceKind.touch) {
      _down = null;
      i.focusNode.unfocus();
      return;
    }
    _down = (pointer: e.pointer, at: e.position);
  }

  void _lifted(EditableTextTapUpOutsideIntent i) {
    final e = i.pointerUpEvent;
    final down = _down;
    _down = null;
    if (down == null || down.pointer != e.pointer) return;
    if ((e.position - down.at).distance <= kTouchSlop) i.focusNode.unfocus();
  }

  @override
  Widget build(BuildContext context) => Actions(
    actions: {
      EditableTextTapOutsideIntent:
          CallbackAction<EditableTextTapOutsideIntent>(onInvoke: _pressed),
      EditableTextTapUpOutsideIntent:
          CallbackAction<EditableTextTapUpOutsideIntent>(onInvoke: _lifted),
    },
    child: widget.child,
  );
}
