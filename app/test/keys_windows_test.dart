import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nushi/chat.dart';
import 'package:nushi/composer.dart';
import 'package:nushi/conventions.dart';
import 'package:nushi/daemon.dart';
import 'package:nushi/main.dart';
import 'package:nushi/native.dart';
import 'package:nushi/settings.dart';
import 'package:nushi/window_drag_windows.dart';

import 'fake_daemon.dart';
import 'harness.dart';

// The app's keys on Windows (ADR 0124): Linux's, held with Control, but
// none with Control and Alt, which is AltGr there: a Spanish keyboard
// types `|`, `@` and `#` with AltGr and 1–3, and Windows reports AltGr as
// Control and right Alt (and takes Ctrl+Alt for it too). The chat's tabs
// are Ctrl+Shift+1–3.

final _windows = TargetPlatformVariant.only(TargetPlatform.windows);

const _ctrlShift = <LogicalKeyboardKey>[
  LogicalKeyboardKey.controlLeft,
  LogicalKeyboardKey.shiftLeft,
];

/// AltGr as Windows sends it: Control first, then right Alt.
const _altGr = <LogicalKeyboardKey>[
  LogicalKeyboardKey.controlLeft,
  LogicalKeyboardKey.altRight,
];

const _ctrlAlt = <LogicalKeyboardKey>[
  LogicalKeyboardKey.controlLeft,
  LogicalKeyboardKey.altLeft,
];

/// The app on Windows on two chats, a shown.
Future<Daemon> _app(WidgetTester tester) async {
  native
    ..conventions = windowsConventions
    ..window = const WindowsWindow();
  viewport(tester, const Size(1200, 760));
  showDetails.value = false;
  final fake = FakeDaemon([
    chat('a', 'claude', 'Fix the login redirect', age: 2),
    chat('b', 'codex', 'Bump the API version', age: 1),
  ]);
  final d = await fake.connect(tester);
  d.select('a');
  await tester.pumpWidget(NushiApp(daemon: d));
  await tester.pumpAndSettle();
  return d;
}

/// Presses [key], typing [character], with [holding] held down, as
/// Windows sends it; whether the app handled the press.
Future<bool> _press(
  WidgetTester tester,
  LogicalKeyboardKey key,
  List<LogicalKeyboardKey> holding, {
  String? character,
}) async {
  for (final k in holding) {
    await tester.sendKeyDownEvent(k, platform: 'windows');
  }
  final handled = await tester.sendKeyDownEvent(
    key,
    platform: 'windows',
    character: character,
  );
  await tester.sendKeyUpEvent(key, platform: 'windows');
  for (final k in holding.reversed) {
    await tester.sendKeyUpEvent(k, platform: 'windows');
  }
  await tester.pumpAndSettle();
  return handled;
}

Finder get _box => find
    .descendant(of: find.byType(Composer), matching: find.byType(EditableText))
    .first;

void main() {
  setUpAll(loadFonts);

  testWidgets('Ctrl+Shift+2 shows Changes and Ctrl+Shift+1 Chat; AltGr+1–3 '
      '(right Alt held) and Ctrl+Alt+1–3 type, into the box from the window '
      'and left to the focused box', (tester) async {
    final d = await _app(tester);
    final a = d.chats['a']!;
    expect(await _press(tester, LogicalKeyboardKey.digit2, _ctrlShift), true);
    expect(a.shownTab, ChatTab.changes);
    expect(await _press(tester, LogicalKeyboardKey.digit1, _ctrlShift), true);
    expect(a.shownTab, ChatTab.chat);

    // With nothing focused, type to write takes it to the box (ADR 0121),
    // which takes the focus.
    await _press(tester, LogicalKeyboardKey.digit2, _altGr, character: '@');
    expect(a.shownTab, ChatTab.chat);
    expect(tester.widget<EditableText>(_box).controller.text, '@');
    expect(tester.widget<EditableText>(_box).focusNode.hasFocus, isTrue);

    for (final (digit, typed) in [
      (LogicalKeyboardKey.digit1, '|'),
      (LogicalKeyboardKey.digit2, '@'),
      (LogicalKeyboardKey.digit3, '#'),
    ]) {
      for (final held in [_altGr, _ctrlAlt]) {
        final handled = await _press(tester, digit, held, character: typed);
        expect(handled, isFalse, reason: typed);
        expect(a.shownTab, ChatTab.chat);
        expect(d.selectedId, 'a');
      }
    }
    await closeView(tester, d);
  }, variant: _windows);
}
