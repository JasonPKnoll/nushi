import 'dart:async';
import 'dart:math';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nushi/beside.dart';
import 'package:nushi/chat_view.dart';
import 'package:nushi/daemon.dart';
import 'package:nushi/files_pane.dart';
import 'package:nushi/gen/nushi/v1/nushi.pb.dart';
import 'package:nushi/panes.dart';
import 'package:nushi/phone/ui.dart';
import 'package:nushi/settings.dart';
import 'package:nushi/spinner.dart';
import 'package:nushi/top_notices.dart';
import 'package:nushi/update_toast.dart';
import 'package:nushi/widgets.dart';

import 'fake_daemon.dart';
import 'harness.dart';
import 'update_fixtures.dart';

// Where the toasts and the update card show, in the window and on the
// phone, rendered to look at.

void main() {
  setUpAll(loadFonts);
  setUp(() {
    dismissedUpdate.value = null;
    brewCask = null;
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(
          const MethodChannel('nushi/fonts'),
          (_) async => <String>[],
        );
  });

  void say(BuildContext context, String text, {ToastAction? action}) =>
      showToast(Toasts.maybeOf(context), text, action: action);
  void hide(BuildContext context) => Toasts.maybeOf(context)!.hide();

  for (final width in [1700.0, 720.0]) {
    for (final (name, brightness) in [
      ('dark', Brightness.dark),
      ('light', Brightness.light),
    ]) {
      for (final hidden in [false, true]) {
        final where = '${width.toInt()}_${name}_${hidden ? 'nobar' : 'bar'}';
        testWidgets('toasts and the update card, $where', (tester) async {
          sidebarHidden.value = hidden;
          // Each Check for updates waits for its answer here.
          var checked = Completer<CheckForUpdatesResponse>();
          var found = UpdateCheck();
          final fake = updateDaemon(UpdateCheck.new)
            ..answer = (input) => switch (input) {
              PingRequest() => PingResponse(
                apiVersion: apiVersion,
                version: '0.1.16',
                update: found,
              ),
              CheckForUpdatesRequest() => checked.future,
              _ => null,
            };
          final d = await openWithUpdate(
            tester,
            fake,
            width: width,
            brightness: brightness,
          );
          final context = tester.element(find.byType(Scaffold));
          Future<void> shot(String what) async {
            await tester.pumpAndSettle();
            await render(tester, find.byType(MaterialApp), '${what}_$where');
          }

          // Under the header, 8 points down, centred over the transcript
          // (the composer's centre), left of the details rail.
          final main = tester.getRect(find.byType(Beside));
          final transcript = tester.getRect(find.byType(ChatView));
          final headerBottom = main.top + PaneHeader.height;
          expect(transcript.top, headerBottom);
          void under(Rect box) {
            expect(box.top, headerBottom + 8);
            expect(box.center.dx, closeTo(transcript.center.dx, 0.5));
            expect(box.width, ToastHost.width);
          }

          say(context, 'Committed 3f2a9c1');
          await shot('a_plain');
          under(toast(tester));
          hide(context);
          await tester.pumpAndSettle();
          say(
            context,
            'Archived side chat',
            action: (label: 'Undo', onPressed: () {}),
          );
          await shot('b_action');
          under(toast(tester));
          hide(context);
          await tester.pumpAndSettle();
          var checking = checkForUpdates(context, d);
          await tester.pump(const Duration(milliseconds: 400));
          await shot('c_checking');
          expect(
            find.descendant(
              of: find.byType(ToastLine),
              matching: find.byType(Spinner),
            ),
            findsOneWidget,
          );
          checked.complete(
            CheckForUpdatesResponse(update: UpdateCheck(latest: '0.1.16')),
          );
          await checking;
          await shot('d_up_to_date');
          expect(find.text('Nushi is up to date (0.1.16).'), findsOneWidget);
          expect(
            find.descendant(
              of: find.byType(ToastLine),
              matching: find.byIcon(Icons.check_circle),
            ),
            findsOneWidget,
          );
          expect(find.byType(Spinner), findsNothing);
          hide(context);
          await tester.pumpAndSettle();
          // The update card, alone and with a toast.
          found = newerUpdate('0.1.17');
          checked = Completer()
            ..complete(CheckForUpdatesResponse(update: newerUpdate('0.1.17')));
          checking = checkForUpdates(context, d);
          await tester.pump();
          await checking;
          await shot('e_card');
          // At the top right, under the header's buttons.
          final card = tester.getRect(updateToast);
          expect(card.top, headerBottom + 8);
          expect(card.right, main.right - TopNotices.cardInset);
          say(
            context,
            'Archived side chat',
            action: (label: 'Undo', onPressed: () {}),
          );
          await shot('f_card_toast');
          final both = toast(tester);
          expect(both.overlaps(card), isFalse);
          // A narrow window has the toast under the card; a wide one beside.
          expect(both.top, both.right < card.left ? card.top : card.bottom + 8);
          // Over Settings, where it was, and pressable.
          d.openSettings();
          await shot('h_settings');
          expect(toast(tester), both);
          expect(find.text('Undo').hitTestable(), findsOneWidget);
          d.closeSettings();
          await tester.pumpAndSettle();
          // Beside a side pane: over what's left of the transcript.
          hide(context);
          toggleFiles(d.chats['c']!);
          await tester.pumpAndSettle();
          say(context, 'Committed 3f2a9c1');
          await shot('i_side_pane');
          final left = tester.getRect(find.byType(ChatView));
          final files = tester.getRect(find.byType(FilesPane));
          expect(
            toast(tester).center.dx,
            closeTo((left.left + min(left.right, files.left)) / 2, 0.5),
          );
          toggleFiles(d.chats['c']!);
          await closeView(tester, d);
        }, variant: macOSOnly);
      }
    }
  }

  testWidgets('the phone: a toast at the bottom', (tester) async {
    viewport(tester, const Size(390, 844), ratio: 3);
    final phone = await startPhone(
      tester,
      FakeDaemon([chat('a', 'claude', 'Fix the login redirect')]),
    );
    await tester.pumpAndSettle();
    phoneSay(tester.element(find.text('Fix the login redirect')), 'Copied');
    await tester.pumpAndSettle();
    await render(tester, find.byType(MaterialApp), 'g_phone');
    // At the bottom, near the thumb, the screen's width but its margins.
    final box = toast(tester);
    expect(box.bottom, 844 - 8);
    expect(box.left, 8);
    expect(box.right, 390 - 8);
    await endPhone(tester, phone);
  });
}

/// The showing toast's card.
Rect toast(WidgetTester tester) => tester.getRect(
  find
      .ancestor(of: find.byType(ToastLine), matching: find.byType(Material))
      .first,
);
