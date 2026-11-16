import 'package:nushi/login_item.dart';

/// Windows starts no daemon at login yet (ADR 0125; a Run key or a
/// Startup folder shortcut later): no call has an answer, so Settings
/// doesn't show Start at login.
class WindowsLoginItem implements LoginItem {
  const new();

  @override
  Future<LoginItemState?> status() async => null;

  @override
  Future<LoginItemState?> set({required bool on}) async => null;

  @override
  Future<LoginItemState?> openSettings() async => null;
}
