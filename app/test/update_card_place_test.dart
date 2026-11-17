import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nushi/browser_pane.dart';
import 'package:nushi/chat_header.dart';
import 'package:nushi/commit_box.dart';
import 'package:nushi/composer.dart';
import 'package:nushi/controls.dart';
import 'package:nushi/daemon.dart';
import 'package:nushi/details_panel.dart';
import 'package:nushi/files_pane.dart';
import 'package:nushi/gen/nushi/v1/nushi.pb.dart';
import 'package:nushi/main.dart';
import 'package:nushi/panes.dart';
import 'package:nushi/pull_tab.dart';
import 'package:nushi/settings.dart';
import 'package:nushi/side_chat.dart';
import 'package:nushi/status_bar.dart';
import 'package:nushi/trust_ask.dart';
import 'package:nushi/update_toast.dart';

import 'browser_fixture.dart' show browserFakes;
import 'fake_daemon.dart' show FakeDaemon, loadFonts;
import 'github_fixture.dart' as gh;
import 'harness.dart';
import 'jira_fixture.dart' as jira;
import 'pull_fixture.dart';
import 'right_panes_fixture.dart';
import 'update_fixtures.dart';

// Where the newer release's card sits in every view and at every width a
// window has (ADR 0128): never over a control (a button, a field, a bar
// or a pane's header; a scrolling list's rows, as the transcript's, it may
// cover), wholly inside the window, and either all there or not shown:
// never half under a pane.

/// The card's answer to a ping, on [fake], and the ping that brings it.
Future<void> _card(WidgetTester tester, Daemon d, FakeDaemon fake) async {
  final before = fake.answer;
  fake.answer = (input) => switch (input) {
    PingRequest() => PingResponse(
      apiVersion: apiVersion,
      version: '0.1.16',
      update: newerUpdate('0.1.17'),
    ),
    _ => before?.call(input),
  };
  await d.ping();
  await tester.pumpAndSettle();
}

/// A diff long enough to fill the Changes view.
final List<FileChange> _longDiff = [
  for (final name in ['auth/redirect.go', 'auth/session.go'])
    FileChange(
      path: name,
      status: 'modified',
      oldText: [for (var i = 0; i < 200; i++) 'line $i'].join('\n'),
      newText: [
        for (var i = 0; i < 200; i++)
          if (i.isEven) 'line $i' else 'changed $i',
      ].join('\n'),
      added: 100,
      removed: 100,
    ),
];

Object? _changes(Object input) => switch (input) {
  GetFolderStatusRequest() => GetFolderStatusResponse(
    git: true,
    branch: 'fix-login',
    defaultBase: 'main',
  ),
  ListCommitsRequest() => ListCommitsResponse(),
  GetDiffRequest() => GetDiffResponse(files: _longDiff),
  _ => null,
};

/// What the card must never lie over, beside every shared control: the
/// bars at a view's foot and a pane's header.
const _bars = [Composer, StatusBar, CommitBox, RepliesWaiting, PaneHeader];

/// Whether [e] is in a list that scrolls up and down (a transcript's rows,
/// a diff's, a pull request's sections), which the card may cover.
bool _scrolls(Element e) {
  var found = false;
  e.visitAncestorElements((a) {
    if (a.widget case Scrollable(:final axisDirection)
        when axisDirectionToAxis(axisDirection) == Axis.vertical) {
      found = true;
    }
    return !found;
  });
  return found;
}

/// Whether what's hit at [p] is the card.
bool _cardAt(WidgetTester tester, Offset p) {
  final cards = updateToast.evaluate().map((e) => e.renderObject).toSet();
  return tester.hitTestOnBinding(p).path.any((h) => cards.contains(h.target));
}

/// The card [width] wide: shown whole or not at all (hidden only where
/// [hideable], under a pane over the transcript), inside the window, and
/// over no control nor a bar, which it would hide.
Future<void> _placed(
  WidgetTester tester,
  String name,
  double width, {
  bool hideable = false,
}) async {
  await render(tester, find.byType(NushiApp), name);
  expect(tester.takeException(), isNull);
  expect(updateToast, findsOneWidget);
  final card = tester.getRect(updateToast);
  final inner = card.deflate(8);
  final hits = [
    for (final p in [
      inner.topLeft,
      inner.topRight,
      inner.bottomLeft,
      inner.bottomRight,
      inner.center,
    ])
      _cardAt(tester, p),
  ];
  if (!hits.contains(true)) {
    expect(hideable, isTrue, reason: '$name: the card hidden');
    return;
  }
  expect(hits.contains(false), isFalse, reason: '$name: half hidden $hits');
  expect(card.left, greaterThanOrEqualTo(0));
  expect(card.right, lessThanOrEqualTo(width));
  expect(card.bottom, lessThanOrEqualTo(tester.view.physicalSize.height));
  final controls = [
    ...find.byType(Pressable).evaluate().where((e) => !_scrolls(e)),
    ...find.byType(EditableText).evaluate(),
    for (final t in _bars) ...find.byType(t).evaluate(),
  ];
  final mine = find
      .descendant(
        of: updateToast,
        matching: find.byWidgetPredicate((_) => true),
      )
      .evaluate()
      .toSet();
  for (final e in controls) {
    if (mine.contains(e)) continue;
    final box = e.renderObject;
    if (box is! RenderBox || !box.hasSize || !box.attached) continue;
    final r = box.localToGlobal(Offset.zero) & box.size;
    final o = card.intersect(r);
    if (o.width <= 0.5 || o.height <= 0.5) continue;
    // Over it only where the card is what's on top there.
    expect(
      _cardAt(tester, o.center),
      isFalse,
      reason: '$name: the card $card over ${e.widget.runtimeType} $r',
    );
  }
}

/// Opens the chat's [tab] (its header folds its tabs when narrow, so wide
/// first) and narrows the window to [width].
Future<void> _tab(WidgetTester tester, String tab, double width) async {
  await tester.tap(
    find.descendant(of: find.byType(ChatHeader), matching: find.text(tab)),
  );
  await tester.pumpAndSettle();
  viewport(tester, Size(width, 800));
  await tester.pumpAndSettle();
}

void main() {
  setUpAll(loadFonts);
  setUp(() {
    dismissedUpdate.value = null;
    brewCask = null;
  });

  for (final width in [400.0, 480.0, 600.0, 720.0, 900.0, 1280.0, 1700.0]) {
    final w = width.toInt();
    Future<(Daemon, FakeDaemon)> open(
      WidgetTester tester, {
      bool details = false,
      PanesOpen panes = noPanes,
      double at = 0,
      bool sidebar = true,
    }) async {
      final (d, fake) = await panesApp(
        tester,
        width: at > 0 ? at : width,
        height: 800,
        sidebar: sidebar,
        details: details,
        open: panes,
      );
      await _card(tester, d, fake);
      return (d, fake);
    }

    group('$w wide', () {
      testWidgets('the Chat tab', (tester) async {
        final (d, _) = await open(tester);
        await _placed(tester, 'chat_$w', width);
        await closeView(tester, d);
      }, variant: macOSOnly);

      testWidgets('the details rail', (tester) async {
        final (d, _) = await open(tester, details: true);
        await _placed(tester, 'details_$w', width);
        if (find.byType(DetailsPanel).hitTestable().evaluate().isNotEmpty) {
          final rail = tester.getRect(find.byType(DetailsPanel));
          expect(tester.getRect(updateToast).right, lessThan(rail.left));
        }
        await closeView(tester, d);
      }, variant: macOSOnly);

      for (final (pane, panes) in [
        ('files', (files: true, side: false, browser: 0)),
        ('side', (files: false, side: true, browser: 0)),
        ('browser', (files: false, side: false, browser: 1)),
      ]) {
        for (final sidebar in [true, false]) {
          final name = '${pane}_${sidebar ? '' : 'nosidebar_'}$w';
          testWidgets('the $pane pane, $name', (tester) async {
            if (panes.browser > 0) browserFakes();
            final (d, _) = await open(
              tester,
              sidebar: sidebar,
              panes: (
                side: panes.side,
                files: panes.files,
                subagent: false,
                command: false,
                previews: 0,
                browser: panes.browser,
              ),
            );
            await _placed(tester, name, width, hideable: true);
            // Never over the pane, which lies over the transcript or beside it.
            final over = find.byType(switch (pane) {
              'files' => FilesPane,
              'side' => SideChat,
              _ => BrowserPane,
            });
            final card = tester.getRect(updateToast);
            if (_cardAt(tester, card.center)) {
              for (final e in over.evaluate()) {
                final r = tester.getRect(
                  find.byElementPredicate((x) => x == e),
                );
                expect(card.overlaps(r), isFalse, reason: '$card over $r');
              }
            }
            await closeView(tester, d);
          }, variant: macOSOnly);
        }
      }

      testWidgets('the Changes tab, a long diff', (tester) async {
        final (d, fake) = await open(tester, at: 1700);
        final before = fake.answer;
        fake.answer = (input) => _changes(input) ?? before?.call(input);
        await _tab(tester, 'Changes', width);
        expect(
          find.textContaining('redirect.go', findRichText: true),
          findsWidgets,
        );
        await _placed(tester, 'changes_$w', width);
        await closeView(tester, d);
      }, variant: macOSOnly);

      // At 400 a to-do row's buttons overflow, the card aside.
      testWidgets('the Pull request tab, cards waiting', skip: width < 480, (
        tester,
      ) async {
        final (d, fake) = await openPullTab(
          tester,
          fixturePull(),
          size: const Size(1700, 800),
          open: false,
        );
        await _card(tester, d, fake);
        d.chats['c']!.draft.replies.value = [
          (quote: 'CI / rubocop', text: 'Fix it', from: 'checks'),
        ];
        await _tab(tester, 'Pull request', width);
        expect(find.byType(RepliesWaiting), findsOneWidget);
        await _placed(tester, 'pull_$w', width);
        d.chats['c']!.draft.replies.value = const [];
        await closeView(tester, d);
      }, variant: macOSOnly);

      for (final (view, go) in <(String, void Function(Daemon))>[
        (
          'nothing',
          (d) => d
            ..selectedId = null
            ..listChanged(),
        ),
        (
          'trust',
          (d) => d
            ..trustAsk = TrustAsk(
              '/Users/me/site',
              'Start Claude',
              const [],
              'claude',
            )
            ..listChanged(),
        ),
      ]) {
        testWidgets(view, (tester) async {
          final (d, _) = await open(tester);
          go(d);
          await tester.pumpAndSettle();
          await _placed(tester, '${view}_$w', width);
          await closeView(tester, d);
        }, variant: macOSOnly);
      }

      // Their lists, then an item open. Narrower than 600 their headers
      // overflow, the card aside.
      for (final (view, opener, row) in [
        ('jira', jira.openView, 'Log in with a passkey'),
        ('github', gh.openView, 'Add Sub and Neg'),
      ]) {
        testWidgets(view, skip: width < 600, (tester) async {
          final (fake, d) = await opener(
            tester,
            window: true,
            size: Size(width, 800),
          );
          await _card(tester, d, fake);
          await _placed(tester, '${view}_list_$w', width);
          await tester.tap(find.text(row).first);
          await tester.pumpAndSettle();
          await _placed(tester, '${view}_item_$w', width);
          await closeView(tester, d);
        }, variant: macOSOnly);
      }
    });
  }
}
