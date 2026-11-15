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

/// The tree's width in a Files pane [w] wide, [saved] the width it was
/// dragged to: the viewer gives first, down to [filesViewerLeast]; then
/// the tree, down to [filesTreeLeast]. Under both (the pane squeezed in a
/// narrow window) the two narrow alike, so no edge jumps.
double treeWidth(double w, double saved) {
  const least = filesTreeLeast + filesViewerLeast;
  if (w < least) return w * filesTreeLeast / least;
  return saved.clamp(filesTreeLeast, treeMost(w));
}

/// The most the tree may be dragged to in a pane [w] wide: the viewer
/// keeps its least.
double treeMost(double w) =>
    max(filesTreeLeast, min(filesTreeMost, w - filesViewerLeast));
