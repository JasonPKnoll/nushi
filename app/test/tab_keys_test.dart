import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nushi/chat.dart';
import 'package:nushi/composer.dart';
import 'package:nushi/controls.dart';
import 'package:nushi/daemon.dart';
import 'package:nushi/gen/nushi/v1/nushi.pb.dart';
import 'package:nushi/main.dart';

import 'fake_daemon.dart';
import 'harness.dart';

// ⌃Tab and ⌃⇧Tab, the chat's tabs, wherever the focus is (`chat_keys.dart`),
// pressed as the Mac sends them: Control, then Shift, then Tab (its
// character a backtab with Shift), and up in reverse.

/// b has a pull request: three tabs.
final _pull = GitHubLink(
  host: 'github.com',
  repo: 'afrase/nushi',
  number: 12,
  id: 'PR_12',
  pull: true,
);

Future<Daemon> _app(WidgetTester tester) async {
  viewport(tester, const Size(1200, 700));
  final fake =
      FakeDaemon([
          chat('a', 'claude', 'Fix the login redirect', age: 2),
          chat('b', 'codex', 'Bump the API version', age: 1)..pull = _pull,
        ])
        ..answer = (input) => input is FindChatPullRequest
            ? FindChatPullResponse(pull: input.chatId == 'b' ? _pull : null)
            : null;
  final d = await fake.connect(tester);
  await tester.pumpWidget(NushiApp(daemon: d));
  await tester.pumpAndSettle();
  d.select('b');
  await tester.pumpAndSettle();
  return d;
}

/// ⌃Tab, or with [back] ⌃⇧Tab, as macOS sends it.
Future<void> _tab(WidgetTester tester, {bool back = false}) async {
  const mac = 'macos';
  await tester.sendKeyDownEvent(LogicalKeyboardKey.controlLeft, platform: mac);
  if (back) {
    await tester.sendKeyDownEvent(LogicalKeyboardKey.shiftLeft, platform: mac);
  }
  await tester.sendKeyDownEvent(
    LogicalKeyboardKey.tab,
    platform: mac,
    character: back ? '\u0019' : '\t',
  );
  await tester.sendKeyUpEvent(LogicalKeyboardKey.tab, platform: mac);
  if (back) {
    await tester.sendKeyUpEvent(LogicalKeyboardKey.shiftLeft, platform: mac);
  }
  await tester.sendKeyUpEvent(LogicalKeyboardKey.controlLeft, platform: mac);
  await tester.pumpAndSettle();
}

Finder get _box => find.descendant(
  of: find.byType(Composer),
  matching: find.byType(EditableText),
);

Finder _segment(String label) => find.descendant(
  of: find.byType(Segmented<ChatTab>),
  matching: find.text(label),
);

void main() {
  setUpAll(loadFonts);
  final macOS = TargetPlatformVariant.only(TargetPlatform.macOS);

  testWidgets('after a click outside the box (a header tab), ⌃⇧Tab and '
      '⌃Tab still step', (tester) async {
    final d = await _app(tester);
    final b = d.chats['b']!;
    await tester.tap(_box);
    await tester.pumpAndSettle();
    // The click lets go of the box (the framework's unfocus on the
    // desktop): the focus stays under the window's keys.
    await tester.tap(_segment('Changes'));
    await tester.pumpAndSettle();
    expect(b.shownTab, ChatTab.changes);
    await _tab(tester, back: true);
    expect(b.shownTab, ChatTab.chat);
    await _tab(tester, back: true);
    expect(b.shownTab, ChatTab.pull); // round
    await _tab(tester);
    expect(b.shownTab, ChatTab.chat);
    // And a character typed then goes into the box (ADR 0121).
    await tester.tap(_box);
    await tester.pumpAndSettle();
    await tester.tap(_segment('Chat'));
    await tester.pumpAndSettle();
    await tester.sendKeyEvent(LogicalKeyboardKey.keyX, platform: 'macos');
    await tester.pumpAndSettle();
    expect(tester.widget<EditableText>(_box).controller.text, 'x');
    d.dispose();
  }, variant: macOS);

  testWidgets('with the box’s choices showing, ⌃Tab and ⌃⇧Tab step the tabs '
      'and pick none', (tester) async {
    final d = await _app(tester);
    final b = d.chats['b']!;
    await tester.tap(_box);
    await tester.enterText(_box, '/co');
    await tester.pumpAndSettle();
    expect(find.textContaining('/copy', findRichText: true), findsWidgets);
    await _tab(tester);
    expect(b.shownTab, ChatTab.changes);
    await _tab(tester, back: true);
    expect(b.shownTab, ChatTab.chat);
    expect(tester.widget<EditableText>(_box).controller.text, '/co');
    d.dispose();
  }, variant: macOS);
}
