import 'dart:convert';
import 'dart:ui' show PictureRecorder;

import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nushi/change_marks.dart';
import 'package:nushi/daemon.dart';
import 'package:nushi/gen/nushi/v1/nushi.pb.dart';
import 'package:nushi/line_changes.dart';
import 'package:nushi/motion.dart';
import 'package:nushi/source_editor.dart';
import 'package:nushi/source_view.dart';
import 'package:nushi/theme/theme.dart';
import 'package:re_editor/re_editor.dart';

import 'fake_daemon.dart' show FakeDaemon, loadFonts;
import 'file_edit_helpers.dart';
import 'harness.dart';

// A file's source with change marks against the last commit (ADR 0131):
// a bar beside added and changed lines, a wedge where lines went, and the
// card of the old lines on hover or a click; in the read-only source and
// the editor.

/// The last commit's text.
const changeBase =
    'package main\n'
    '\n'
    'import "fmt"\n'
    '\n'
    '// greet says hello.\n'
    'func greet() {\n'
    '\tfmt.Println("hi")\n'
    '}\n'
    '\n'
    'func old() {}\n'
    '\n'
    'func main() {\n'
    '\tgreet()\n'
    '}\n';

/// The text now: lines 4–6 changed, func old's two lines removed, main's
/// call changed and a function added at the end.
const changeShown =
    'package main\n'
    '\n'
    'import "fmt"\n'
    '\n'
    '// greet says hello to name.\n'
    'func greet(name string) {\n'
    '\tfmt.Println("hi", name)\n'
    '}\n'
    '\n'
    'func main() {\n'
    '\tgreet("you")\n'
    '}\n'
    '\n'
    'func extra() {}\n';

const _pad = EdgeInsets.fromLTRB(28, 22, 28, 28);

Widget _source({
  String? base = changeBase,
  String text = changeShown,
  String path = 'main.go',
  bool wrap = true,
  Tokens tokens = Tokens.dark,
}) => themed(
  tokens: tokens,
  SizedBox(
    width: 560,
    height: 380,
    child: SourceView(text, path: path, base: base, wrap: wrap, padding: _pad),
  ),
);

/// The marks' cells, by the line each is on, top to bottom.
List<MarkCell> _cells(WidgetTester tester) {
  final marks = tester.widgetList<ChangeMark>(find.byType(ChangeMark)).toList()
    ..sort(
      (a, b) => tester
          .getTopLeft(find.byWidget(a))
          .dy
          .compareTo(tester.getTopLeft(find.byWidget(b)).dy),
    );
  return [for (final m in marks) m.cell];
}

Finder _line(String text) => find.textContaining(text, findRichText: true);

Future<TestGesture> _mouse(WidgetTester tester) async {
  final mouse = await tester.createGesture(kind: PointerDeviceKind.mouse);
  await mouse.addPointer(location: Offset.zero);
  addTearDown(mouse.removePointer);
  return mouse;
}

void main() {
  setUpAll(loadFonts);

  for (final wrap in [true, false]) {
    testWidgets('the source marks its changed lines, wrapped $wrap', (
      tester,
    ) async {
      await tester.pumpWidget(_source(wrap: wrap));
      final cells = _cells(tester);
      // Lines 4–6 changed (three rows), the removal's wedge on main's
      // line, main's call changed, two lines added.
      expect(cells.map((c) => c.bar?.kind), [
        MarkKind.changed,
        MarkKind.changed,
        MarkKind.changed,
        null,
        MarkKind.changed,
        MarkKind.added,
        MarkKind.added,
      ]);
      expect(cells[3].top!.old, ['func old() {}', '']);
      // Each mark is on its line.
      final greet = tester.getTopLeft(_line('func greet(name')).dy;
      final mark = tester.getTopLeft(find.byType(ChangeMark).at(1)).dy;
      expect(mark, closeTo(greet, 1));
    });
  }

  testWidgets('the code starts where it did, marks or not', (tester) async {
    for (final wrap in [true, false]) {
      await tester.pumpWidget(_source(base: null, wrap: wrap));
      final plain = tester.getTopLeft(_line('func greet(name')).dx;
      await tester.pumpWidget(_source(wrap: wrap));
      expect(tester.getTopLeft(_line('func greet(name')).dx, plain);
      expect(find.byType(ChangeMark), findsWidgets);
    }
  });

  testWidgets('none without a base, for the same text, or for JSON it '
      'pretty-printed', (tester) async {
    await tester.pumpWidget(_source(base: null));
    expect(find.byType(ChangeMark), findsNothing);
    await tester.pumpWidget(_source(base: changeShown));
    expect(find.byType(ChangeMark), findsNothing);
    await tester.pumpWidget(
      _source(path: 'a.json', base: '{"a":1}', text: '{"a":2}'),
    );
    expect(find.byType(ChangeMark), findsNothing);
    // JSON already formatted shows as it is, and is marked.
    await tester.pumpWidget(
      _source(
        path: 'a.json',
        base: '{\n  "a": 1\n}\n',
        text: '{\n  "a": 2\n}\n',
      ),
    );
    expect(find.byType(ChangeMark), findsOneWidget);
  });

  testWidgets('a base that comes later marks the lines', (tester) async {
    await tester.pumpWidget(_source(base: null));
    await tester.pumpWidget(_source());
    expect(find.byType(ChangeMark), findsNWidgets(7));
  });

  testWidgets('the pointer on a mark shows the old lines after a moment', (
    tester,
  ) async {
    await tester.pumpWidget(_source());
    final mouse = await _mouse(tester);
    await mouse.moveTo(tester.getCenter(find.byType(ChangeMark).first));
    await tester.pump(const Duration(milliseconds: 100));
    expect(find.byType(ChangeCard), findsNothing);
    await tester.pump(const Duration(milliseconds: 400));
    expect(find.text('Before'), findsOneWidget);
    expect(find.textContaining('// greet says hello.'), findsOneWidget);
    await mouse.moveTo(Offset.zero);
    await tester.pumpAndSettle(const Duration(milliseconds: 200));
    expect(find.byType(ChangeCard), findsNothing);
  });

  testWidgets('a click shows them at once, and keeps them', (tester) async {
    await tester.pumpWidget(_source());
    // The removal's wedge, on main's line.
    await tester.tap(find.byType(ChangeMark).at(3));
    await tester.pump();
    expect(find.text('2 lines removed'), findsOneWidget);
    expect(find.textContaining('func old() {}'), findsOneWidget);
    // The pointer leaving doesn't hide it; a click elsewhere does.
    await tester.pump(const Duration(seconds: 1));
    expect(find.byType(ChangeCard), findsOneWidget);
    await tester.tapAt(const Offset(400, 300));
    await tester.pump();
    expect(find.byType(ChangeCard), findsNothing);
    // An added run's card says so.
    await tester.tap(find.byType(ChangeMark).last);
    await tester.pump();
    expect(find.text('New lines'), findsOneWidget);
    expect(find.text('Not in the last commit'), findsOneWidget);
  });

  testWidgets("near the window's foot, the card opens over its line", (
    tester,
  ) async {
    const cell = MarkCell(bar: LineMark(MarkKind.changed, 0, 1, ['was']));
    Widget at(Alignment where) => themed(
      Align(
        alignment: where,
        child: const SizedBox(
          width: 30,
          height: 17,
          child: ChangeMark(cell: cell, barX: 26),
        ),
      ),
    );
    for (final (where, under) in [
      (Alignment.topLeft, true),
      (Alignment.bottomLeft, false),
    ]) {
      await tester.pumpWidget(at(where));
      await tester.tap(find.byType(ChangeMark));
      await tester.pump();
      final mark = tester.getRect(find.byType(ChangeMark));
      final card = tester.getRect(find.byType(ChangeCard));
      if (under) {
        expect(card.top, greaterThanOrEqualTo(mark.bottom));
      } else {
        expect(card.bottom, lessThanOrEqualTo(mark.top));
      }
      await tester.tapAt(const Offset(400, 300));
      await tester.pump();
    }
  });

  testWidgets("a removal's wedge is whole in a column that ends at the bar", (
    tester,
  ) async {
    // The editor's mark column ends at the bar's right edge and clips
    // there: the wedge must fit, 7 wide, not a 3-point sliver.
    const box = Rect.fromLTWH(0, 0, markGap + markBar, 20);
    const cell = MarkCell(top: LineMark(MarkKind.removed, 1, 0, ['gone']));
    final recorder = PictureRecorder();
    final canvas = Canvas(recorder)..clipRect(box);
    paintChangeMark(canvas, box.translate(0, 5), cell, markGap, Tokens.light);
    final image = await tester.runAsync(
      () => recorder.endRecording().toImage(7, 30),
    );
    final bytes = (await tester.runAsync(image!.toByteData))!;
    // Inked across the box (but its anti-aliased tip) on the edge's row.
    for (var x = 0; x < 6; x++) {
      expect(
        bytes.getUint8((5 * 7 + x) * 4 + 3),
        greaterThan(200),
        reason: 'column $x',
      );
    }
    image.dispose();
  });

  testWidgets('a card lists 40 old lines at most', (tester) async {
    final old = [for (var i = 1; i <= 50; i++) 'old line $i'];
    await tester.pumpWidget(
      themed(
        Center(
          child: ChangeCard(marks: [LineMark(MarkKind.changed, 0, 1, old)]),
        ),
      ),
    );
    expect(find.textContaining('old line 40'), findsOneWidget);
    expect(find.textContaining('old line 41'), findsNothing);
    expect(find.text('… 10 more lines'), findsOneWidget);
    expect(tester.getSize(find.byType(ChangeCard)).height, lessThan(270));
  });

  for (final (name, tokens) in [
    ('light', Tokens.light),
    ('dark', Tokens.dark),
  ]) {
    testWidgets('the gutter and a card, $name', (tester) async {
      await tester.pumpWidget(
        themed(
          tokens: tokens,
          const Padding(
            padding: EdgeInsets.all(16),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              spacing: 16,
              children: [
                RepaintBoundary(
                  key: Key('gutter'),
                  child: SizedBox(
                    width: 420,
                    height: 360,
                    child: SourceView(
                      changeShown,
                      path: 'main.go',
                      base: changeBase,
                      padding: _pad,
                    ),
                  ),
                ),
                RepaintBoundary(
                  key: Key('card'),
                  child: ChangeCard(
                    marks: [
                      LineMark(MarkKind.removed, 0, 0, ['func old() {}', '']),
                      LineMark(MarkKind.changed, 0, 2, [
                        '// greet says hello.',
                        'func greet() {',
                        '\tfmt.Println("hi")',
                      ]),
                    ],
                  ),
                ),
              ],
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();
      await expectLater(
        find.byKey(const Key('gutter')),
        matchesGoldenFile('goldens/change_marks_gutter_$name.png'),
      );
      await expectLater(
        find.byKey(const Key('card')),
        matchesGoldenFile('goldens/change_marks_card_$name.png'),
      );
    });
  }

  group('the editor', () {
    Finder marksIn() => find.descendant(
      of: find.byType(SourceEditor),
      matching: find.byType(ChangeMark),
    );

    testWidgets('marks its lines, follows typing after a pause, unsaved', (
      tester,
    ) async {
      final (d, fake) = await openEditorWith(tester);
      await tester.pumpAndSettle();
      expect(marksIn(), findsNWidgets(7));
      // Each on its line: the numbers' own rows.
      final rows = tester.widget<SourceEditor>(find.byType(SourceEditor));
      expect(rows.base!.value, changeBase);

      // A line typed at the top: a new mark once typing pauses, kept while
      // the edit is unsaved.
      final e = editOf(d);
      e.controller.selection = const CodeLineSelection.collapsed(
        index: 0,
        offset: 0,
      );
      await typeIn(tester, e, '// top\n');
      expect(marksIn(), findsNWidgets(7));
      await tester.pump(Waits.typing);
      await tester.pump();
      expect(marksIn(), findsNWidgets(8));
      expect(e.unsaved, isTrue);
      final top = tester.getTopLeft(marksIn().first).dy;
      final numbers = tester.getTopLeft(find.byType(DefaultCodeLineNumber)).dy;
      expect(top, closeTo(numbers + 22, 2));
      expect(fake.fileDiffs, isNotEmpty);
      await endEditor(tester, d);
    }, variant: onMac);

    testWidgets("its text starts 14 after the numbers, as the source's", (
      tester,
    ) async {
      final (d, _) = await openEditorWith(tester);
      await tester.pumpAndSettle();
      final editor = tester.widget<CodeEditor>(find.byType(CodeEditor));
      final numbers = tester.getTopRight(find.byType(DefaultCodeLineNumber));
      final marks = find.ancestor(
        of: find.byType(DefaultCodeLineNumber),
        matching: find.byType(Stack),
      );
      final gutter = tester.getTopRight(marks.first);
      expect(gutter.dx - numbers.dx, markGap + markBar);
      expect(gutter.dx - numbers.dx + (editor.padding! as EdgeInsets).left, 14);
      await endEditor(tester, d);
    }, variant: onMac);
  });
}

/// The editor on `main.go` as [changeShown], its last commit's text
/// [changeBase].
Future<(Daemon, FakeDaemon)> openEditorWith(
  WidgetTester tester, {
  double width = 1200,
  Brightness brightness = Brightness.light,
}) => openEditor(
  tester,
  {'main.go': utf8.encode(changeShown)},
  width: width,
  brightness: brightness,
  fileDiffs: {
    '/Users/me/nushi/main.go': FileChange(
      path: 'main.go',
      status: 'modified',
      oldText: changeBase,
    ),
  },
);
