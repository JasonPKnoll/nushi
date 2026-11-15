import 'dart:math';

import 'package:flutter_test/flutter_test.dart';
import 'package:nushi/beside_fit.dart';
import 'package:nushi/chat.dart';
import 'package:nushi/chat_panes.dart';
import 'package:nushi/chat_view.dart';
import 'package:nushi/details_panel.dart';
import 'package:nushi/preview_entry.dart';
import 'package:nushi/settings.dart';

import 'fake_daemon.dart';
import 'harness.dart';

import 'right_panes_fixture.dart';

void main() {
  BesideFit fit(
    double room,
    List<PaneSpec> panes, {
    bool wantRail = true,
    Map<String, double> saved = const {},
    Map<String, double> live = const {},
  }) => chatFit(
    room: room,
    panes: panes,
    saved: saved,
    live: live,
    wantRail: wantRail,
  );

  test('the thresholds, worked out from the constants, are the table', () {
    // A changed constant fails here loudly, not as a geometry test far off.
    expect(DetailsPanel.width, 320);
    expect(ChatView.leastBesideWide, 360);
    expect(ChatView.leastBesideDetails, 484);
    expect(ChatView.minWidth, 556);
    for (final row in thresholdRows) {
      expect(railFrom(row.panes), row.rail, reason: row.name);
      expect(besideFrom(row.panes), row.beside, reason: row.name);
    }
  });

  test('each row of the table holds at its line and half a point either '
      'side', () {
    for (final row in thresholdRows) {
      final panes = row.panes;
      final sum = panes.fold(0.0, (w, p) => w + p.initial);
      for (final d in [-0.5, 0.0, 0.5]) {
        final a = row.rail + d;
        final f = fit(a, panes);
        expect(f.rail, d >= 0, reason: '${row.name} at $a');
        expect(f.over, false, reason: '${row.name} at $a');
        expect(f.left, a - sum, reason: '${row.name} at $a');
        expect(f.widths, {for (final p in panes) p.name: p.initial});
        if (row.beside case final b?) {
          final g = fit(b + d, panes);
          expect(g.over, d < 0, reason: '${row.name} at ${b + d}');
          expect(g.rail, false, reason: '${row.name} at ${b + d}');
          expect(g.left, d < 0 ? b + d : b + d - sum);
        }
      }
    }
  });

  test('swept across every width, the rail and over each flip once and are '
      'never both on', () {
    for (final row in thresholdRows) {
      for (final want in [true, false]) {
        bool? rail;
        bool? over;
        var railFlips = 0;
        var overFlips = 0;
        for (var room = 300.0; room <= 2600; room += 0.5) {
          final f = fit(room, row.panes, wantRail: want);
          expect(f.rail && f.over, false, reason: '${row.name} at $room');
          if (!f.over && row.panes.isNotEmpty) {
            final least = row.panes.any(_wide)
                ? ChatView.leastBesideWide
                : ChatView.minWidth;
            expect(f.left, greaterThanOrEqualTo(least));
          }
          if (rail != null && rail != f.rail) {
            railFlips++;
            expect(f.rail, true, reason: 'the rail only comes as it widens');
          }
          if (over != null && over != f.over) {
            overFlips++;
            expect(f.over, false, reason: 'the panes only go beside');
          }
          rail = f.rail;
          over = f.over;
        }
        expect(railFlips, want ? 1 : 0, reason: row.name);
        expect(overFlips, row.panes.isEmpty ? 0 : 1, reason: row.name);
      }
    }
  });

  test('a drag, from the least to the most, never moves the panes over or '
      "off, and its width is the edge's own clamp", () {
    for (final row in thresholdRows.where((r) => r.panes.isNotEmpty)) {
      for (final room in [row.beside! + 200, row.beside! - 100]) {
        final rest = fit(room, row.panes);
        for (final p in row.panes) {
          final most = rest.most[p.name]!;
          final top = min(most, 2600);
          for (var w = p.min - 20; w <= top + 20; w += 1) {
            final f = fit(room, row.panes, live: {p.name: w});
            expect(f.over, rest.over, reason: '${row.name} ${p.name} $w');
            // Resizable._width: what it's asked, within its least and the
            // most it was given, which the drag itself doesn't change.
            expect(f.most[p.name], most);
            expect(f.widths[p.name], w.clamp(p.min, max(p.min, most)));
            if (!f.over) {
              final least = row.panes.any(_wide)
                  ? ChatView.leastBesideWide
                  : ChatView.minWidth;
              expect(f.left, greaterThanOrEqualTo(least));
            }
          }
        }
      }
    }
  });

  test('the rail is an input to nothing', () {
    for (final row in thresholdRows) {
      for (var room = 600.0; room <= 2200; room += 7) {
        final live = {
          for (final p in row.panes.take(1)) p.name: p.initial + 40,
        };
        final on = fit(room, row.panes, live: live);
        final off = fit(room, row.panes, wantRail: false, live: live);
        expect(on.widths, off.widths);
        expect(on.most, off.most);
        expect(on.over, off.over);
        expect(on.left, off.left);
      }
    }
  });

  test("saved widths past a pane's least or most are clamped, and one "
      'wider than the room puts the panes over', () {
    const side = ChatPanes.side;
    expect(fit(2000, [side], saved: {'side': 900}).widths['side'], 640);
    expect(fit(2000, [side], saved: {'side': 10}).widths['side'], 300);
    // A relaunch in a smaller window than the width saved.
    final small = fit(700, [ChatPanes.preview], saved: {'preview': 1200});
    expect(small.over, true);
    expect(small.widths['preview'], 700 - 48);
    // No room even for its least: it keeps its least, over the view.
    final none = fit(320, [ChatPanes.preview]);
    expect(none.over, true);
    expect(none.most['preview'], 320 - 48);
    expect(none.widths['preview'], 300);
    final tiny = fit(200, [ChatPanes.files, ChatPanes.preview]);
    expect(tiny.over, true);
    expect(tiny.most['preview'], lessThan(300));
    expect(tiny.widths['preview'], 300);
  });

  test("it gives the same answer whatever was asked before, in the panes' "
      'order', () {
    final asks = [
      for (final row in thresholdRows)
        for (final room in const [700.0, 1100.0, 1500.0, 1900.0]) (row, room),
    ];
    final first = [for (final (row, room) in asks) fit(room, row.panes)];
    final order = [for (var i = 0; i < asks.length; i++) i]..shuffle(Random(7));
    for (final i in order) {
      final (row, room) = asks[i];
      final again = fit(room, row.panes);
      expect(again.over, first[i].over);
      expect(again.rail, first[i].rail);
      expect(again.left, first[i].left);
      expect(again.widths, first[i].widths);
      expect(again.most, first[i].most);
      expect(again.widths.keys, [for (final p in row.panes) p.name]);
    }
  });

  testWidgets('a hidden or archived side chat takes no room, and a '
      "subagent's pane takes the preview's place", (tester) async {
    final d = await FakeDaemon([chat('c', 'claude', 'Fix it')]).connect(tester);
    final c = d.chats['c']!;
    List<String> names({bool? preview}) => [
      for (final p in openPanes(c, d, preview: preview)) p.name,
    ];
    expect(names(), isEmpty);
    c.side.value = ChatState('s', '/Users/me');
    filesChats.value = {'c'};
    c.preview.value = const PreviewFile('/Users/me/a.md');
    expect(names(), ['side', 'files', 'preview']);
    c.sideHidden.value = true;
    expect(names(), ['files', 'preview']);
    c
      ..sideHidden.value = false
      ..archived = true;
    expect(names(), ['files', 'preview']);
    c.archived = false;
    c.pane.value = 'a';
    expect(names(), ['side', 'files', 'subagent']);
    // The room a preview the agent asks for would take: the subagent's
    // pane gives it its place.
    expect(names(preview: true), ['side', 'files', 'preview']);
    c.pane.value = null;
    c.preview.value = null;
    expect(names(), ['side', 'files']);
    expect(names(preview: true), ['side', 'files', 'preview']);
    await closeView(tester, d);
  });

  test('working it out writes no setting', () {
    var told = 0;
    void count() => told++;
    paneWidths.addListener(count);
    showDetails.addListener(count);
    addTearDown(() {
      paneWidths.removeListener(count);
      showDetails.removeListener(count);
    });
    for (final row in thresholdRows) {
      for (var room = 300.0; room <= 2600; room += 50) {
        fit(room, row.panes, saved: paneWidths.value);
      }
    }
    expect(told, 0);
  });
}

bool _wide(PaneSpec p) => p == ChatPanes.subagent || p == ChatPanes.preview;
