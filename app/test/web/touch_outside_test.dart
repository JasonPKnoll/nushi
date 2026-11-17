// A touch outside a focused text field in a phone's browser, where the
// framework itself lets go of the field on the touch's press (kIsWeb): a
// scroll closed the keyboard. The phone's TouchOutside lets go on a tap's
// lift only (touch_outside.dart).
// Run with: flutter test --platform chrome test/web
@TestOn('browser')
library;

import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nushi/phone/touch_outside.dart';

/// A list to scroll over a field with the focus, as the chat has them.
Future<FocusNode> _typing(WidgetTester tester, {required bool phone}) async {
  final focus = FocusNode();
  addTearDown(focus.dispose);
  final page = Scaffold(
    body: Column(
      children: [
        Expanded(
          child: ListView(
            children: [
              for (var i = 0; i < 40; i++)
                SizedBox(height: 40, child: Text('Answer $i')),
            ],
          ),
        ),
        TextField(focusNode: focus),
      ],
    ),
  );
  await tester.pumpWidget(
    MaterialApp(home: phone ? TouchOutside(child: page) : page),
  );
  focus.requestFocus();
  await tester.pump();
  expect(focus.hasFocus, isTrue);
  return focus;
}

/// An iPhone's Safari.
final _iPhone = TargetPlatformVariant.only(TargetPlatform.iOS);

void main() {
  testWidgets("the framework's own rule: a scroll lets go of the field", (
    tester,
  ) async {
    final focus = await _typing(tester, phone: false);
    await tester.drag(find.text('Answer 3'), const Offset(0, -200));
    await tester.pump();
    expect(focus.hasFocus, isFalse);
  }, variant: _iPhone);

  testWidgets("the phone's: a scroll keeps the field", (tester) async {
    final focus = await _typing(tester, phone: true);
    await tester.drag(find.text('Answer 3'), const Offset(0, -200));
    await tester.pump();
    expect(find.text('Answer 3'), findsNothing, reason: 'scrolled');
    expect(focus.hasFocus, isTrue);
  }, variant: _iPhone);

  testWidgets("the phone's: a tap lets go as the finger lifts", (tester) async {
    final focus = await _typing(tester, phone: true);
    final finger = await tester.startGesture(
      tester.getCenter(find.text('Answer 3')),
    );
    await tester.pump();
    expect(focus.hasFocus, isTrue);
    await finger.up();
    await tester.pump();
    expect(focus.hasFocus, isFalse);
  }, variant: _iPhone);

  testWidgets("the phone's: a mouse's press lets go at once", (tester) async {
    final focus = await _typing(tester, phone: true);
    final mouse = await tester.startGesture(
      tester.getCenter(find.text('Answer 3')),
      kind: PointerDeviceKind.mouse,
    );
    await tester.pump();
    expect(focus.hasFocus, isFalse);
    await mouse.up();
  }, variant: _iPhone);
}
