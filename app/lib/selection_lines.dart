import 'package:flutter/rendering.dart';

final _marker = RegExp(r'^\s*[^\p{L}\s]{1,3}\s*$', unicode: true);

/// The gap that makes two lines separate paragraphs: more than a table's
/// rows have.
const paragraphGap = 9.0;

/// The text selected under [root], with its lines. Flutter joins the texts
/// of a selection with nothing between them, so a copy of two paragraphs ran
/// them together. Here what's lower starts a new line, after a blank one
/// where a gap says it's a new paragraph (a list's items have the same gap,
/// and take none), and what's beside goes after a tab (a table's cell) or a
/// space (a list's marker). What's selected is left to the selection: this
/// only reads it, on a copy or a reply.
String selectionLines(RenderObject root) {
  final text = StringBuffer();
  Rect? last;
  var inList = false; // the line starts with a list's marker
  var afterMarker = false; // the last text was one
  void visit(RenderObject object) {
    if (object is! RenderParagraph) return object.visitChildren(visit);
    final part = _selectedIn(object);
    if (part.isEmpty) return;
    final rect = object.localToGlobal(Offset.zero) & object.size;
    final marker = _marker.hasMatch(part);
    final gap = last == null ? 0.0 : rect.top - last!.bottom;
    if (last == null) {
      inList = marker;
    } else if (gap > -rect.height / 2) {
      final item = marker && inList;
      text.write(gap >= paragraphGap && !item ? '\n\n' : '\n');
      inList = marker;
    } else {
      text.write(afterMarker ? ' ' : '\t');
    }
    text.write(part);
    last = rect;
    afterMarker = marker;
  }

  visit(root);
  return text.toString();
}

/// What's selected of [paragraph], each widget span's own selected text in
/// its place. Flutter cuts a paragraph's selection at its widget spans, and
/// a span's text (a file's link) is a paragraph of its own under this one:
/// read one by one, a link's name went after its line.
String _selectedIn(RenderParagraph paragraph) {
  final all = paragraph.text.toPlainText(includeSemanticsLabels: false);
  // The only way to a paragraph's selection from outside it.
  // ignore: invalid_use_of_visible_for_testing_member
  final ranges = paragraph.selections
      .where((s) => !s.isCollapsed && s.end <= all.length)
      .toList();
  final spans = <RenderObject>[];
  paragraph.visitChildren(spans.add);
  final out = StringBuffer();
  var span = 0;
  for (var i = 0; i < all.length; i++) {
    if (all.codeUnitAt(i) == _placeholder && span < spans.length) {
      out.write(_selectedUnder(spans[span++]));
    } else if (ranges.any((r) => r.start <= i && i < r.end)) {
      out.writeCharCode(all.codeUnitAt(i));
    }
  }
  return out.toString();
}

/// The selected text of the paragraphs under [object], run together: a
/// widget span's, which sits within a line.
String _selectedUnder(RenderObject object) {
  if (object is RenderParagraph) return _selectedIn(object);
  final out = StringBuffer();
  object.visitChildren((child) => out.write(_selectedUnder(child)));
  return out.toString();
}

/// Where a widget span sits in a paragraph's plain text.
const _placeholder = 0xFFFC;
