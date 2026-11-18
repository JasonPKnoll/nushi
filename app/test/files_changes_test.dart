import 'dart:convert';

import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nushi/change_marks.dart';
import 'package:nushi/changes_view.dart';
import 'package:nushi/daemon.dart';
import 'package:nushi/files_tree.dart';
import 'package:nushi/gen/nushi/v1/nushi.pb.dart';
import 'package:nushi/main.dart';
import 'package:nushi/preview_path.dart';
import 'package:nushi/source_editor.dart';

import 'change_marks_test.dart' show changeBase, changeShown;
import 'fake_daemon.dart' show FakeDaemon, loadFonts;
import 'file_edit_helpers.dart';
import 'harness.dart';

// The Files pane with change marks (ADR 0131): a written file's
// right-click Show changes, a new file's note, and full windows of the
// marks and their card to look at (written only with NUSHI_RENDERS set).

Item _write(String id, String path) => Item(
  id: id,
  kind: ItemKind.ITEM_KIND_TOOL_CALL,
  toolKind: 'edit',
  title: 'Write',
  status: 'completed',
  paths: [path],
);

/// The chat wrote main.go and notes.md; README.md it didn't.
final List<Item> _items = [
  ...editItems,
  _write('w1', 'main.go'),
  _write('w2', 'notes.md'),
];

final _folder = [
  FolderEntry(name: 'main.go', kind: 'file'),
  FolderEntry(name: 'notes.md', kind: 'file'),
  FolderEntry(name: 'README.md', kind: 'file'),
];

/// The chat's folder on a branch of its own, off main: the marks count
/// from where it left main (`FakeDaemon.fileDiffs` answers either way).
Object? _onBranch(Object input) => switch (input) {
  GetFolderStatusRequest() => GetFolderStatusResponse(
    git: true,
    branch: 'nushi/parser',
    defaultBase: 'main',
  ),
  _ => null,
};

/// The chat's folder on main itself: the marks count from the last
/// commit.
Object? _onMain(Object input) => switch (input) {
  GetFolderStatusRequest() => GetFolderStatusResponse(
    git: true,
    branch: 'main',
    defaultBase: 'main',
  ),
  _ => null,
};

Future<(Daemon, FakeDaemon)> _open(
  WidgetTester tester, {
  List<String> open = const ['main.go'],
  double width = 1700,
  Brightness brightness = Brightness.light,
  String base = changeBase,
  Object? Function(Object input)? answer = _onBranch,
}) => openEditor(
  tester,
  {
    'main.go': utf8.encode(changeShown),
    'notes.md': utf8.encode('# Notes\n\nWhat the parser still misses.\n'),
    'README.md': utf8.encode('# Parser\n'),
  },
  open: open,
  width: width,
  brightness: brightness,
  items: _items,
  folder: _folder,
  answer: answer,
  fileDiffs: {
    '/Users/me/nushi/main.go': FileChange(
      path: 'main.go',
      status: 'modified',
      oldText: base,
    ),
    '/Users/me/nushi/notes.md': FileChange(
      path: 'notes.md',
      status: 'untracked',
    ),
  },
);

Finder _row(String name) =>
    find.descendant(of: find.byType(FilesTree), matching: find.text(name));

void main() {
  setUpAll(loadFonts);

  testWidgets("a written file's right-click opens it in Changes; others "
      'have no menu', (tester) async {
    final (d, _) = await _open(tester);
    await tester.tap(_row('README.md'), buttons: kSecondaryButton);
    await tester.pumpAndSettle();
    expect(find.text('Show changes'), findsNothing);

    await tester.tap(_row('main.go'), buttons: kSecondaryButton);
    await tester.pumpAndSettle();
    expect(find.text('Show changes'), findsOneWidget);
    expect(find.byType(ChangesView), findsNothing);
    await tester.tap(find.text('Show changes'));
    await tester.pumpAndSettle();
    expect(find.byType(ChangesView), findsOneWidget);
    await endEditor(tester, d);
  }, variant: onMac);

  testWidgets('a new file says so on its path line, and has no marks', (
    tester,
  ) async {
    final (d, _) = await _open(tester, open: ['notes.md']);
    await tester.pumpAndSettle();
    expect(
      find.descendant(
        of: find.byType(PathLine),
        matching: find.text('New file'),
      ),
      findsOneWidget,
    );
    expect(find.byType(ChangeMark), findsNothing);
    await endEditor(tester, d);
  }, variant: onMac);

  testWidgets('a changed file has marks in its editor and no note', (
    tester,
  ) async {
    final (d, _) = await _open(tester);
    await tester.pumpAndSettle();
    expect(find.text('New file'), findsNothing);
    expect(
      find.descendant(
        of: find.byType(SourceEditor),
        matching: find.byType(ChangeMark),
      ),
      findsNWidgets(7),
    );
    await endEditor(tester, d);
  }, variant: onMac);

  testWidgets("on the chat's branch, the marks count from main and stay "
      'after a commit; the card says so', (tester) async {
    var committed = false;
    final (d, fake) = await _open(
      tester,
      answer: (input) => switch (input) {
        // Committed, HEAD has the text: only the branch's base differs.
        GetDiffRequest(:final base) when committed && base.isEmpty =>
          GetDiffResponse(),
        _ => _onBranch(input),
      },
    );
    await tester.pumpAndSettle();
    List<GetDiffRequest> asked() => [
      for (final r in fake.calls.whereType<GetDiffRequest>())
        if (r.path == 'main.go') r,
    ];
    expect(asked().last.base, 'main');
    expect(find.byType(ChangeMark), findsNWidgets(7));

    committed = true;
    chatC(d).committed();
    await tester.pumpAndSettle();
    expect(asked().last.base, 'main');
    expect(find.byType(ChangeMark), findsNWidgets(7));
    await tester.tap(find.byType(ChangeMark).first);
    await tester.pumpAndSettle();
    expect(find.text('Before this branch'), findsOneWidget);
    await endEditor(tester, d);
  }, variant: onMac);

  testWidgets('on main itself, from the last commit, and the card says '
      'Before', (tester) async {
    final (d, fake) = await _open(tester, answer: _onMain);
    await tester.pumpAndSettle();
    final asked = fake.calls.whereType<GetDiffRequest>().where(
      (r) => r.path == 'main.go',
    );
    expect(asked.last.base, isEmpty);
    await tester.tap(find.byType(ChangeMark).first);
    await tester.pumpAndSettle();
    expect(find.text('Before'), findsOneWidget);
    expect(find.text('Before this branch'), findsNothing);
    await endEditor(tester, d);
  }, variant: onMac);

  for (final brightness in Brightness.values) {
    final theme = brightness.name;
    for (final width in [1700.0, 720.0]) {
      final w = width.toInt();
      testWidgets(
        'renders: marks, a card, a new file, the menu, $w, $theme',
        (tester) async {
          final (d, _) = await _open(
            tester,
            width: width,
            brightness: brightness,
          );
          await tester.pumpAndSettle();
          final app = find.byType(NushiApp);
          await render(tester, app, 'cl_${w}_marks_$theme');

          // A changed run's card, and the removal's.
          await tester.tap(find.byType(ChangeMark).first);
          await tester.pumpAndSettle();
          expect(find.byType(ChangeCard), findsOneWidget);
          await render(tester, app, 'cl_${w}_card_changed_$theme');
          // A second click hides it.
          await tester.tap(find.byType(ChangeMark).first);
          await tester.pumpAndSettle();
          expect(find.byType(ChangeCard), findsNothing);
          await tester.tap(find.byType(ChangeMark).at(3));
          await tester.pumpAndSettle();
          expect(find.text('2 lines removed'), findsOneWidget);
          await render(tester, app, 'cl_${w}_card_removed_$theme');
          await tester.tap(find.byType(ChangeMark).at(3));
          await tester.pumpAndSettle();

          // The pointer on an added line's mark.
          final mouse = await tester.createGesture(
            kind: PointerDeviceKind.mouse,
          );
          await mouse.addPointer(location: Offset.zero);
          await mouse.moveTo(tester.getCenter(find.byType(ChangeMark).last));
          await tester.pump(const Duration(milliseconds: 500));
          expect(find.text('New lines'), findsOneWidget);
          await render(tester, app, 'cl_${w}_card_added_$theme');
          await mouse.moveTo(Offset.zero);
          await tester.pumpAndSettle();
          await mouse.removePointer();

          // The written file's menu, if the tree shows.
          if (_row('main.go').evaluate().isNotEmpty) {
            await tester.tap(_row('main.go'), buttons: kSecondaryButton);
            await tester.pumpAndSettle();
            await render(tester, app, 'cl_${w}_menu_$theme');
            await tester.tapAt(tester.getCenter(find.byType(PathLine)));
            await tester.pumpAndSettle();
          }

          chatC(d).openInFiles('notes.md');
          await tester.pumpAndSettle();
          await render(tester, app, 'cl_${w}_new_file_$theme');
          await endEditor(tester, d);
        },
        variant: onMac,
        skip: rendersDir == null,
      );
      testWidgets(
        "renders: a long old line's card, $w, $theme",
        (tester) async {
          // The comment was 130 columns long before.
          final long = '// greet says hello. ${'and more words ' * 7}end.';
          final (d, _) = await _open(
            tester,
            width: width,
            brightness: brightness,
            base: changeBase.replaceFirst('// greet says hello.', long),
          );
          await tester.pumpAndSettle();
          await tester.tap(find.byType(ChangeMark).first);
          await tester.pumpAndSettle();
          expectTextInside(tester, width);
          await render(
            tester,
            find.byType(NushiApp),
            'cl_${w}_card_long_$theme',
          );
          await endEditor(tester, d);
        },
        variant: onMac,
        skip: rendersDir == null,
      );
    }
  }
}
