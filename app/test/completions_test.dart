import 'package:flutter_test/flutter_test.dart';
import 'package:nushi/completions.dart';

void main() {
  test('names whose last part starts with what is typed first, then the '
      'shortest, then as listed; 8 at most', () {
    expect(
      rankChoices([
        'lib/chat_view.dart',
        'lib/chat.dart',
        'test/view_chat_test.dart',
        'docs/chat:notes.md',
        'lib/flat.dart',
        'lib/a/chat.go',
        'lib/b/chat.go',
      ], 'chat'),
      [
        'lib/chat.dart',
        'lib/a/chat.go',
        'lib/b/chat.go',
        'lib/chat_view.dart',
        'docs/chat:notes.md', // its last part is notes.md
        'test/view_chat_test.dart',
      ],
    );
    // Case aside; as long as each other: as listed.
    expect(rankChoices(['b/readme.md', 'A/README.md', 'c/x.md'], 'READ'), [
      'b/readme.md',
      'A/README.md',
    ]);
    expect(
      rankChoices([for (var i = 0; i < 20; i++) 'f$i'], 'f'),
      hasLength(8),
    );
  });

  // A bare "@" ranks every file the folder lists, at every key, so each
  // name is read and scored once: that is what's checked. The time is a
  // measure, printed (11–15 ms on a Mac), as long_chat_open_test's is: a
  // bound on it failed on a busy runner.
  test('20,000 paths are ranked in one pass', () {
    var read = 0;
    final paths = Iterable.generate(20000, (i) {
      read++;
      return 'packages/module_${i % 97}/lib/src/part_$i/file_$i.dart';
    });
    final watch = Stopwatch()..start();
    final got = rankChoices(paths, 'file');
    watch.stop();
    expect(got, hasLength(8));
    expect(got.first, 'packages/module_0/lib/src/part_0/file_0.dart');
    expect(read, 20000);
    // The measure, for whoever runs it.
    // ignore: avoid_print
    print('20,000 paths ranked in ${watch.elapsedMilliseconds} ms');
  });
}
