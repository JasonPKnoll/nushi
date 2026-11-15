import 'dart:async';
import 'dart:ui' show Offset;

import 'package:flutter/gestures.dart' show kDoubleTapSlop, kDoubleTapTimeout;

/// Tells a second press on the same thing, soon after the first and near
/// it, as a double click, beside a tap that runs on each press as it did:
/// nothing waits to see whether a second comes (the Files pane's preview
/// tabs, ADR 0070). Timed by a timer, as Flutter's double tap is, not by
/// the clock or the events' times, so a test's pumps move it.
final class DoubleClicks {
  Object? _first;
  Offset _at = Offset.zero;
  Timer? _wait;

  /// Whether a press on [what] at [where] is a double click's second.
  bool down(Object what, Offset where) {
    final again =
        _wait != null &&
        what == _first &&
        (where - _at).distance <= kDoubleTapSlop;
    _wait?.cancel();
    if (again) {
      _wait = _first = null;
      return true;
    }
    _first = what;
    _at = where;
    _wait = Timer(kDoubleTapTimeout, () => _wait = _first = null);
    return false;
  }

  void dispose() => _wait?.cancel();
}
