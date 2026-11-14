import 'package:nushi/alerts.dart';

/// Windows posts no notifications yet (ADR 0124; toasts need the app
/// registered with an AppUserModelID, which its package will do): every
/// call does nothing, and whether the system lets the app notify has no
/// answer, so Settings doesn't offer it.
class WindowsAlertCenter implements AlertCenter {
  const new();

  @override
  Future<void> post({
    required String id,
    required String chat,
    required String title,
    required String body,
    required bool sound,
  }) async {}

  @override
  Future<void> withdraw(List<String> ids) async {}

  @override
  Future<void> clear(String chat) async {}

  @override
  Future<void> badge(String label) async {}

  @override
  Future<NotifyPermission?> allowed() async => null;

  @override
  Future<void> openSettings() async {}

  @override
  void onOpen(void Function(String chat)? open) {}
}
