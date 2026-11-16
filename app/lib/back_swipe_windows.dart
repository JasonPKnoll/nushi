import 'package:flutter/widgets.dart';

import 'package:nushi/back_swipe.dart';

/// Windows has no swipe of its own beyond what Flutter sees: a precision
/// touchpad's pans and the mouse's back button reach the engine, which
/// [BackSwipe] already takes.
class WindowsPageSwipes implements PageSwipes {
  const new();

  @override
  void onBack(void Function(Offset at) back) {}
}
