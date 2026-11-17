import 'package:flutter_test/flutter_test.dart';
import 'package:nushi/files_split.dart';

// How the Files pane shares its width between its viewer and its tree
// (ADR 0123): the viewer gives first, then the tree, and under both the
// tree gives on, no edge jumping, until it would be too narrow for its
// rows: then it goes at once.

void main() {
  test('the tree at its saved width, the viewer giving first', () {
    for (final (w, tree) in [
      (1400.0, 260.0),
      (820.0, 260.0),
      (560.0, 260.0),
      (520.0, 220.0),
      (500.0, 200.0),
      (440.0, 140.0),
      (420.0, 120.0),
      (419.0, 0.0),
      (400.0, 0.0),
      (300.0, 0.0),
      (100.0, 0.0),
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
      'that it has the whole pane', () {
    for (var w = 300.0; w <= 1400; w += 1) {
      for (final saved in [200.0, 260.0, 480.0]) {
        expect(w - treeWidth(w, saved), greaterThanOrEqualTo(300));
      }
    }
    // Narrowed alike, a 720 window's squeezed Files had a viewer of 83, its
    // file's name breaking a letter or two a line.
    for (var w = 10.0; w <= 300; w += 1) {
      expect(treeWidth(w, 260), 0, reason: '$w');
    }
  });

  test('a point at a time, from 100 to 1400, no edge moves more than a '
      'point but where the tree goes, the tree on either side, shown or '
      'hidden', () {
    // Where the tree would be narrower than its rows hold.
    const folds = filesViewerLeast + filesTreeFolds;
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
              if (shown && w == folds) {
                expect((now[1] - last[1]).abs(), closeTo(filesTreeFolds, 0.5));
                continue;
              }
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

  test('no jump at the seam where the tree gives on', () {
    for (final saved in [200.0, 260.0, 480.0]) {
      for (final seam in [500.0]) {
        final at = [
          for (final w in [seam - 0.5, seam, seam + 0.5]) treeWidth(w, saved),
        ];
        expect((at[1] - at[0]).abs(), lessThanOrEqualTo(0.5));
        expect((at[2] - at[1]).abs(), lessThanOrEqualTo(0.5));
      }
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
