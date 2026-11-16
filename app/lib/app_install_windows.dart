import 'dart:convert';
import 'dart:io';

import 'package:nushi/app_install.dart';
import 'package:nushi/app_log.dart';

/// Windows's install (ADR 0125): `nushi.exe` with the daemon,
/// `nushid.exe`, beside it, the version Flutter writes into
/// `data\flutter_assets\version.json`, and the data folder nushid picks
/// itself (ADR 0124). The daemon starts on its own, with the app's
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

  /// Started detached with its standard streams, so what it says on
  /// stderr goes to the app's log: a daemon whose data folder isn't the
  /// user's alone says there why it won't run, before it has a log of its
  /// own (ADR 0124). One that starts says nothing: it starts itself again
  /// with no window, its stderr `nushid.log`, and exits.
  @override
  Future<void> launch(String dataDir) async {
    final program = daemon;
    if (program == null) return;
    final Process started;
    try {
      started = await Process.start(
        program,
        const [],
        environment: launchEnvironment(_env, dataDir),
        includeParentEnvironment: false,
        mode: ProcessStartMode.detachedWithStdio,
      );
    } on ProcessException catch (e) {
      appLog.warn("the bundled daemon didn't start", {'error': e.message});
      return;
    }
    started.stdin.close().ignore();
    started.stdout.drain<void>().ignore();
    final said = await daemonSaid(started.stderr);
    if (said.isNotEmpty) {
      appLog.warn('the bundled daemon said, starting', {'stderr': said});
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

/// The first of what [stderr] says, trimmed: at most about 4 KB, after
/// which it's no longer read, so a daemon that goes on writing there
/// fills nothing.
Future<String> daemonSaid(Stream<List<int>> stderr) async {
  final said = StringBuffer();
  await for (final text in const Utf8Decoder(
    allowMalformed: true,
  ).bind(stderr)) {
    said.write(text);
    if (said.length >= 4096) break;
  }
  return said.toString().trim();
}
