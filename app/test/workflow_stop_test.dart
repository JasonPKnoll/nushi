import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nushi/chat.dart';
import 'package:nushi/clock.dart';
import 'package:nushi/gen/nushi/v1/nushi.pb.dart';
import 'package:nushi/motion.dart';
import 'package:nushi/phone/phone_app.dart';
import 'package:nushi/phone/platform.dart';
import 'package:nushi/subagents.dart';
import 'package:nushi/workflow_card.dart';
import 'package:nushi/workflow_pane.dart';

import 'fake_daemon.dart';
import 'harness.dart';
import 'workflow_card_test.dart' show workflowCall;

// What a workflow's card does beyond how it looks (ADR 0127): its Stop
// under the pointer, its time ticking only while it runs, and on the
// phone, a card that opens nothing and the chat's sheet that stops it.

final Chat _chat = chat(
  'c',
  'claude',
  'Review the right-panes branch',
  status: ChatStatus(background: 1, workflows: 1),
);

FakeDaemon _fake() => FakeDaemon([_chat])
  ..answer = (input) => switch (input) {
    GetChatRequest() => GetChatResponse(
      chat: _chat,
      items: [workflowCall('running')],
      status: _chat.status,
    ),
    _ => null,
  };

void main() {
  setUp(() => Shine.stopAt = 0.3);
  tearDown(() => Shine.stopAt = null);

  testWidgets('under the pointer a running card shows Stop, which stops '
      'its task; an ended one has none', (tester) async {
    final fake = FakeDaemon([]);
    final d = await fake.connect(tester);
    final chat = ChatState('c', '/x');
    for (final state in ['running', 'done']) {
      await tester.pumpWidget(
        themed(WorkflowCard(workflowCall(state), chat, daemon: d, still: true)),
      );
      final mouse = await tester.createGesture(kind: PointerDeviceKind.mouse);
      await mouse.addPointer(
        location: tester.getCenter(find.byType(WorkflowCard)),
      );
      await tester.pump();
      expect(
        find.byTooltip('Stop'),
        state == 'running' ? findsOne : findsNothing,
      );
      if (state == 'running') {
        await tester.tap(find.byTooltip('Stop'));
        await tester.pump();
      }
      await mouse.removePointer();
    }
    final stop = fake.calls.whereType<StopTaskRequest>().single;
    expect((stop.chatId, stop.callId), ('c', 'wf'));
    await closeView(tester, d);
  });

  testWidgets('its time ticks while it runs, not once stalled or ended', (
    tester,
  ) async {
    var now = fixtureNow;
    final was = clock;
    clock = () => now;
    addTearDown(() => clock = was);
    final chat = ChatState('c', '/x');
    for (final (state, stalled, live) in [
      ('running', false, true),
      ('running', true, false),
      ('done', false, false),
    ]) {
      await tester.pumpWidget(
        themed(WorkflowCard(workflowCall(state, stalled: stalled), chat)),
      );
      final text = tester.widget<LiveText>(find.byType(LiveText));
      expect(text.live, live, reason: '$state, stalled $stalled');
      final before = text.text();
      now = now.add(const Duration(seconds: 5));
      await tester.pump(const Duration(seconds: 1));
      final shown = tester
          .widgetList<Text>(find.byType(Text))
          .map((t) => t.data);
      expect(shown.contains(before), !live, reason: '$state: $before');
    }
    await tester.pumpWidget(const SizedBox());
  });

  testWidgets('on the phone the card opens nothing; the sheet stops the '
      'workflow by its name', (tester) async {
    viewport(tester, const Size(390, 844), ratio: 3);
    final fake = _fake();
    final phone = await startPhone(
      tester,
      fake,
      platform: PhonePlatform()..saved = pairedRecord(),
    );
    for (var i = 0; i < 5; i++) {
      await tester.pump(
        const Duration(milliseconds: 200),
      ); // design: a test's step
    }
    await tester.tap(find.text('Review the right-panes branch'));
    for (var i = 0; i < 5; i++) {
      await tester.pump(
        const Duration(milliseconds: 200),
      ); // design: a test's step
    }
    await tester.tap(find.byType(WorkflowCard));
    await tester.pump();
    expect(find.byType(WorkflowPane), findsNothing);
    expect(fake.calls.whereType<GetWorkflowRequest>(), isEmpty);
    await tester.tap(find.byIcon(Icons.more_horiz));
    for (var i = 0; i < 5; i++) {
      await tester.pump(
        const Duration(milliseconds: 200),
      ); // design: a test's step
    }
    await tester.tap(find.text('Stop Workflow: review-changes'));
    for (var i = 0; i < 5; i++) {
      await tester.pump(
        const Duration(milliseconds: 200),
      ); // design: a test's step
    }
    final stop = fake.calls.whereType<StopTaskRequest>().single;
    expect((stop.chatId, stop.callId), ('c', 'wf'));
    expect(find.byType(PhoneApp), findsOne);
    await endPhone(tester, phone);
  });
}
