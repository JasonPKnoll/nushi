import 'dart:convert';

import 'package:connectrpc/connect.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nushi/chat.dart';
import 'package:nushi/daemon.dart';
import 'package:nushi/gen/nushi/v1/nushi.pb.dart';
import 'package:nushi/line_changes.dart';
import 'package:nushi/preview.dart';

import 'fake_daemon.dart';
import 'file_edit_helpers.dart' show sumOf;
import 'harness.dart';

// A file preview's change marks (ADR 0131): which lines differ from the
// last commit, and the last commit's text, asked of the daemon.

const _base = 'one\ntwo\nthree\nfour\nfive\n';

/// The GetDiff questions with a path [fake] was asked.
List<GetDiffRequest> _asked(FakeDaemon fake) =>
    fake.calls.whereType<GetDiffRequest>().toList();

void main() {
  group('lineMarks', () {
    test('equal texts have none', () {
      expect(lineMarks(_base, _base), isEmpty);
    });

    test('added, changed and removed lines', () {
      const shown = 'one\nTWO\nthree\nnew a\nnew b\nfour\n';
      expect(lineMarks(_base, shown), [
        const LineMark(MarkKind.changed, 1, 1, ['two']),
        const LineMark(MarkKind.added, 3, 2),
        const LineMark(MarkKind.removed, 6, 0, ['five']),
      ]);
    });

    test('lines removed at the top', () {
      expect(lineMarks(_base, 'three\nfour\nfive\n'), [
        const LineMark(MarkKind.removed, 0, 0, ['one', 'two']),
      ]);
    });

    test('line endings and a BOM aside, the same text has none', () {
      expect(lineMarks(_base.replaceAll('\n', '\r\n'), _base), isEmpty);
      expect(lineMarks('﻿$_base', _base), isEmpty);
      expect(lineMarks(_base, '﻿$_base'), isEmpty);
    });

    test("a CRLF base's old lines have no CR", () {
      expect(lineMarks('a\r\nb\r\n', 'a\nB\n'), [
        const LineMark(MarkKind.changed, 1, 1, ['b']),
      ]);
    });

    test('past 10,000 lines on a side, none', () {
      final long = List.filled(10001, 'x').join('\n');
      expect(lineMarks(long, '$long\ny'), isEmpty);
      expect(lineMarks('y', long), isEmpty);
    });

    test('a long text is diffed off the UI thread, alike', () async {
      final base = [for (var i = 0; i < 3000; i++) 'line $i'].join('\n');
      final shown = base.replaceFirst('line 1500', 'line X');
      final soon = lineMarksSoon(base, shown);
      expect(soon, isA<Future<List<LineMark>>>());
      expect(await soon, lineMarks(base, shown));
      expect(lineMarksSoon(_base, _base), isA<List<LineMark>>());
    });
  });

  test('markCells puts each mark on its lines', () {
    final cells = markCells(const [
      LineMark(MarkKind.changed, 1, 2, ['b']),
      LineMark(MarkKind.removed, 0, 0, ['z']),
      LineMark(MarkKind.removed, 4, 0, ['end']),
    ], 4);
    expect(cells[0]!.top!.old, ['z']);
    expect(cells[0]!.bar, isNull);
    expect(cells[1]!.bar!.kind, MarkKind.changed);
    expect(cells[2]!.bar!.kind, MarkKind.changed);
    // Removed at the end: the last line's bottom edge.
    expect(cells[3]!.bottom!.old, ['end']);
    expect(markCells(const [], 0), isEmpty);
  });

  group('LineBase', () {
    late FakeDaemon fake;
    late Daemon d;
    late ChatState c;
    late FileWatch watch;
    late LineBase lines;

    /// [path] on the fake's disk as [text], its change [change] (a
    /// FileChange or an exception; none: unchanged), watched and asked.
    Future<LineBase> start(
      WidgetTester tester, {
      String path = 'src/a.go',
      String text = _base,
      Object? change,
      int api = oneFileDiffApi,
      Object? Function(Object input)? answer,
    }) async {
      fake = FakeDaemon([chat('c', 'claude', 'Fix')])..answer = answer;
      fake.files[path] = utf8.encode(text);
      if (change != null) fake.fileDiffs['/Users/me/nushi/$path'] = change;
      d = (await fake.connect(tester))..daemonApi = api;
      c = d.chats['c']!;
      watch = FileWatch(d, 'c', path);
      lines = LineBase(d, c, watch);
      await tester.pumpAndSettle();
      return lines;
    }

    Future<void> end(WidgetTester tester) async {
      lines.dispose();
      watch.dispose();
      await closeView(tester, d);
    }

    testWidgets("asks of the file's own folder, its name as the path", (
      tester,
    ) async {
      await start(tester);
      expect(_asked(fake).single.folder, '/Users/me/nushi/src');
      expect(_asked(fake).single.path, 'a.go');
      // No change: its text now is the last commit's.
      expect(lines.value, _base);
      expect(lines.isNew, isFalse);
      await end(tester);
    });

    testWidgets("an absolute path's folder, as the daemon names it", (
      tester,
    ) async {
      const path = '/elsewhere/lib/b.dart';
      await start(tester, path: path);
      final data = utf8.encode('b\n');
      fake.sendWatch(
        path,
        WatchChatFileResponse(path: path, data: data, sha256: sumOf(data)),
      );
      await tester.pumpAndSettle();
      expect(_asked(fake).last.folder, '/elsewhere/lib');
      expect(_asked(fake).last.path, 'b.dart');
      await end(tester);
    });

    testWidgets("a modified file's base is the last commit's text", (
      tester,
    ) async {
      await start(
        tester,
        path: 'a.go',
        text: 'new\n',
        change: FileChange(path: 'a.go', status: 'modified', oldText: 'old\n'),
      );
      expect(lines.value, 'old\n');
      await end(tester);
    });

    for (final (what, change, isNew) in [
      ('untracked', FileChange(status: 'untracked'), true),
      ('added', FileChange(status: 'added'), true),
      (
        'ignored',
        FileChange(status: 'untracked', skipped: 'not tracked'),
        false,
      ),
      ('a link', FileChange(status: 'modified', skipped: 'a link'), false),
      ('binary', FileChange(status: 'modified', skipped: 'binary'), false),
      ('deleted', FileChange(status: 'deleted'), false),
    ]) {
      testWidgets('$what: no base, new $isNew', (tester) async {
        await start(tester, path: 'a.go', change: change..path = 'a.go');
        expect(lines.value, isNull);
        expect(lines.isNew, isNew);
        await end(tester);
      });
    }

    testWidgets('not in a repository: no base', (tester) async {
      await start(
        tester,
        change: ConnectException(
          Code.failedPrecondition,
          'not a git repository',
        ),
      );
      expect(lines.value, isNull);
      expect(lines.isNew, isFalse);
      await end(tester);
    });

    testWidgets('asks again as the file changes, a turn ends and the chat '
        'commits', (tester) async {
      await start(tester, path: 'a.go');
      expect(_asked(fake), hasLength(1));
      fake.putFile('a.go', utf8.encode('changed\n'));
      await tester.pumpAndSettle();
      expect(_asked(fake), hasLength(2));
      // An empty answer: the text when it was asked.
      expect(lines.value, 'changed\n');
      // The same file again: nothing to ask.
      fake.putFile('a.go', utf8.encode('changed\n'));
      await tester.pumpAndSettle();
      expect(_asked(fake), hasLength(2));

      // A turn's state as the daemon's events set it.
      c
        ..busy = true
        ..notifyListeners();
      await tester.pump();
      expect(_asked(fake), hasLength(2));
      c
        ..busy = false
        ..notifyListeners();
      await tester.pumpAndSettle();
      expect(_asked(fake), hasLength(3));

      c.committed();
      await tester.pumpAndSettle();
      expect(_asked(fake), hasLength(4));
      await end(tester);
    });

    testWidgets('only the latest answer lands', (tester) async {
      var n = 0;
      await start(
        tester,
        path: 'a.go',
        text: 'v1\n',
        answer: (input) {
          if (input is! GetDiffRequest) return null;
          // The first answer comes last.
          if (++n == 1) {
            return Future<Object?>.delayed(
              const Duration(seconds: 1),
              () => GetDiffResponse(
                files: [
                  FileChange(
                    path: 'a.go',
                    status: 'modified',
                    oldText: 'old\n',
                  ),
                ],
              ),
            );
          }
          return GetDiffResponse();
        },
      );
      fake.putFile('a.go', utf8.encode('v2\n'));
      await tester.pump();
      await tester.pump(const Duration(seconds: 2));
      expect(lines.value, 'v2\n');
      await end(tester);
    });

    testWidgets('an older daemon is never asked', (tester) async {
      await start(tester, api: oneFileDiffApi - 1);
      expect(_asked(fake), isEmpty);
      expect(lines.value, isNull);
      await end(tester);
    });
  });
}
