import 'dart:async';
import 'dart:convert';

import 'package:diff_view/diff_view.dart';
import 'package:flutter/foundation.dart';

import 'package:nushi/chat.dart';
import 'package:nushi/daemon.dart';
import 'package:nushi/diff_box.dart' show maxDiffLines, offThread;
import 'package:nushi/gen/nushi/v1/nushi.pb.dart';
import 'package:nushi/preview.dart' show FileWatch;

// A file preview's change marks (ADR 0131): which of its lines differ from
// the last commit, worked out in the app from HEAD's text (GetDiff with a
// path) and the text it shows, so they follow unsaved edits too.

/// What a change mark says of its lines.
enum MarkKind { added, changed, removed }

/// A run of lines that differ from the last commit: [count] lines from
/// [first] (from 0) of the text shown, added or changed; or, removed,
/// none, the lines having stood before line [first]. [old] is what the
/// last commit had there (none for added lines).
@immutable
class LineMark {
  const new(this.kind, this.first, this.count, [this.old = const []]);

  final MarkKind kind;
  final int first;
  final int count;
  final List<String> old;

  @override
  bool operator ==(Object other) =>
      other is LineMark &&
      other.kind == kind &&
      other.first == first &&
      other.count == count &&
      listEquals(other.old, old);

  @override
  int get hashCode => Object.hash(kind, first, count, Object.hashAll(old));

  @override
  String toString() => 'LineMark(${kind.name} $first+$count $old)';
}

String _noBom(String s) => s.startsWith('﻿') ? s.substring(1) : s;

/// [text]'s lines as `TextDiff` splits them: a last line break ends the
/// last line, and a line's CR is not its text.
List<String> _lines(String text) {
  final lines = text.split('\n');
  if (lines.last.isEmpty) lines.removeLast();
  String noCr(String l) => l.endsWith('\r') ? l.substring(0, l.length - 1) : l;
  return [for (final l in lines) noCr(l)];
}

/// The marks of [shown] against [base], the last commit's text: none when
/// they're the same, line endings and a leading BOM aside, or either has
/// more than [maxDiffLines] lines (as a diff isn't drawn then).
List<LineMark> lineMarks(String base, String shown) {
  final old = _noBom(base);
  final now = _noBom(shown);
  if (old == now) return const [];
  final oldLines = _lines(old);
  if (oldLines.length > maxDiffLines || _lines(now).length > maxDiffLines) {
    return const [];
  }
  final blocks = TextDiff.compute(old, now).document.blocks;
  return [
    for (final b in blocks)
      switch (b.kind) {
        BlockKind.inserted => LineMark(
          MarkKind.added,
          b.neu.start,
          b.neu.end - b.neu.start,
        ),
        BlockKind.modified => LineMark(
          MarkKind.changed,
          b.neu.start,
          b.neu.end - b.neu.start,
          oldLines.sublist(b.old.start, b.old.end),
        ),
        BlockKind.deleted => LineMark(
          MarkKind.removed,
          b.neu.start,
          0,
          oldLines.sublist(b.old.start, b.old.end),
        ),
      },
  ];
}

List<LineMark> _marksOf((String, String) texts) =>
    lineMarks(texts.$1, texts.$2);

/// [lineMarks]: at once, or off the UI thread past [offThread]
/// characters, as a diff's box does.
FutureOr<List<LineMark>> lineMarksSoon(String base, String shown) {
  if (base.length + shown.length < offThread) return lineMarks(base, shown);
  return compute(_marksOf, (base, shown));
}

/// What one line's gutter shows: its [bar] (an added or changed run it's
/// in), and a removed run's mark on its [top] or (the last line, where
/// lines went at the end) its [bottom] edge.
@immutable
class MarkCell {
  const new({this.bar, this.top, this.bottom});

  final LineMark? bar;
  final LineMark? top;
  final LineMark? bottom;

  /// What its card tells, top to bottom.
  List<LineMark> get marks => [?top, ?bar, ?bottom];
}

/// Each of [lines] lines' cell of [marks] (null for a line with none).
List<MarkCell?> markCells(List<LineMark> marks, int lines) {
  final cells = List<MarkCell?>.filled(lines, null);
  if (lines == 0) return cells;
  for (final m in marks) {
    if (m.kind == MarkKind.removed) {
      if (m.first < lines) {
        final c = cells[m.first];
        cells[m.first] = MarkCell(bar: c?.bar, top: m, bottom: c?.bottom);
      } else {
        final c = cells[lines - 1];
        cells[lines - 1] = MarkCell(bar: c?.bar, top: c?.top, bottom: m);
      }
      continue;
    }
    for (var i = m.first; i < m.first + m.count && i < lines; i++) {
      final c = cells[i];
      cells[i] = MarkCell(bar: m, top: c?.top, bottom: c?.bottom);
    }
  }
  return cells;
}

/// A shown file's last commit's text, what its change marks are drawn
/// against ([value]; null draws none), asked of the daemon (GetDiff with
/// the file's path, API [oneFileDiffApi]) as the file changes on disk,
/// the chat's turn ends, it commits or its folder changes; and whether the
/// file is new to git ([isNew]: untracked or added), which shows beside
/// its path instead. The file's own folder is asked, so a file in another
/// root or a nested repository is compared against its own repository.
class LineBase extends ChangeNotifier implements ValueListenable<String?> {
  new(this.daemon, this.chat, this.watch) {
    _folder = chat.folder;
    _commits = chat.commits;
    _wasBusy = chat.busy;
    chat.addListener(_chatChanged);
    watch.addListener(_fileChanged);
    _fileChanged();
  }

  final Daemon daemon;
  final ChatState chat;
  final FileWatch watch;

  String? _base;
  bool _isNew = false;

  @override
  String? get value => _base;

  /// Whether git doesn't know the file yet (untracked, or added to the
  /// index): every line would be marked, so none is.
  bool get isNew => _isNew;

  String? _sha;
  late String _folder;
  late int _commits;
  late bool _wasBusy;
  int _asked = 0; // only the latest question's answer lands
  bool _disposed = false;

  void _fileChanged() {
    final f = watch.file;
    final sha = f == null || f.missing || f.tooLarge ? null : f.sha256;
    if (sha == _sha) return;
    _sha = sha;
    unawaited(ask());
  }

  void _chatChanged() {
    final ended = _wasBusy && !chat.busy;
    _wasBusy = chat.busy;
    if (ended || _folder != chat.folder || _commits != chat.commits) {
      _folder = chat.folder;
      _commits = chat.commits;
      unawaited(ask());
    }
  }

  /// Asks again what the file was in the last commit.
  Future<void> ask() async {
    final asked = ++_asked;
    final f = watch.file;
    if (f == null ||
        f.missing ||
        f.tooLarge ||
        watch.notText ||
        daemon.daemonApi < oneFileDiffApi) {
      return _set(asked, null, isNew: false);
    }
    // Unchanged since the last commit, its text then is this.
    final now = utf8.decode(f.data, allowMalformed: true);
    final abs = f.path.isNotEmpty
        ? f.path
        : absoluteIn(chat.folder, watch.path);
    final slash = abs.lastIndexOf('/');
    if (slash < 0) return _set(asked, null, isNew: false);
    try {
      final r = await daemon.client.getDiff(
        GetDiffRequest(
          folder: slash == 0 ? '/' : abs.substring(0, slash),
          path: abs.substring(slash + 1),
        ),
      );
      if (r.files.isEmpty) return _set(asked, now, isNew: false);
      final c = r.files.first;
      final known =
          c.skipped.isEmpty &&
          (c.status == 'modified' || c.status == 'renamed');
      _set(
        asked,
        known ? c.oldText : null,
        isNew:
            c.skipped.isEmpty &&
            (c.status == 'untracked' || c.status == 'added'),
      );
    } on Object {
      // Not in a repository, refused, the daemon gone: no marks.
      _set(asked, null, isNew: false);
    }
  }

  void _set(int asked, String? base, {required bool isNew}) {
    if (_disposed || asked != _asked) return;
    if (base == _base && isNew == _isNew) return;
    _base = base;
    _isNew = isNew;
    notifyListeners();
  }

  @override
  void dispose() {
    _disposed = true;
    chat.removeListener(_chatChanged);
    watch.removeListener(_fileChanged);
    super.dispose();
  }
}
