import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nushi/browser_pane.dart';
import 'package:nushi/files_pane.dart';
import 'package:nushi/side_chat.dart';
import 'package:nushi/subagent_pane.dart';

import 'browser_fixture.dart' show browserFakes;
import 'harness.dart';
import 'panes_app.dart';

// Panes in a narrow window (ADR 0123): under their summed least they lie
// over the transcript and narrow to fit, each down to the least its rows
// hold before the others give, so no row of theirs overflows in a window
// from 600.

final _mac = TargetPlatformVariant.only(TargetPlatform.macOS);

/// Each laid-out row whose children overflow it (a [RenderFlex] says so
/// in its short description), by where it was made.
List<String> _overflows(WidgetTester tester) => [
  for (final e
      in find
          .descendant(
            of: find.byWidgetPredicate(
              (w) =>
                  w is SideChat ||
                  w is FilesPane ||
                  w is BrowserPane ||
                  w is SubagentPane,
            ),
            matching: find.byWidgetPredicate((w) => w is Flex),
          )
          .evaluate())
    if (e.renderObject case final RenderFlex r
        when r.hasSize && r.toStringShort().contains('OVERFLOWING'))
      '${r.size.width.toStringAsFixed(1)} ${_where(e)}',
];

/// The file and line that made [e].
String _where(Element e) {
  final chain = e.debugGetCreatorChain(9);
  return RegExp(r'lib/([a-z_]+\.dart:\d+)').firstMatch(chain)?.group(1) ??
      chain;
}

PanesOpen _with({bool subagent = false, bool browser = false}) => (
  side: true,
  files: true,
  subagent: subagent,
  command: false,
  previews: 1,
  browser: browser ? 1 : 0,
);

void main() {
  for (final (name, open) in [
    ('side + Files + browser', _with(browser: true)),
    ('side + Files + subagent', _with(subagent: true)),
  ]) {
    for (final sidebar in [true, false]) {
      testWidgets('$name, sidebar ${sidebar ? 'shown' : 'hidden'}: '
          'nothing overflows from 600 to 900', (tester) async {
        browserFakes();
        final found = <String>[];
        // A row reports its overflow once, in whichever frame it first
        // has one (a pane sliding in, too): the settled rows are what's
        // checked, each window afresh.
        final was = FlutterError.onError;
        FlutterError.onError = (d) {
          if (!d.toString().contains('overflowed')) was?.call(d);
        };
        addTearDown(() => FlutterError.onError = was);
        for (var w = 900.0; w >= 600; w -= 10) {
          final (d, _) = await panesApp(
            tester,
            width: w,
            sidebar: sidebar,
            open: open,
          );
          found.addAll([for (final o in _overflows(tester)) '$w: $o']);
          await closeView(tester, d);
        }
        FlutterError.onError = was;
        expect(found, isEmpty);
      }, variant: _mac);
    }
  }
}
