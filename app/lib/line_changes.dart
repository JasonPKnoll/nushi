import 'dart:async';
import 'dart:convert';

import 'package:diff_view/diff_view.dart';
import 'package:flutter/foundation.dart';

import 'package:nushi/changes_span.dart';
import 'package:nushi/chat.dart';
import 'package:nushi/daemon.dart';
import 'package:nushi/diff_box.dart' show maxDiffLines, offThread;
import 'package:nushi/gen/nushi/v1/nushi.pb.dart';
import 'package:nushi/preview.dart' show FileWatch;

// A file preview's change marks (ADR 0131): which of its lines the chat's
// branch changed (else, with no branch base, since the last commit), worked
// out in the app from the base's text (GetDiff with a path) and the text it
// shows, so they follow unsaved edits too.

/// What a change mark says of its lines.
enum MarkKind { added, changed, removed }

/// A run of lines that differ from the base (`LineBase`): [count] lines
/// from [first] (from 0) of the text shown, added or changed; or, removed,
/// none, the lines having stood before line [first]. [old] is what the
/// base had there (none for added lines).
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

/// The marks of [shown] against [base], the text before: none when
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

/// A shown file's text before its chat's work, what its change marks are
/// drawn against ([value]; null draws none): as the chat's branch left its
/// base ([branch]), committed work and all, else in the last commit (ADR
/// 0131). The base is the one the Changes tab's Branch view counts from
/// (`ChangeSpan.branchBase`); there is none when the chat works on that
/// branch itself or has no base, or the file is outside the chat's folder
/// (another root, so another repository's branches). Asked of the daemon
/// (GetDiff with the file's path and the base, API [oneFileDiffApi]) as
/// the file changes on disk, the chat's turn ends, it commits or its
/// folder or worktree changes; the base itself is found again when the
/// turn ends or the folder changes, as the branch checked out may have.
/// Also whether the file is new to git against it ([isNew]: untracked, or
/// added since), which shows beside its path instead. The file's own
/// folder is asked, so a file in a nested repository is compared against
/// its own repository.
class LineBase extends ChangeNotifier implements ValueListenable<String?> {
  new(this.daemon, this.chat, this.watch) {
    _folder = chat.folder;
    _worktree = chat.worktree?.base ?? '';
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
  bool _branch = false;

  @override
  String? get value => _base;

  /// Whether the file is new against [value]'s point (untracked, added to
  /// the index, or added on the branch): every line would be marked, so
  /// none is.
  bool get isNew => _isNew;

  /// Whether [value] is the text from before the chat's branch, not the
  /// last commit's: a card says "Before this branch".
  bool get branch => _branch;

  String? _sha;
  late String _folder;
  late String _worktree;
  late int _commits;
  late bool _wasBusy;
  Future<String>? _branchBase; // the base found, until it may have changed
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
    final worktree = chat.worktree?.base ?? '';
    final moved = _folder != chat.folder || _worktree != worktree;
    if (ended || moved || _commits != chat.commits) {
      if (ended || moved) _branchBase = null;
      _folder = chat.folder;
      _worktree = worktree;
      _commits = chat.commits;
      unawaited(ask());
    }
  }

  /// The branch the chat's Branch view counts from: '' for none, or when
  /// HEAD is on it. A failure is found again next time.
  Future<String> _findBase() async {
    try {
      final b = await ChangeSpan.branchBase(daemon, chat);
      final span = ChangeSpan(chat.folder, Scope.branch, b.base, b.status);
      return b.status.git && !span.onBase ? b.base : '';
    } on Object {
      _branchBase = null;
      return '';
    }
  }

  /// Asks again what the file was before the branch, or in the last
  /// commit.
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
    // Unchanged since the base, its text then is this.
    final now = utf8.decode(f.data, allowMalformed: true);
    final abs = f.path.isNotEmpty
        ? f.path
        : absoluteIn(chat.folder, watch.path);
    final slash = abs.lastIndexOf('/');
    if (slash < 0) return _set(asked, null, isNew: false);
    final inChat = abs.startsWith('${chat.folder}/');
    final base = inChat ? await (_branchBase ??= _findBase()) : '';
    if (_disposed || asked != _asked) return;
    Future<GetDiffResponse> diff(String base) => daemon.client.getDiff(
      GetDiffRequest(
        folder: slash == 0 ? '/' : abs.substring(0, slash),
        path: abs.substring(slash + 1),
        base: base,
      ),
    );
    try {
      var against = base;
      GetDiffResponse r;
      try {
        r = await diff(against);
      } on Object {
        // Not a branch of the file's own repository (a nested one): from
        // its last commit instead.
        if (against.isEmpty) rethrow;
        against = '';
        r = await diff(against);
      }
      final branch = against.isNotEmpty;
      if (r.files.isEmpty) {
        return _set(asked, now, isNew: false, branch: branch);
      }
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
        branch: branch,
      );
    } on Object {
      // Not in a repository, refused, the daemon gone: no marks.
      _set(asked, null, isNew: false);
    }
  }

  void _set(
    int asked,
    String? base, {
    required bool isNew,
    bool branch = false,
  }) {
    if (_disposed || asked != _asked) return;
    if (base == _base && isNew == _isNew && branch == _branch) return;
    _base = base;
    _isNew = isNew;
    _branch = branch;
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
