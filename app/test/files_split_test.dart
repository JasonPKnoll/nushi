import 'package:flutter_test/flutter_test.dart';
import 'package:nushi/files_split.dart';

// How the Files pane shares its width between its viewer and its tree
// (ADR 0123): the viewer gives first, then the tree, and under both they
// narrow alike, so no edge ever jumps.

void main() {
  test('the tree at its saved width, the viewer giving first', () {
    for (final (w, tree) in [
      (1400.0, 260.0),
      (820.0, 260.0),
      (560.0, 260.0),
      (520.0, 220.0),
      (500.0, 200.0),
      (400.0, 160.0),
      (100.0, 40.0),
    ]) {
      expect(treeWidth(w, 260), tree, reason: '$w');
    }
    // Dragged to its most, the viewer keeps its least.
    expect(treeWidth(1400, 480), 480);
    expect(treeWidth(700, 480), 400);
    // At its least, it stays there while the pane has room for both.
    for (var w = 500.0; w <= 1400; w += 50) {
      expect(treeWidth(w, 200), 200, reason: '$w');
    }
  });

  test('the viewer keeps its least while there is room for it; under '
      'that the two share alike', () {
    for (var w = 500.0; w <= 1400; w += 1) {
      for (final saved in [200.0, 260.0, 480.0]) {
        expect(w - treeWidth(w, saved), greaterThanOrEqualTo(300));
      }
    }
    for (var w = 10.0; w < 500; w += 1) {
      expect(treeWidth(w, 260) / w, closeTo(0.4, 1e-9), reason: '$w');
    }
  });

  test('a point at a time, from 100 to 1400, no edge moves more than a '
      'point, the tree on either side, shown or hidden', () {
    for (final saved in [200.0, 260.0, 480.0]) {
      for (final right in [true, false]) {
        for (final shown in [true, false]) {
          // The pane's left edge at 0: its edges, the viewer's and the
          // tree's, left to right.
          List<double> edges(double w) {
            final tree = shown ? treeWidth(w, saved) : 0.0;
            final viewer = w - tree;
            return right ? [0, viewer, w] : [0, tree, w];
          }

          var last = edges(100);
          for (var w = 100.5; w <= 1400; w += 0.5) {
            final now = edges(w);
            for (var i = 0; i < now.length; i++) {
              expect(
                (now[i] - last[i]).abs(),
                lessThanOrEqualTo(1),
                reason: 'saved $saved, right $right, shown $shown, at $w',
              );
            }
            if (!shown) expect(now[1], right ? w : 0);
            last = now;
          }
        }
      }
    }
  });

  test('no jump at the seam where the two start to narrow alike', () {
    for (final saved in [200.0, 260.0, 480.0]) {
      final at = [
        for (final w in [499.5, 500.0, 500.5]) treeWidth(w, saved),
      ];
      expect((at[1] - at[0]).abs(), lessThanOrEqualTo(0.5));
      expect((at[2] - at[1]).abs(), lessThanOrEqualTo(0.5));
    }
  });

  test("the most a drag reaches: the viewer's least kept, never under the "
      "tree's least", () {
    expect(treeMost(1400), 480);
    expect(treeMost(700), 400);
    expect(treeMost(500), 200);
    expect(treeMost(300), 200);
  });
}
