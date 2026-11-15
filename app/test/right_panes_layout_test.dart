import 'dart:typed_data';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nushi/background_commands.dart';
import 'package:nushi/beside_fit.dart';
import 'package:nushi/browser_bar.dart';
import 'package:nushi/browser_pane.dart';
import 'package:nushi/chat_header.dart';
import 'package:nushi/chat_panes.dart';
import 'package:nushi/details_panel.dart';
import 'package:nushi/files_pane.dart';
import 'package:nushi/gen/nushi/v1/nushi.pb.dart';
import 'package:nushi/main.dart';
import 'package:nushi/panes.dart';
import 'package:nushi/preview_pane.dart';
import 'package:nushi/preview_path.dart';
import 'package:nushi/preview_tabs.dart';
import 'package:nushi/side_chat.dart';
import 'package:nushi/subagent_pane.dart';
import 'package:nushi/terminal_panel.dart';
import 'package:nushi/theme/theme.dart';
import 'package:nushi/transcript.dart';

import 'browser_fixture.dart' show browserFakes;
import 'fake_daemon.dart' show loadFonts;
import 'harness.dart';
import 'right_panes_fixture.dart';

// Where a chat's parts are with panes open (ADR 0123): the transcript, the
// details rail, then every pane, each the pane's full height with its top
// row level with the chat's header, which spans the transcript and the
// rail; the terminal under those two. Narrower, the rail tucks away;
// narrower still, the panes lie over the transcript, under the header.

final _mac = TargetPlatformVariant.only(TargetPlatform.macOS);

/// A kind of pane, how to open it, what it opens as (its specs, left to
/// right) and the widget each is.
typedef _Kind = ({
  String name,
  PanesOpen open,
  List<PaneSpec> specs,
  List<Type> types,
});

PanesOpen _open({
  bool side = false,
  bool files = false,
  bool subagent = false,
  bool command = false,
  int previews = 0,
  int browser = 0,
}) => (
  side: side,
  files: files,
  subagent: subagent,
  command: command,
  previews: previews,
  browser: browser,
);

final List<_Kind> _kinds = [
  (
    name: 'side',
    open: _open(side: true),
    specs: [ChatPanes.side],
    types: [SideChat],
  ),
  (
    name: 'files',
    open: _open(files: true),
    specs: [ChatPanes.files],
    types: [FilesPane],
  ),
  (
    name: 'subagent',
    open: _open(subagent: true),
    specs: [ChatPanes.subagent],
    types: [SubagentPane],
  ),
  (
    name: 'command',
    open: _open(command: true),
    specs: [ChatPanes.subagent],
    types: [BackgroundCommandPane],
  ),
  (
    name: '3 files',
    open: _open(previews: 3),
    specs: [ChatPanes.preview],
    types: [PreviewPane],
  ),
  (
    name: '2 browser tabs',
    open: _open(browser: 2),
    specs: [ChatPanes.preview],
    types: [BrowserPane],
  ),
  (
    name: 'files + browser',
    open: _open(files: true, browser: 1),
    specs: [ChatPanes.files, ChatPanes.preview],
    types: [FilesPane, BrowserPane],
  ),
];

Rect _rect(WidgetTester tester, Finder f) => tester.getRect(f);

Finder _transcript() => find.byWidgetPredicate(
  (w) => w is Transcript && w.items == null && w.chat.id == 'c',
);

/// The pane's own top row: its first [PaneHeader].
Finder _topRow(Finder pane) =>
    find.descendant(of: pane, matching: find.byType(PaneHeader)).first;

void main() {
  setUpAll(loadFonts);

  for (final width in [1900.0, 1700.0, 720.0]) {
    for (final sidebar in [true, false]) {
      for (final kind in _kinds) {
        for (final terminal in [false, true]) {
          final where = sidebar ? 'the sidebar shown' : 'the sidebar hidden';
          final term = terminal ? ', the terminal open' : '';
          testWidgets('${kind.name} at ${width.toInt()}, $where$term', (
            tester,
          ) async {
            browserFakes();
            final (d, _) = await panesApp(
              tester,
              width: width,
              sidebar: sidebar,
              terminal: terminal,
              open: kind.open,
            );
            expect(tester.takeException(), isNull);
            expectTextInside(tester, width);
            final area = _rect(tester, find.byType(Pane));
            final fit = chatFit(
              room: area.width,
              panes: kind.specs,
              saved: const {},
              wantRail: true,
            );
            final panes = [
              for (final t in kind.types) _rect(tester, find.byType(t)),
            ];
            final header = _rect(tester, find.byType(ChatHeader));
            final menu = _rect(tester, find.byTooltip('Chat menu'));
            final transcript = _rect(tester, _transcript());
            final bottom = terminal
                ? _rect(tester, find.byType(TerminalPanel)).top
                : area.bottom;
            // Left to right, touching, the last at the window's margin.
            for (var i = 1; i < panes.length; i++) {
              expect(panes[i].left, panes[i - 1].right);
            }
            expect(panes.last.right, width - NushiTheme.windowPad);
            expect(header.top, area.top);
            expect(header.left, area.left);
            expect(header.height, PaneHeader.height);
            final first = panes.first;
            if (!fit.over) {
              expect(first.left, area.left + fit.left);
              // The panes run the pane's full height, the terminal or not.
              for (final p in panes) {
                expect(p.top, area.top);
                expect(p.bottom, area.bottom);
              }
              // Each top row level with the chat's header.
              for (final t in kind.types) {
                final row = _rect(tester, _topRow(find.byType(t)));
                expect(row.top, header.top);
                expect(row.bottom, header.bottom);
              }
              expect(header.right, first.left);
              expect(menu.right, first.left - ChatHeader.end);
              if (terminal) {
                final term = _rect(tester, find.byType(TerminalPanel));
                expect(term.left, area.left);
                expect(term.right, first.left);
              }
              if (fit.rail) {
                final rail = _rect(tester, find.byType(DetailsPanel));
                expect(transcript.right, rail.left);
                expect(rail.right, first.left);
                expect(rail.top, header.bottom);
                expect(rail.bottom, bottom);
              } else {
                expect(find.byType(DetailsPanel), findsNothing);
                expect(transcript.right, first.left);
              }
            } else {
              // Over the transcript, under the header, over the terminal.
              expect(header.right, area.right);
              expect(menu.right, area.right - ChatHeader.end);
              expect(transcript.right, area.right);
              expect(find.byType(DetailsPanel), findsNothing);
              for (final p in panes) {
                expect(p.top, header.bottom);
                expect(p.bottom, bottom);
              }
              if (terminal) {
                final term = _rect(tester, find.byType(TerminalPanel));
                expect(term.left, area.left);
                expect(term.right, area.right);
              }
            }
            // Each pane's buttons in its own top row, none of them in the
            // chat's header; one top row each, nothing under it a header.
            for (final t in kind.types) {
              final pane = find.byType(t);
              final row = _rect(tester, _topRow(pane));
              expect(row.top, _rect(tester, pane).top);
              final heads = find.descendant(
                of: pane,
                matching: find.byType(PaneHeader),
              );
              expect(heads, findsOneWidget);
              final close = find.descendant(
                of: pane,
                matching: find.byWidgetPredicate(
                  (w) =>
                      w is Tooltip &&
                      {'Close', 'End the side chat'}.contains(w.message),
                ),
              );
              expect(close, findsAtLeast(1));
              for (final e in close.evaluate()) {
                final r = tester.getRect(find.byWidget(e.widget));
                expect(row.contains(r.center), isTrue, reason: '$t');
              }
            }
            expect(
              find.descendant(
                of: find.byType(ChatHeader),
                matching: find.byTooltip('Close'),
              ),
              findsNothing,
            );
            // The preview's tabs in its top row; the address and the path
            // under it.
            if (kind.types.contains(PreviewPane) ||
                kind.types.contains(BrowserPane)) {
              final pane = find.byType(kind.types.last);
              final row = _rect(tester, _topRow(pane));
              final tabs = find.descendant(
                of: pane,
                matching: find.byType(PreviewTabs),
              );
              if (tabs.evaluate().isNotEmpty) {
                final r = _rect(tester, tabs);
                expect(r.top, greaterThanOrEqualTo(row.top));
                expect(r.bottom, lessThanOrEqualTo(row.bottom));
              }
              final below = kind.types.last == BrowserPane
                  ? find.byType(BrowserBar)
                  : find.byType(PathLine);
              expect(_rect(tester, below).top, row.bottom);
            }
            await closeView(tester, d);
          }, variant: _mac);
        }
      }
    }
  }

  /// The color at ([x], [y]) in logical points of [image], drawn at 1x.
  Color at(ui.Image image, ByteData bytes, double x, double y) {
    final i = (y.floor() * image.width + x.floor()) * 4;
    return Color.fromARGB(
      bytes.getUint8(i + 3),
      bytes.getUint8(i),
      bytes.getUint8(i + 1),
      bytes.getUint8(i + 2),
    );
  }

  for (final (name, brightness, tokens) in [
    ('dark', Brightness.dark, Tokens.dark),
    ('light', Brightness.light, Tokens.light),
  ]) {
    testWidgets("the header's line meets the pane's top row's, and the "
        "pane's edge runs the full height, $name", (tester) async {
      final (d, _) = await panesApp(
        tester,
        width: 1700,
        brightness: brightness,
        open: _open(files: true),
      );
      final files = _rect(tester, find.byType(FilesPane));
      final rail = _rect(tester, find.byType(DetailsPanel));
      final line = Color.alphaBlend(tokens.line, tokens.base);
      void isLine(Color c, String where) {
        expect(c.r, closeTo(line.r, 1.5 / 255), reason: where);
        expect(c.g, closeTo(line.g, 1.5 / 255), reason: where);
        expect(c.b, closeTo(line.b, 1.5 / 255), reason: where);
      }

      final element = find.byType(NushiApp).evaluate().single;
      await tester.runAsync(() async {
        final image = await captureImage(element);
        final bytes = (await image.toByteData())!;
        // The header's line runs on into the pane's, with no gap.
        const y = NushiTheme.windowPad + PaneHeader.height - 1;
        for (final x in [files.left - 30, files.left - 2, files.left + 2]) {
          isLine(at(image, bytes, x, y), 'the top line at $x');
        }
        isLine(at(image, bytes, files.left + 120, y), 'in the pane');
        // The pane's edge from the top to the bottom, the header's band
        // too; the rail's under the header.
        for (final dy in [20.0, 300.0, 880.0]) {
          isLine(at(image, bytes, files.left, dy), 'the edge at $dy');
        }
        isLine(at(image, bytes, rail.left, 300), "the rail's line");
        image.dispose();
      });
      await closeView(tester, d);
    }, variant: _mac);
  }

  testWidgets('a header 360 wide, with a long title and every chip, '
      'folds and keeps ⋯ at its end', (tester) async {
    browserFakes();
    // The subagent's pane dragged as wide as it goes beside the
    // transcript, which keeps its least: the header is 360 wide.
    final width = windowFor(928, sidebar: false);
    final (d, _) = await panesApp(
      tester,
      width: width,
      open: _open(subagent: true),
      widths: {'subagent': 928 - 360},
    );
    d.chats['c']!
      ..title = 'A title long enough to want all of the room it can get'
      ..jiraKey = 'NUSHI-1234'
      ..github = GitHubLink(
        host: 'github.com',
        repo: 'afrase/nushi',
        number: 1234,
      )
      ..linkPull(
        GitHubLink(
          host: 'github.com',
          repo: 'afrase/nushi',
          number: 1240,
          pull: true,
        ),
      );
    await tester.pumpAndSettle();
    final header = _rect(tester, find.byType(ChatHeader));
    expect(header.width, 360);
    expect(tester.takeException(), isNull);
    expectTextInside(tester, width);
    expect(
      _rect(tester, find.byTooltip('Chat menu')).right,
      header.right - ChatHeader.end,
    );
    await closeView(tester, d);
  }, variant: _mac);
}
