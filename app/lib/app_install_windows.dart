import 'dart:convert';
import 'dart:io';

import 'package:nushi/app_install.dart';
import 'package:nushi/app_log.dart';

/// Windows's install (ADR 0124): `nushi.exe` with the daemon,
/// `nushid.exe`, beside it, the version Flutter writes into
/// `data\flutter_assets\version.json`, and the data folder nushid picks
/// itself (ADR 0123). The daemon starts on its own, with the app's
/// environment and no login shell (Windows has none to start it
/// through), and logs to its folder itself.
class WindowsAppInstall implements AppInstall {
  /// A test passes the [executable] the app runs as and its [environment].
  new({String? executable, Map<String, String>? environment})
    : _bundle = File(executable ?? Platform.resolvedExecutable).parent.path,
      _env = environment ?? Platform.environment;

  final String _bundle;
  final Map<String, String> _env;

  /// `%LOCALAPPDATA%\Nushi`, else under the home's `AppData\Local`:
  /// nushid's own rule.
  @override
  String get dataFolder => switch (_env['LOCALAPPDATA']) {
    final local? when local.contains(':') => '$local\\Nushi',
    _ => '${_env['USERPROFILE']}\\AppData\\Local\\Nushi',
  };

  @override
  late final String? daemon = () {
    final path = '$_bundle\\nushid.exe';
    return File(path).existsSync() ? path : null;
  }();

  @override
  late final String? daemonBuild = () {
    if (daemon case final path?) {
      final st = File(path).statSync();
      return '${st.size}-${st.modified.millisecondsSinceEpoch}';
    }
    return null;
  }();

  /// The build's `version` and `build_number`, as Linux's.
  @override
  late final String version = () {
    try {
      final json = jsonDecode(
        File('$_bundle\\data\\flutter_assets\\version.json').readAsStringSync(),
      ) as Map<String, Object?>;
      return '${json['version'] ?? ''}+${json['build_number'] ?? ''}';
    } on Object {
      return '';
    }
  }();

  @override
  String get stopByHand => 'end nushid.exe in Task Manager';

  /// No: see [stop].
  @override
  bool get canStop => false;

  @override
  Future<void> launch(String dataDir) async {
    final program = daemon;
    if (program == null) return;
    try {
      await Process.start(
        program,
        const [],
        environment: launchEnvironment(_env, dataDir),
        includeParentEnvironment: false,
        mode: ProcessStartMode.detached,
      );
    } on ProcessException catch (e) {
      appLog.warn("the bundled daemon didn't start", {'error': e.message});
    }
  }

  /// Nothing yet: Windows doesn't name a Unix socket's other end, so which
  /// nushid listens in [dataDir] isn't known, and none is stopped
  /// (docs/roadmap.md).
  @override
  Future<void> stop(String dataDir) async {
    appLog.warn("the daemon isn't stopped on Windows yet");
  }
}
