import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nushi/beside_fit.dart';
import 'package:nushi/chat.dart';
import 'package:nushi/chat_header.dart';
import 'package:nushi/chat_panes.dart';
import 'package:nushi/daemon.dart';
import 'package:nushi/details_panel.dart';
import 'package:nushi/files_pane.dart';
import 'package:nushi/main.dart';
import 'package:nushi/palette.dart';
import 'package:nushi/panes.dart';
import 'package:nushi/preview_pane.dart';
import 'package:nushi/settings.dart';
import 'package:nushi/side_chat.dart';
import 'package:nushi/subagent_pane.dart';
import 'package:nushi/terminal_panel.dart';
import 'package:nushi/transcript.dart';

import 'fake_daemon.dart' show loadFonts;
import 'harness.dart';
import 'right_panes_fixture.dart';

// The details rail tucks itself away when the panes beside the chat need
// its room, and comes back when there's room again (ADR 0123): decided on
// the room alone, never saved, the header's button still the user's
// choice. Only once the transcript is too narrow even then do the panes
// lie over it.

final _mac = TargetPlatformVariant.only(TargetPlatform.macOS);

PanesOpen _openOf(List<PaneSpec> panes) => (
  side: panes.contains(ChatPanes.side),
  files: panes.contains(ChatPanes.files),
  subagent: panes.contains(ChatPanes.subagent),
  command: false,
  previews: panes.contains(ChatPanes.preview) ? 1 : 0,
  browser: 0,
);

Type _typeOf(PaneSpec p) => switch (p.name) {
  'side' => SideChat,
  'files' => FilesPane,
  'subagent' => SubagentPane,
  _ => PreviewPane,
};

/// The rail shows (drawn: not tucked away).
bool _rail() => find.byType(DetailsPanel).evaluate().isNotEmpty;

/// The panes lie over the transcript: the first starts under the header.
bool _over(WidgetTester tester, List<PaneSpec> panes) =>
    tester.getRect(find.byType(_typeOf(panes.first))).top >
    tester.getRect(find.byType(Pane)).top;

/// The header's details button, by its tooltip.
Finder get _button => find.descendant(
  of: find.byType(ChatHeader),
  matching: find.byWidgetPredicate(
    (w) =>
        w is Tooltip &&
        ((w.message ?? '').endsWith('details') ||
            (w.message ?? '').startsWith('Hide details') ||
            w.message == 'The window is too narrow'),
  ),
);

String _tip(WidgetTester tester) => tester.widget<Tooltip>(_button).message!;

void main() {
  setUpAll(loadFonts);

  // Every row of the table, the sidebar shown and hidden, at its line and
  // a point either side: at the line it's the better state.
  for (final row in thresholdRows) {
    for (final sidebar in [true, false]) {
      final where = sidebar ? 'sidebar shown' : 'sidebar hidden';
      final beside = switch (row.beside) {
        final b? => windowFor(b, sidebar: sidebar).toInt(),
        null => '-',
      };
      testWidgets('${row.name}, $where: the rail from '
          '${windowFor(row.rail, sidebar: sidebar).toInt()}, beside from '
          '$beside', (tester) async {
        final rail = windowFor(row.rail, sidebar: sidebar);
        final (d, _) = await panesApp(
          tester,
          width: rail,
          sidebar: sidebar,
          open: _openOf(row.panes),
        );
        Future<void> at(double w) async {
          viewport(tester, Size(w, 900));
          await tester.pumpAndSettle();
        }

        for (final (w, shows) in [
          (rail + 1, true),
          (rail, true),
          (rail - 1, false),
        ]) {
          await at(w);
          expect(_rail(), shows, reason: 'at $w');
          if (row.panes.isNotEmpty) expect(_over(tester, row.panes), false);
          if (!sidebar && w == rail || !sidebar && w == rail - 1) {
            await render(
              tester,
              find.byType(NushiApp),
              'rail_line_${row.name}_${w.toInt()}',
            );
          }
        }
        if (row.beside case final b?) {
          final beside = windowFor(b, sidebar: sidebar);
          for (final (w, over) in [(beside, false), (beside - 1, true)]) {
            await at(w);
            expect(_rail(), false, reason: 'at $w');
            expect(_over(tester, row.panes), over, reason: 'at $w');
            if (!sidebar) {
              await render(
                tester,
                find.byType(NushiApp),
                'beside_line_${row.name}_${w.toInt()}',
              );
            }
          }
        }
        expect(showDetails.value, isTrue);
        await closeView(tester, d);
      }, variant: _mac);
    }
  }

  testWidgets('hiding it for room is never saved, and the button keeps the '
      "user's choice", (tester) async {
    // Files and the preview: the rail tucks under 1500 for the chat.
    final width = windowFor(1450, sidebar: false);
    final (d, _) = await panesApp(tester, width: width + 200);
    await tester.pump(const Duration(seconds: 2));
    final saved = readSaved();
    var told = 0;
    void count() => told++;
    showDetails.addListener(count);
    addTearDown(() => showDetails.removeListener(count));
    viewport(tester, Size(width, 900));
    await openPanesIn(tester, d, _openOf([ChatPanes.files, ChatPanes.preview]));
    expect(_rail(), false);
    expect(told, 0);
    await tester.pump(const Duration(seconds: 2)); // a save would be done
    expect(readSaved()['details'], saved['details']);
    expect((readSaved()['panes'] as Map?)?['details'], isNull);
    // The button: lit, it hides them, and says they're tucked away.
    final button = tester.widget<Tooltip>(_button);
    expect(
      button.message,
      'Hide details (tucked away while the panes need the room)',
    );
    final header = tester.getRect(find.byType(ChatHeader));
    final preview = tester.getRect(find.byType(PreviewPane));
    final menu = tester.getRect(find.byTooltip('Chat menu'));
    await tester.tap(_button);
    await tester.pumpAndSettle();
    expect(showDetails.value, isFalse);
    expect(_tip(tester), 'Show details');
    // Nothing moved.
    expect(tester.getRect(find.byType(ChatHeader)), header);
    expect(tester.getRect(find.byType(PreviewPane)), preview);
    expect(tester.getRect(find.byTooltip('Chat menu')), menu);
    await tester.tap(_button);
    await tester.pumpAndSettle();
    expect(showDetails.value, isTrue);
    expect(_rail(), false);
    expect(tester.getRect(find.byType(PreviewPane)), preview);
    // Room again: they're back. Not wanted, they don't come.
    viewport(tester, Size(width + 200, 900));
    await tester.pumpAndSettle();
    expect(_rail(), true);
    showDetails.value = false;
    viewport(tester, Size(width, 900));
    await tester.pumpAndSettle();
    viewport(tester, Size(width + 400, 900));
    await tester.pumpAndSettle();
    expect(_rail(), false);
    await closeView(tester, d);
  }, variant: _mac);

  group('it comes back when there is room again:', () {
    // The subagent's pane at the line, less a point: tucked.
    final width = windowFor(944, sidebar: true) - 1;

    Future<void> tucked(WidgetTester tester) async {
      expect(_rail(), false);
      expect(find.byType(DetailsPanel, skipOffstage: false), findsOneWidget);
    }

    /// It slides in: drawn right of where it ends on its first frame.
    Future<void> slidesBack(WidgetTester tester) async {
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 16));
      expect(_rail(), true);
      final first = tester.getRect(find.byType(DetailsPanel)).left;
      await tester.pumpAndSettle();
      expect(_rail(), true);
      expect(
        first,
        greaterThan(tester.getRect(find.byType(DetailsPanel)).left),
      );
    }

    Future<Daemon> app(WidgetTester tester) async {
      final (d, _) = await panesApp(
        tester,
        width: width,
        sidebar: true,
        open: _openOf([ChatPanes.subagent]),
      );
      await tucked(tester);
      return d;
    }

    testWidgets('the window widens', (tester) async {
      final d = await app(tester);
      viewport(tester, Size(width + 40, 900));
      await slidesBack(tester);
      await closeView(tester, d);
    }, variant: _mac);

    testWidgets('the pane closes', (tester) async {
      final d = await app(tester);
      await tester.tap(
        find.descendant(
          of: find.byType(SubagentPane),
          matching: find.byTooltip('Close'),
        ),
      );
      await slidesBack(tester);
      await closeView(tester, d);
    }, variant: _mac);

    testWidgets('its edge is dragged narrower', (tester) async {
      final d = await app(tester);
      final pane = tester.getRect(find.byType(SubagentPane));
      final drag = await tester.startGesture(
        Offset(pane.left + Resizable.edgeWidth / 2, pane.center.dy),
      );
      await drag.moveBy(const Offset(20, 0));
      await tester.pump();
      await drag.moveBy(const Offset(20, 0));
      await slidesBack(tester);
      await drag.up();
      await tester.pump(const Duration(seconds: 2));
      expect(_rail(), true);
      await closeView(tester, d);
    }, variant: _mac);

    testWidgets('its edge, dragged wide, is reset with a double-click', (
      tester,
    ) async {
      final (d, _) = await panesApp(
        tester,
        width: width + 40,
        sidebar: true,
        open: _openOf([ChatPanes.subagent]),
        widths: {'subagent': 400},
      );
      await tucked(tester);
      final pane = tester.getRect(find.byType(SubagentPane));
      final at = Offset(pane.left + Resizable.edgeWidth / 2, pane.center.dy);
      await tester.tapAt(at);
      await tester.pump(const Duration(milliseconds: 50));
      await tester.tapAt(at);
      await slidesBack(tester);
      expect(paneWidths.value['subagent'], isNull);
      await closeView(tester, d);
    }, variant: _mac);

    testWidgets('the sidebar hides', (tester) async {
      final d = await app(tester);
      sidebarHidden.value = true;
      await slidesBack(tester);
      await closeView(tester, d);
    }, variant: _mac);

    testWidgets('the Files pane closes (⇧⌘E)', (tester) async {
      final (d, _) = await panesApp(
        tester,
        width: windowFor(1204, sidebar: true) - 1,
        sidebar: true,
        open: _openOf([ChatPanes.files, ChatPanes.subagent]),
      );
      await tucked(tester);
      expect(find.byType(FilesPane), findsOneWidget);
      toggleFiles(_chat(tester));
      await slidesBack(tester);
      await closeView(tester, d);
    }, variant: _mac);

    testWidgets('the side chat hides (⇧⌘B)', (tester) async {
      final (d, _) = await panesApp(
        tester,
        width: windowFor(1324, sidebar: true) - 1,
        sidebar: true,
        open: _openOf([ChatPanes.side, ChatPanes.subagent]),
      );
      await tucked(tester);
      _chat(tester).sideHidden.value = true;
      await slidesBack(tester);
      await closeView(tester, d);
    }, variant: _mac);
  });

  testWidgets("with no pane, the window's own rule (ADR 0092): too narrow "
      'under 804 for the chat, the button disabled and saying so', (
    tester,
  ) async {
    for (final sidebar in [true, false]) {
      final line = windowFor(804, sidebar: sidebar);
      final (d, _) = await panesApp(tester, width: line, sidebar: sidebar);
      expect(_rail(), true);
      expect(_tip(tester), 'Hide details');
      viewport(tester, Size(line - 1, 900));
      await tester.pumpAndSettle();
      expect(_rail(), false);
      expect(_tip(tester), 'The window is too narrow');
      // Gone, not tucked: as before.
      expect(find.byType(DetailsPanel, skipOffstage: false), findsNothing);
      await closeView(tester, d);
    }
  }, variant: _mac);

  testWidgets('a pane open in a chat area under 804: too narrow, no '
      'button', (tester) async {
    final (d, _) = await panesApp(
      tester,
      width: windowFor(800, sidebar: false),
      open: _openOf([ChatPanes.subagent]),
    );
    expect(_rail(), false);
    // Its button folds into ⋯ first, and ⋯ has no details either.
    expect(_button, findsNothing);
    await closeView(tester, d);
  }, variant: _mac);

  testWidgets('over the transcript: the header across it all, the panes '
      "under it and over the terminal, the first one's shadow on it", (
    tester,
  ) async {
    final (d, _) = await panesApp(
      tester,
      width: windowFor(1500, sidebar: false),
      terminal: true,
      open: _openOf([ChatPanes.side, ChatPanes.files, ChatPanes.preview]),
    );
    final area = tester.getRect(find.byType(Pane));
    final header = tester.getRect(find.byType(ChatHeader));
    final terminal = tester.getRect(find.byType(TerminalPanel));
    expect(header.width, area.width);
    for (final t in [SideChat, FilesPane, PreviewPane]) {
      final r = tester.getRect(find.byType(t));
      expect(r.top, header.bottom);
      expect(r.bottom, terminal.top);
    }
    final transcript = tester.getRect(
      find.byWidgetPredicate(
        (w) => w is Transcript && w.items == null && w.chat.id == 'c',
      ),
    );
    expect(transcript.right, area.right);
    // The shadow, on the first pane alone.
    bool shadowed(Type pane) => tester
        .widgetList<DecoratedBox>(
          find.ancestor(
            of: find.byType(pane),
            matching: find.byType(DecoratedBox),
          ),
        )
        .any(
          (b) => switch (b.decoration) {
            BoxDecoration(:final boxShadow?) => boxShadow.isNotEmpty,
            _ => false,
          },
        );
    expect(shadowed(SideChat), isTrue);
    expect(shadowed(FilesPane), isFalse);
    expect(shadowed(PreviewPane), isFalse);
    await closeView(tester, d);
  }, variant: _mac);

  testWidgets('Artifacts… lists in the palette while the rail is tucked, '
      'and opens it where there is room', (tester) async {
    final (d, _) = await panesApp(
      tester,
      width: windowFor(1100, sidebar: false),
      open: _openOf([ChatPanes.preview]),
    );
    final c = d.chats['c']!;
    expect(c.panelFits(), isFalse);
    showDetails.value = false;
    await tester.pumpAndSettle();
    revealArtifacts(tester.element(find.byType(ChatHeader)), c);
    await tester.pumpAndSettle();
    expect(find.byType(Palette), findsOneWidget);
    expect(showDetails.value, isFalse);
    Navigator.of(tester.element(find.byType(Palette))).pop();
    await tester.pumpAndSettle();
    c.preview.value = null;
    await tester.pumpAndSettle();
    expect(c.panelFits(), isTrue);
    revealArtifacts(tester.element(find.byType(ChatHeader)), c);
    await tester.pumpAndSettle();
    expect(find.byType(Palette), findsNothing);
    expect(showDetails.value, isTrue);
    expect(_rail(), true);
    await closeView(tester, d);
  }, variant: _mac);

  testWidgets("an archived chat's side chat takes no room", (tester) async {
    // At the preview's own line: the rail shows beside it, the side chat
    // not placed.
    final (d, _) = await panesApp(
      tester,
      width: windowFor(1240, sidebar: false),
      archived: true,
      open: _openOf([ChatPanes.side, ChatPanes.preview]),
    );
    expect(find.byType(SideChat, skipOffstage: false), findsNothing);
    expect(find.byType(PreviewPane), findsOneWidget);
    expect(_rail(), true);
    await closeView(tester, d);
  }, variant: _mac);

  testWidgets('Changes and back, tucked: nothing slides, the header across '
      'on Changes', (tester) async {
    final (d, _) = await panesApp(
      tester,
      width: windowFor(1100, sidebar: false),
      open: _openOf([ChatPanes.preview]),
    );
    final c = d.chats['c']!;
    final preview = tester.getRect(find.byType(PreviewPane));
    expect(_rail(), false);
    c.tab.value = ChatTab.changes;
    await tester.pump();
    expect(find.byType(PreviewPane), findsNothing);
    expect(
      find.byType(DetailsPanel, skipOffstage: false),
      findsNothing,
      reason: 'no details on Changes',
    );
    expect(
      tester.getRect(find.byType(ChatHeader)).width,
      tester.getRect(find.byType(Pane)).width,
    );
    await tester.pumpAndSettle();
    c.tab.value = ChatTab.chat;
    await tester.pump();
    expect(tester.getRect(find.byType(PreviewPane)), preview);
    expect(_rail(), false);
    await tester.pumpAndSettle();
    expect(tester.getRect(find.byType(PreviewPane)), preview);
    await closeView(tester, d);
  }, variant: _mac);

  testWidgets('another chat with room shows its rail at once, and back '
      'the first is tucked, with no slide', (tester) async {
    final (d, _) = await panesApp(
      tester,
      width: windowFor(1100, sidebar: false),
      open: _openOf([ChatPanes.preview]),
    );
    final preview = tester.getRect(find.byType(PreviewPane));
    d.select('d');
    await tester.pump();
    await tester.pump();
    expect(_rail(), true);
    final rail = tester.getRect(find.byType(DetailsPanel));
    await tester.pumpAndSettle();
    expect(tester.getRect(find.byType(DetailsPanel)), rail);
    d.select('c');
    await tester.pump();
    await tester.pump();
    expect(_rail(), false);
    expect(tester.getRect(find.byType(PreviewPane)), preview);
    await closeView(tester, d);
  }, variant: _mac);
}

ChatState _chat(WidgetTester tester) =>
    tester.widget<ChatHeader>(find.byType(ChatHeader)).chat;
