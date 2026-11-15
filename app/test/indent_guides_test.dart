import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nushi/indent_guides.dart';
import 'package:nushi/settings.dart';
import 'package:nushi/source_editor.dart';
import 'package:nushi/source_spans.dart';
import 'package:nushi/source_view.dart';
import 'package:nushi/theme/theme.dart';
import 'package:nushi/whitespace_marks.dart';

import 'fake_daemon.dart';
import 'file_edit_helpers.dart';
import 'harness.dart';

// Indent guides (the Files pane's ⋯, Claude desktop's): a faint line down
// each indent level, in the source and the editor.

/// Python, indented with four spaces, a blank line inside its block.
const _python =
    'def brew(tea):\n'
    '    if tea:\n'
    '        pour(tea)\n'
    '\n'
    '        stir(tea)\n'
    '    return tea\n';

/// The x of each guide [of] paints.
List<double> _guides(WidgetTester tester, Finder of) {
  final xs = <double>[];
  expect(
    tester.renderObject(of),
    paints..everything((method, args) {
      if (method == #drawLine) xs.add((args[0] as Offset).dx);
      return true;
    }),
  );
  return xs;
}

Finder _line(String text) => find.ancestor(
  of: find.textContaining(text, findRichText: true).first,
  matching: find.byType(IndentGuides),
);

void main() {
  setUpAll(loadFonts);

  group('indentLevels', () {
    test('leading columns in steps of the tab size, a tab to its stop', () {
      expect(indentLevels(['    a', '\ta', '  \ta', '        a', 'a'], 4), [
        1,
        1,
        1,
        2,
        0,
      ]);
      expect(indentLevels(['    a', '\ta', '   a'], 2), [2, 1, 1]);
      expect(indentLevels(['\t\ta'], 8), [2]);
    });

    test('a blank line takes the lesser of the lines around it; one at the '
        'end or the start, none', () {
      expect(indentLevels(['a', '    b', '', '        c', '    d', ''], 4), [
        0,
        1,
        1,
        2,
        1,
        0,
      ]);
      expect(indentLevels(['', '  ', '    a'], 4), [0, 0, 1]);
    });

    test('a blank run past the look has none, and one pass each way does '
        'it', () {
      final lines = [
        '        a',
        for (var i = 0; i < 500; i++) '',
        '        b',
      ];
      final levels = indentLevels(lines, 4);
      // The line with text after it is 500 on: past the look, so none.
      expect(levels[1], 0);
      expect(levels[250], 0, reason: 'past 200 lines each way');
      expect(levels[500], 0);
      expect(
        indentLevels(['    a', ...List.filled(199, ''), '    b'], 4)[1],
        1,
      );
      // Large and linear: 50,000 lines in one go.
      final big = [
        for (var i = 0; i < 50000; i++)
          if (i.isEven) '    x' else '',
      ];
      expect(indentLevels(big, 4), hasLength(50000));
    });

    test('the guides sit at each level’s first column', () {
      expect(guidesAt(3, 28).toList(), [0, 28, 56]);
      expect(guidesAt(0, 28), isEmpty);
    });
  });

  for (final wrap in [true, false]) {
    testWidgets('the source view draws a line per level, the blank line '
        'too, a tab’s columns apart; none with them off, wrap $wrap', (
      tester,
    ) async {
      viewport(tester, const Size(900, 600));
      Future<void> show({required bool guides, int size = 4}) async {
        await tester.pumpWidget(
          themed(
            SourceView(
              _python,
              path: 'brew.py',
              wrap: wrap,
              tabSize: size,
              guides: guides,
            ),
          ),
        );
        await tester.pump();
      }

      await show(guides: true);
      final context = tester.element(find.byType(SourceView));
      final column = columnWidth(
        DefaultTextStyle.of(context).style
            .merge(NushiTheme.of(context).code(Colors.black, size: 12)),
        TextScaler.noScaling,
      );
      final pour = _guides(tester, _line('pour'));
      expect(pour, hasLength(2));
      expect(pour[1] - pour[0], closeTo(4 * column, 1));
      expect(_guides(tester, _line('if tea')), hasLength(1));
      expect(_guides(tester, _line('def brew')), isEmpty);
      // The blank line between pour and stir keeps both.
      expect(
        tester
            .widgetList<IndentGuides>(find.byType(IndentGuides))
            .map((g) => g.level),
        containsAllInOrder([0, 1, 2, 2, 2, 1]),
      );
      final line = tester.getRect(_line('pour'));
      expect(
        tester.getSize(find.byType(IndentGuides).at(2)).height,
        line.height,
      );
      expect(
        tester.widget<IndentGuides>(_line('pour')).color,
        whitespaceColor(NushiTheme.of(context).t),
      );

      await show(guides: true, size: 2);
      expect(_guides(tester, _line('pour')), hasLength(4));

      await show(guides: false);
      expect(_guides(tester, _line('pour')), isEmpty);
    });
  }

  testWidgets('the editor draws them where re_editor lays its lines out; '
      'the text and the saved bytes are as they were', (tester) async {
    final (d, fake) = await openEditor(
      tester,
      {'brew.py': utf8.encode(_python)},
      open: const ['brew.py'],
    );
    expect(find.byType(IndentGuideOverlay), findsOneWidget);
    final lines = <(Offset, Offset)>[];
    expect(
      tester.renderObject(
        find.descendant(
          of: find.byType(IndentGuideOverlay),
          matching: find.byType(CustomPaint),
        ),
      ),
      paints..everything((method, args) {
        if (method == #drawLine) {
          lines.add((args[0] as Offset, args[1] as Offset));
        }
        return true;
      }),
    );
    // def 0, if 1, pour 2, the blank 2, stir 2, return 1.
    expect(lines, hasLength(8));
    final xs = {for (final (a, _) in lines) a.dx}.toList()..sort();
    expect(xs, hasLength(2));
    final step =
        4 *
        columnWidth(
          NushiTheme.of(tester.element(find.byType(SourceEditor)))
              .code(Colors.black, size: 12),
          TextScaler.noScaling,
        );
    expect(xs[1] - xs[0], closeTo(step, 1));
    final e = editOf(d, 'brew.py');
    expect(e.controller.text, _python);
    indentGuides.value = false;
    await tester.pump();
    expect(find.byType(IndentGuideOverlay), findsNothing);
    indentGuides.value = true;
    await tester.pump();
    expect(find.byType(IndentGuideOverlay), findsOneWidget);
    await typeIn(tester, e, 'x');
    await e.save();
    await tester.pump();
    expect(utf8.decode(fake.writes.last.data), contains('def brew'));
    expect(utf8.decode(fake.writes.last.data), isNot(contains('│')));
    await endEditor(tester, d);
  }, variant: onMac);

  for (final (name, tokens) in [
    ('dark', Tokens.dark),
    ('light', Tokens.light),
  ]) {
    testWidgets('guides in the source and the editor, 4 and 2, $name', (
      tester,
    ) async {
      viewport(tester, const Size(520, 140));
      await tester.pumpWidget(
        themed(
          tokens: tokens,
          RepaintBoundary(
            key: const ValueKey('guides'),
            child: ColoredBox(
              color: tokens.base,
              child: Row(
                children: [
                  for (final size in [4, 2])
                    Expanded(
                      child: SourceView(
                        _python,
                        path: 'brew.txt',
                        wrap: false,
                        tabSize: size,
                        guides: true,
                      ),
                    ),
                ],
              ),
            ),
          ),
        ),
      );
      await tester.pump();
      await expectLater(
        find.byKey(const ValueKey('guides')),
        matchesGoldenFile('goldens/indent_guides_$name.png'),
      );
    }, variant: onMac);
  }
}
