import 'package:flutter/material.dart';
import 'package:flutter/src/scheduler/ticker.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nushi/motion.dart';
import 'package:nushi/slides.dart';
import 'package:nushi/theme/theme.dart';

/// Counts its States: one made again would count two.
class _Probe extends StatefulWidget {
  const new();

  static int made = 0;

  @override
  State<_Probe> createState() => _ProbeState();
}

class _ProbeState extends State<_Probe> with SingleTickerProviderStateMixin {
  late final Ticker ticker = createTicker((_) {});

  @override
  void initState() {
    super.initState();
    _Probe.made++;
  }

  @override
  void dispose() {
    ticker.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) =>
      const SizedBox(key: Key('probe'), width: 100, height: 100);
}

void main() {
  setUp(() => _Probe.made = 0);

  Widget presence({required bool shown, bool child = true}) => MaterialApp(
    theme: const NushiTheme(Tokens.dark).toThemeData(),
    home: Align(
      alignment: Alignment.topLeft,
      child: SizedBox(
        width: 100,
        height: 100,
        child: Presence(
          shown: shown,
          child: child ? const _Probe() : null,
          builder: (_, run, c) => paneSlide(run, c),
        ),
      ),
    ),
  );

  final probe = find.byKey(const Key('probe'), skipOffstage: false);
  final probeState = find.byType(_Probe, skipOffstage: false);
  bool ticking(WidgetTester tester) =>
      TickerMode.valuesOf(tester.element(probe)).enabled;
  bool offstage(WidgetTester tester) =>
      find.byKey(const Key('probe')).evaluate().isEmpty;

  testWidgets('not shown, it slides out and stays, kept but not drawn, '
      'reached or ticking', (tester) async {
    await tester.pumpWidget(presence(shown: true));
    final state = tester.state(probeState);
    await tester.pumpWidget(presence(shown: false));
    await tester.pump(Motion.exit ~/ 2);
    expect(offstage(tester), false);
    expect(
      tester
          .widget<IgnorePointer>(
            find
                .ancestor(of: probe, matching: find.byType(IgnorePointer))
                .first,
          )
          .ignoring,
      true,
    );
    await tester.pump(Motion.exit);
    expect(offstage(tester), true);
    expect(ticking(tester), false);
    expect(tester.state(probeState), same(state));
    expect(
      tester
          .widget<ExcludeFocus>(
            find.ancestor(of: probe, matching: find.byType(ExcludeFocus)).first,
          )
          .excluding,
      true,
    );
    await tester.pumpWidget(presence(shown: true));
    await tester.pump(Motion.enter);
    expect(offstage(tester), false);
    expect(ticking(tester), true);
    expect(tester.state(probeState), same(state));
    expect(_Probe.made, 1);
  });

  testWidgets('shown again mid-way, it turns back from where it is', (
    tester,
  ) async {
    await tester.pumpWidget(presence(shown: true));
    await tester.pumpWidget(presence(shown: false));
    await tester.pump(Motion.exit ~/ 2);
    final mid = tester.getTopLeft(probe).dx;
    expect(mid, greaterThan(0));
    await tester.pumpWidget(presence(shown: true));
    await tester.pump();
    expect(tester.getTopLeft(probe).dx, closeTo(mid, 2));
    await tester.pump(Motion.enter);
    expect(tester.getTopLeft(probe).dx, 0);
    expect(_Probe.made, 1);
  });

  testWidgets('taken away while tucked, it is gone at once', (tester) async {
    await tester.pumpWidget(presence(shown: false));
    expect(offstage(tester), true);
    expect(probe, findsOneWidget);
    await tester.pumpWidget(presence(shown: false, child: false));
    expect(probe, findsNothing);
  });

  testWidgets('under Reduce Motion it goes and comes at once', (tester) async {
    tester.platformDispatcher.accessibilityFeaturesTestValue =
        const FakeAccessibilityFeatures(disableAnimations: true);
    addTearDown(tester.platformDispatcher.clearAccessibilityFeaturesTestValue);
    await tester.pumpWidget(presence(shown: true));
    await tester.pumpWidget(presence(shown: false));
    await tester.pump();
    expect(offstage(tester), true);
    await tester.pumpWidget(presence(shown: true));
    await tester.pump();
    expect(offstage(tester), false);
    expect(tester.getTopLeft(probe).dx, 0);
    expect(_Probe.made, 1);
  });

  testWidgets('first built not shown, it is there, tucked', (tester) async {
    await tester.pumpWidget(presence(shown: false));
    expect(offstage(tester), true);
    await tester.pump();
    expect(tester.binding.hasScheduledFrame, false);
    await tester.pumpWidget(presence(shown: true));
    await tester.pump(Motion.enter ~/ 2);
    expect(tester.getTopLeft(probe).dx, greaterThan(0));
    await tester.pump(Motion.enter);
    expect(tester.getTopLeft(probe).dx, 0);
  });
}
