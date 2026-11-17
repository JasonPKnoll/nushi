import 'dart:math';

// How the Files pane shares its width between its viewer and its tree
// (ADR 0123): pure, so its edges are checked point by point.

/// The least the Files pane's tree takes beside its viewer.
const double filesTreeLeast = 200;

/// The width the tree opens at until dragged.
const double filesTreeInitial = 260;

/// The most the tree may be dragged to.
const double filesTreeMost = 480;

/// The least the viewer keeps beside the tree.
const double filesViewerLeast = 300;

/// The least the tree's search field and rows hold: squeezed narrower, it
/// isn't there at all ([treeWidth]).
const double filesTreeFolds = 120;

/// The tree's width in a Files pane [w] wide, [saved] the width it was
/// dragged to: the viewer gives first, down to [filesViewerLeast]; then
/// the tree, down to [filesTreeLeast]. Under both (the pane squeezed over
/// the transcript in a narrow window) the tree gives on, the viewer
/// keeping its least, until it would be narrower than [filesTreeFolds]:
/// then it's gone and the viewer has the whole pane, so a file in it
/// reads. Point by point but at that one line, where the viewer takes the
/// tree's last 120 at once rather than leave a sliver its field and rows
/// would run past; the same with a file open or none, so opening one never
/// moves the tree.
double treeWidth(double w, double saved) {
  const least = filesTreeLeast + filesViewerLeast;
  if (w < least) {
    final tree = w - filesViewerLeast;
    return tree < filesTreeFolds ? 0 : tree;
  }
  return saved.clamp(filesTreeLeast, treeMost(w));
}

/// The most the tree may be dragged to in a pane [w] wide: the viewer
/// keeps its least.
double treeMost(double w) =>
    max(filesTreeLeast, min(filesTreeMost, w - filesViewerLeast));
