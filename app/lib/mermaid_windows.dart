import 'dart:ui';

import 'package:nushi/mermaid.dart';

/// Windows draws no diagrams yet: there's no web view to run mermaid.js in
/// (ADR 0124). A diagram shows its source and says so.
class WindowsDiagrams implements Diagrams {
  const new();

  @override
  Future<DrawnDiagram> draw(
    String source, {
    required bool dark,
    required double width,
    required Color background,
    required Color foreground,
  }) async => const DrawnDiagram(error: 'Diagrams aren’t drawn on Windows yet');
}
