import 'package:nushi/window_drag.dart';

/// Windows's window, which keeps the system's title bar (ADR 0124), as
/// Linux's does: it moves and zooms the window, so a header does neither,
/// and there's no menu bar for the app's menus. Nothing presents it yet:
/// no notification's click asks.
class WindowsWindow implements AppWindow {
  const new();

  /// The title bar is above the content, so nothing is in the way.
  @override
  double get titleInset => 8;

  @override
  bool get headersMoveWindow => false;

  @override
  bool get menuBar => false;

  @override
  Future<void> present({String? token}) async {}

  @override
  Future<void> arm(int micros) async {}

  @override
  Future<void> zoom() async {}
}
