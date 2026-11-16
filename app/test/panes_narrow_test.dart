import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nushi/browser_pane.dart';
import 'package:nushi/controls.dart';
import 'package:nushi/file_view.dart';
import 'package:nushi/files_bar.dart';
import 'package:nushi/files_pane.dart';
import 'package:nushi/files_tree.dart';
import 'package:nushi/folder_tree.dart';
import 'package:nushi/settings.dart' show filesTree;
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

PanesOpen _with({
  bool subagent = false,
  bool browser = false,
  int previews = 1,
}) => (
  side: true,
  files: true,
  subagent: subagent,
  command: false,
  previews: previews,
  browser: browser ? 1 : 0,
);

/// Overflows reported only as [_overflows] finds them: a row reports its
/// overflow once, in whichever frame it first has one (a pane sliding in,
/// too), so the settled rows are what's checked, each window afresh. The
/// handler that was, to put back.
void Function(FlutterErrorDetails)? _settledRowsOnly() {
  final was = FlutterError.onError;
  FlutterError.onError = (d) {
    if (!d.toString().contains('overflowed')) was?.call(d);
  };
  addTearDown(() => FlutterError.onError = was);
  return was;
}

/// The width the Files pane gives its tree (its edge's box).
double _treeRoom(WidgetTester tester) => tester
    .getSize(
      find.ancestor(
        of: find.byType(FilesTree),
        matching: find.byKey(const ValueKey('tree')),
      ),
    )
    .width;

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
        final was = _settledRowsOnly();
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

  for (final sidebar in [true, false]) {
    testWidgets('side chat, Files and the browser in a 720 window, sidebar '
        "${sidebar ? 'shown' : 'hidden'}: Files' viewer has all of it, the "
        'tree slid away, its choice kept', (tester) async {
      browserFakes();
      _settledRowsOnly();
      final (d, _) = await panesApp(
        tester,
        width: 720,
        sidebar: sidebar,
        open: _with(browser: true),
      );
      final files = tester.getSize(find.byType(FilesPane)).width;
      // Narrowed alike, its viewer had 83 here and its tree 57, the file's
      // name breaking a letter or two a line.
      expect(files, lessThan(300));
      expect(tester.getSize(find.byType(FileView)).width, closeTo(files, 0.01));
      expect(_treeRoom(tester), 0);
      expect(filesTree.value, isTrue);
      expect(_overflows(tester), isEmpty);
      await closeView(tester, d);
    }, variant: _mac);
  }

  testWidgets("squeezed between the viewer's least and both's, the tree "
      'gives on; with no file the blank names ⌘P once it has gone', (
    tester,
  ) async {
    browserFakes();
    _settledRowsOnly();
    // A chat of 969, 980 past the title buttons: the three need 1100,
    // and Files narrows to 400.
    final (d, _) = await panesApp(
      tester,
      width: 1185,
      open: _with(browser: true),
    );
    expect(tester.getSize(find.byType(FilesPane)).width, closeTo(400, 0.01));
    expect(tester.getSize(find.byType(FileView)).width, closeTo(300, 0.01));
    expect(_treeRoom(tester), closeTo(100, 0.01));
    await closeView(tester, d);

    final (e, _) = await panesApp(
      tester,
      width: 720,
      open: _with(browser: true, previews: 0),
    );
    expect(_treeRoom(tester), 0);
    expect(find.textContaining('Go to a file with ⌘P'), findsOneWidget);
    expect(_overflows(tester), isEmpty);
    await closeView(tester, e);
  }, variant: _mac);

  testWidgets('squeezed out, the tree has no lit button: it says Show, '
      "it and Collapse can't be had, and the choice is kept", (tester) async {
    browserFakes();
    _settledRowsOnly();
    final (d, _) = await panesApp(
      tester,
      width: 720,
      open: _with(browser: true),
    );
    // A folder open, so only the squeeze keeps Collapse from it.
    folderTreeOf(d.chats['c']!).setOpen('auth', open: true);
    await tester.pumpAndSettle();
    expect(_treeRoom(tester), 0);
    Finder inBar(Finder f) => find.descendant(
      of: find.descendant(
        of: find.byType(FilesPane),
        matching: find.byType(FilesBar),
      ),
      matching: f,
    );
    IconTap button(LineIcon icon) => tester.widget<IconTap>(
      inBar(find.byWidgetPredicate((w) => w is IconTap && w.line == icon)),
    );
    final tree = button(LineIcon.fileTree);
    expect(tree.lit, isFalse);
    expect(tree.tooltip, withKey('Show file tree', '⇧⌘Y'));
    expect(tree.onTap, isNull);
    expect(button(LineIcon.collapseAll).onTap, isNull);

    await tester.tap(inBar(find.byTooltip('More')));
    await tester.pumpAndSettle();
    MenuItemButton row(String label) => tester.widget<MenuItemButton>(
      find.ancestor(
        of: find.text(label),
        matching: find.byType(MenuItemButton),
      ),
    );
    expect(row('Show file tree').onPressed, isNull);
    expect(
      find.descendant(
        of: find.ancestor(
          of: find.text('Show file tree'),
          matching: find.byType(MenuItemButton),
        ),
        matching: find.byIcon(Icons.check),
      ),
      findsNothing,
    );
    expect(row('Collapse all folders').onPressed, isNull);
    expect(filesTree.value, isTrue, reason: 'the choice is kept');
    await tester.tapAt(Offset.zero);
    await tester.pumpAndSettle();
    await closeView(tester, d);
  }, variant: _mac);
}
