import 'dart:math';

import 'package:flutter/widgets.dart' show SizedBox;
import 'package:flutter_test/flutter_test.dart';
import 'package:nushi/beside.dart' show SidePane;
import 'package:nushi/beside_fit.dart';
import 'package:nushi/chat.dart';
import 'package:nushi/chat_panes.dart';
import 'package:nushi/chat_view.dart';
import 'package:nushi/details_panel.dart';
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

  test('the panes dragged as wide as wanted say so on their spec', () {
    expect(ChatPanes.subagent.wide, isTrue);
    expect(ChatPanes.browser.wide, isTrue);
    expect(ChatPanes.files.wide, isTrue);
    expect(ChatPanes.filesNoTree.wide, isTrue);
    expect(ChatPanes.side.wide, isFalse);
    // Files with its tree or without shares one width: the tree hidden
    // leaves the pane where it was.
    expect(ChatPanes.filesNoTree.name, ChatPanes.files.name);
    expect(
      (ChatPanes.files.initial, ChatPanes.files.min, ChatPanes.files.cap),
      (820, 500, double.infinity),
    );
    expect(
      (ChatPanes.filesNoTree.initial, ChatPanes.filesNoTree.min),
      (820, 300),
    );
    expect((ChatPanes.browser.initial, ChatPanes.browser.min), (560, 300));
    // A SidePane made of a spec carries it, which the chat area reads.
    final pane = SidePane.of(ChatPanes.subagent, child: const SizedBox());
    expect(pane.wide, isTrue);
  });

  test('the thresholds, worked out from the constants, are the table', () {
    // A changed constant fails here loudly, not as a geometry test far off.
    expect(DetailsPanel.width, 320);
    expect(ChatView.leastBesideWide, 360);
    expect(ChatView.leastBesideDetails, 484);
    expect(ChatView.minWidth, 556);
    for (final row in [...thresholdRows, filesNoTreeRow]) {
      expect(railFrom(row.panes), row.rail, reason: row.name);
      expect(narrowFrom(row.panes), row.narrow, reason: row.name);
      expect(besideFrom(row.panes), row.beside, reason: row.name);
    }
  });

  test('each row of the table holds at its lines and half a point either '
      'side', () {
    for (final row in thresholdRows) {
      final panes = row.panes;
      final sum = panes.fold(0.0, (w, p) => w + p.initial);
      final least = leastBeside(panes);
      for (final d in [-0.5, 0.0, 0.5]) {
        final a = row.rail + d;
        final f = fit(a, panes);
        expect(f.rail, d >= 0, reason: '${row.name} at $a');
        expect(f.over, false, reason: '${row.name} at $a');
        expect(f.left, a - sum, reason: '${row.name} at $a');
        expect(f.widths, {for (final p in panes) p.name: p.initial});
        if (row.narrow case final n?) {
          // At their widths down to the line; under it, narrowed, the
          // transcript at its least.
          final g = fit(n + d, panes);
          expect(g.over, false, reason: '${row.name} at ${n + d}');
          expect(g.rail, false, reason: '${row.name} at ${n + d}');
          expect(g.left, d < 0 ? least : n + d - sum);
          if (d >= 0) {
            expect(g.widths, {for (final p in panes) p.name: p.initial});
          }
        }
        if (row.beside case final b?) {
          // At their least down to the line; under it, over the transcript,
          // which keeps its least under them.
          final g = fit(b + d, panes);
          expect(g.over, d < 0, reason: '${row.name} at ${b + d}');
          expect(g.rail, false, reason: '${row.name} at ${b + d}');
          expect(g.left, least, reason: '${row.name} at ${b + d}');
          final used = g.widths.values.fold(0.0, (w, x) => w + x);
          if (d <= 0) {
            expect(g.widths, {for (final p in panes) p.name: p.min});
          } else {
            expect(used, b + d - least);
          }
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
          if (row.panes.isNotEmpty) {
            final least = leastBeside(row.panes);
            if (f.over) {
              expect(f.left, min(least, room));
            } else {
              expect(f.left, greaterThanOrEqualTo(least));
            }
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

  // Widths saved as dragged: each pane at its most, its least, and between.
  final savedSets = <String, Map<String, double>>{
    'default': const {},
    'wide': const {
      'side': 600,
      'filesPane': 2000,
      'subagent': 900,
      'browser': 1000,
    },
    'least': const {
      'side': 300,
      'filesPane': 500,
      'subagent': 220,
      'browser': 300,
    },
    'mixed': const {
      'side': 320,
      'filesPane': 820,
      'subagent': 240,
      'browser': 700,
    },
  };

  test("stepped a point at a time across every line, the panes' left edge "
      "and the header's width never jump", () {
    for (final row in thresholdRows.where((r) => r.panes.isNotEmpty)) {
      for (final MapEntry(key: name, value: saved) in savedSets.entries) {
        for (final want in [true, false]) {
          double? edge;
          double? header;
          for (var room = 200.0; room <= 2600; room += 1) {
            final f = fit(room, row.panes, saved: saved, wantRail: want);
            final used = f.widths.values.fold(0.0, (w, x) => w + x);
            // Where the panes start; in a room under their least widths
            // they're narrowed alike from its left edge (Beside).
            final e = max(0.0, room - used);
            // The header spans the transcript and the rail, beside the
            // panes or under them: shown or tucked, it's [left].
            final h = f.left;
            final why = '${row.name}, $name, rail $want, at $room';
            if (edge != null) {
              expect(
                (e - edge).abs(),
                lessThanOrEqualTo(1 + 1e-9),
                reason: why,
              );
              expect(
                (h - header!).abs(),
                lessThanOrEqualTo(1 + 1e-9),
                reason: why,
              );
            }
            edge = e;
            header = h;
          }
        }
      }
    }
  });

  test('the widest gives first, down to its least, then the next, the '
      'transcript keeping its least', () {
    const files = ChatPanes.files;
    const browser = ChatPanes.browser;
    // Files alone: its 820 and the transcript's 360 make 1180.
    Map<String, double> one(double room) => fit(room, [files]).widths;
    expect(one(1180), {'filesPane': 820});
    expect(one(1000), {'filesPane': 640});
    expect(one(920), {'filesPane': 560});
    expect(one(860), {'filesPane': 500});
    expect(fit(859, [files]).over, isTrue);
    // Files 820 and the browser 560 want 1740: Files, the wider, gives
    // first, down to its 500, then the browser.
    Map<String, double> two(double room) => fit(room, [files, browser]).widths;
    expect(two(1740), {'filesPane': 820, 'browser': 560});
    expect(two(1600), {'filesPane': 680, 'browser': 560});
    expect(two(1420), {'filesPane': 500, 'browser': 560});
    expect(two(1300), {'filesPane': 500, 'browser': 440});
    expect(two(1160), {'filesPane': 500, 'browser': 300});
    expect(fit(1159, [files, browser]).over, isTrue);
    for (final room in [1600.0, 1420.0, 1300.0, 1160.0]) {
      expect(fit(room, [files, browser]).left, 360);
      expect(fit(room, [files, browser]).over, false);
    }
    // Widest by width, not by kind: the browser dragged wider than Files
    // gives first.
    final dragged = fit(1700, [files, browser], saved: {'browser': 1000});
    expect(dragged.widths, {'filesPane': 820, 'browser': 520});
    // Files and a subagent's pane: Files gives first, to 500, then the
    // subagent's to 220.
    const sub = ChatPanes.subagent;
    Map<String, double> fs(double room) => fit(room, [files, sub]).widths;
    expect(fs(1444), {'filesPane': 820, 'subagent': 264});
    expect(fs(1200), {'filesPane': 576, 'subagent': 264});
    expect(fs(1124), {'filesPane': 500, 'subagent': 264});
    expect(fs(1080), {'filesPane': 500, 'subagent': 220});
    // The side chat, Files and the browser at 1800: Files gives first; the
    // side chat and the browser keep theirs until it's at its least.
    const side = ChatPanes.side;
    expect(fit(1800, [side, files, browser]).widths, {
      'side': 380,
      'filesPane': 500,
      'browser': 560,
    });
  });

  test('narrowing never touches the saved widths, and they come back as '
      'soon as there is room', () {
    final saved = {'filesPane': 900.0, 'browser': 800.0};
    final before = {...saved};
    const panes = [ChatPanes.files, ChatPanes.browser];
    for (var room = 2400.0; room >= 700; room -= 10) {
      fit(room, panes, saved: saved);
    }
    expect(saved, before);
    // 900 + 800 and the transcript's 360: back at 2060 and over.
    expect(fit(2060, panes, saved: saved).widths, saved);
    expect(fit(2300, panes, saved: saved).widths, saved);
    expect(fit(2059, panes, saved: saved).widths, {
      'filesPane': 899,
      'browser': 800,
    });
  });

  test('a drag, from the least to the most, never moves the panes over or '
      "off, and its width is the edge's own clamp", () {
    for (final row in thresholdRows.where((r) => r.panes.isNotEmpty)) {
      for (final room in [
        row.narrow! + 200,
        (row.narrow! + row.beside!) / 2,
        row.beside! - 100,
      ]) {
        final rest = fit(room, row.panes);
        for (final p in row.panes) {
          final most = rest.most[p.name]!;
          final top = min(most, 2600);
          for (var w = p.min - 20; w <= top + 20; w += 1) {
            final f = fit(room, row.panes, live: {p.name: w});
            final why = '${row.name} ${p.name} $w at $room';
            expect(f.over, rest.over, reason: why);
            // Resizable._width: what it's asked, within its least and the
            // most it was given, which the drag itself doesn't change.
            expect(f.most[p.name], most, reason: why);
            expect(f.widths[p.name], w.clamp(p.min, max(p.min, most)));
            // The rest are never wider than they'd be at rest's widths
            // and their own saved ones.
            for (final q in row.panes.where((q) => q != p)) {
              expect(
                f.widths[q.name],
                lessThanOrEqualTo(q.initial),
                reason: why,
              );
              expect(
                f.widths[q.name],
                greaterThanOrEqualTo(rest.widths[q.name]!),
                reason: why,
              );
            }
            if (!f.over) {
              expect(f.left, greaterThanOrEqualTo(leastBeside(row.panes)));
            }
          }
        }
      }
    }
  });

  test('narrowed, a pane dragged narrower gives the rest their width back, '
      'and goes no wider than it is', () {
    const panes = [ChatPanes.files, ChatPanes.browser];
    // 1400: Files narrowed to 500, the browser to 540.
    final rest = fit(1400, panes);
    expect(rest.widths, {'filesPane': 500, 'browser': 540});
    expect(rest.most, {'filesPane': 500, 'browser': 540});
    final f = fit(1400, panes, live: {'browser': 400});
    expect(f.widths, {'filesPane': 640, 'browser': 400});
    expect(f.left, 360);
    expect(fit(1400, panes, live: {'browser': 600}).widths['browser'], 540);
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
      'wider than the room is narrowed, not put over', () {
    const side = ChatPanes.side;
    expect(fit(2000, [side], saved: {'side': 900}).widths['side'], 640);
    expect(fit(2000, [side], saved: {'side': 10}).widths['side'], 300);
    // A relaunch in a smaller window than the width saved.
    final small = fit(700, [ChatPanes.browser], saved: {'browser': 1200});
    expect(small.over, false);
    expect(small.widths['browser'], 700 - 360);
    // No room even for its least: it keeps its least, over the view, its
    // edge stopping there.
    final none = fit(320, [ChatPanes.browser]);
    expect(none.over, true);
    expect(none.most['browser'], 300);
    expect(none.widths['browser'], 300);
    expect(none.left, 320);
    final tiny = fit(200, [ChatPanes.files, ChatPanes.browser]);
    expect(tiny.over, true);
    expect(tiny.widths, {'filesPane': 500, 'browser': 300});
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

  testWidgets('a hidden or archived side chat takes no room, a '
      "subagent's pane sits beside Files and takes the browser's place", (
    tester,
  ) async {
    final d = await FakeDaemon([chat('c', 'claude', 'Fix it')]).connect(tester);
    final c = d.chats['c']!;
    List<String> names({bool files = false, bool browser = false}) => [
      for (final p in openPanes(c, d, files: files, browser: browser)) p.name,
    ];
    expect(names(), isEmpty);
    // The room a file the agent asks for would take: the Files pane's.
    expect(names(files: true), ['filesPane']);
    c.side.value = ChatState('s', '/Users/me');
    c.openInFiles('/Users/me/a.md');
    expect(names(), ['side', 'filesPane']);
    c.sideHidden.value = true;
    expect(names(), ['filesPane']);
    c
      ..sideHidden.value = false
      ..archived = true;
    expect(names(), ['filesPane']);
    c.archived = false;
    c.pane.value = 'a';
    expect(names(), ['side', 'filesPane', 'subagent']);
    c.openBrowserTab('http://localhost:3000');
    // The browser took the subagent's place.
    expect(c.pane.value, isNull);
    expect(names(), ['side', 'filesPane', 'browser']);
    c.pane.value = 'a';
    expect(c.browser.shown.value, isNull);
    // The room a page the agent asks for would take: the subagent's pane
    // gives it its place.
    expect(names(browser: true), ['side', 'filesPane', 'browser']);
    // The Files pane hidden keeps its tabs, and takes no room.
    filesChats.value = {};
    expect(c.shownFile, '/Users/me/a.md');
    expect(names(), ['side', 'subagent']);
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
