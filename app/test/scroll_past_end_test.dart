import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nushi/settings.dart';
import 'package:nushi/source_editor.dart';
import 'package:nushi/source_spans.dart';
import 'package:nushi/source_view.dart';

import 'file_edit_helpers.dart';
import 'harness.dart';

// Scroll past end (the Files pane's ⋯, Claude desktop's): a source scrolls
// on until its last line is at the top.

/// 80 lines: more than any view here shows.
final String _long = [for (var i = 1; i <= 80; i++) 'line $i'].join('\n');

const _pad = EdgeInsets.fromLTRB(28, 22, 28, 28);

/// The vertical scroll of the view below [of].
ScrollPosition _down(WidgetTester tester, Finder of) => tester
    .state<ScrollableState>(
      find
          .descendant(
            of: of,
            matching: find.byWidgetPredicate(
              (w) => w is Scrollable && w.axisDirection == AxisDirection.down,
            ),
          )
          .first,
    )
    .position;

void main() {
  test('the room under the last line puts it at the top, never less than '
      'the inset', () {
    expect(pastEndPadding(600, 18, top: 28, bottom: 28), 554);
    expect(pastEndPadding(10, 18, top: 28, bottom: 28), 28);
  });

  for (final wrap in [true, false]) {
    testWidgets('the source view: off, its end is the last line; on, the '
        'last line scrolls to the top, wrap $wrap', (tester) async {
      viewport(tester, const Size(600, 400));
      Future<Rect> end({required bool on}) async {
        await tester.pumpWidget(
          themed(
            SourceView(
              _long,
              path: 'a.txt',
              wrap: wrap,
              padding: _pad,
              pastEnd: on,
            ),
          ),
        );
        await tester.pump();
        final p = _down(tester, find.byType(SourceView));
        p.jumpTo(p.maxScrollExtent);
        await tester.pump();
        return tester.getRect(
          find.textContaining('line 80', findRichText: true).first,
        );
      }

      final off = await end(on: false);
      final view = tester.getRect(find.byType(SourceView));
      expect(off.bottom, closeTo(view.bottom - _pad.bottom, 1));
      final on = await end(on: true);
      expect(on.top, closeTo(view.top + _pad.top, 1));
      await render(tester, find.byType(SourceView), 'scroll_past_end_$wrap');
    });
  }

  testWidgets('the editor scrolls on as far, and as the window grows', (
    tester,
  ) async {
    final (d, _) = await openEditor(
      tester,
      {'a.txt': utf8.encode(_long)},
      open: const ['a.txt'],
    );
    double most() => _down(tester, find.byType(SourceEditor)).maxScrollExtent;
    final off = most();
    scrollPastEnd.value = true;
    await tester.pumpAndSettle();
    final on = most();
    final height = tester.getSize(find.byType(SourceEditor)).height;
    // The room it adds: the view less a line and both insets.
    expect(on - off, greaterThan(height - 2 * 28 - 22 - 20));
    expect(on - off, lessThan(height));
    // A taller view has as far to go: the room follows its height, so the
    // last line still comes to the top.
    viewport(tester, const Size(1700, 1000));
    await tester.pumpAndSettle();
    expect(most(), closeTo(on, 1));
    await endEditor(tester, d);
  }, variant: onMac);
}
