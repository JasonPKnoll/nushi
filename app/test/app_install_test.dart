import 'package:flutter_test/flutter_test.dart';
import 'package:nushi/app_install.dart';

// The environment the app gives what it starts: its own, less Nushi's
// variables; the daemon's also names its data folder.

void main() {
  const env = {
    'PATH': '/usr/bin',
    'NUSHI_DATA': '/tmp/nd1',
    'NUSHID_DATA': '/elsewhere',
    'NUSHID_TOKEN': 'x',
    'NUSHIX': 'kept',
  };

  test('a program gets none of Nushi’s variables', () {
    expect(withoutNushi(env), {'PATH': '/usr/bin', 'NUSHIX': 'kept'});
  });

  test('the daemon gets its data folder, and NUSHI_DATA when the app has '
      'it', () {
    expect(launchEnvironment(env, '/data'), {
      'PATH': '/usr/bin',
      'NUSHIX': 'kept',
      'NUSHID_DATA': '/data',
      'NUSHI_DATA': '/tmp/nd1',
    });
    expect(launchEnvironment({'PATH': '/usr/bin'}, '/data'), {
      'PATH': '/usr/bin',
      'NUSHID_DATA': '/data',
    });
  });
}
