import 'dart:async';
import 'dart:convert';
import 'dart:ui';

import 'package:nushi/daemon.dart' show dataDir;
import 'package:nushi/mermaid.dart';
import 'package:nushi/web_view_windows.dart';

/// How long a diagram waits on the hidden web view, its queue included.
Duration diagramWait = const Duration(minutes: 1);

/// Windows's diagrams: mermaid.js in one hidden WebView2 of the runner's,
/// made when the first is asked for, drawing one at a time. Its web view
/// takes no snapshot of itself, so the page draws the diagram onto a
/// canvas as PNG (mermaid.html's `png`; labels as SVG text, which a canvas
/// takes). Each gets 10 s, so a slow one doesn't hold the rest; a call
/// that waits past [diagramWait] fails, to be drawn again when next asked
/// for, as the Mac's.
class WindowsDiagrams implements Diagrams {
  const new();

  @override
  Future<DrawnDiagram> draw(
    String source, {
    required bool dark,
    required double width,
    required Color background,
    required Color foreground,
  }) {
    final drawn = _renderer.queue(
      () => _renderer.draw(
        source,
        opts: {
          'dark': dark,
          'background': cssColor(background),
          'foreground': cssColor(foreground),
          'scale': _scale(),
        },
        width: width.clamp(120, 2400).toDouble(),
      ),
    );
    return drawn.timeout(
      diagramWait,
      onTimeout: () => const DrawnDiagram(
        error: 'The diagram took too long to draw',
        retry: true,
      ),
    );
  }
}

/// The screen's scale, the PNG's pixels to a point.
double _scale() {
  final views = PlatformDispatcher.instance.views;
  return views.isEmpty ? 1 : views.first.devicePixelRatio;
}

final _renderer = _Renderer();

/// The hidden web view and its queue.
class _Renderer {
  WebView2? _web;
  Future<bool>? _loaded;
  Future<void> _last = Future.value();
  var _jobs = 0;

  /// Runs [job] after every one queued before it.
  Future<DrawnDiagram> queue(Future<DrawnDiagram> Function() job) {
    final done = _last.then((_) => job());
    _last = done.then((_) {}, onError: (Object _) {});
    return done;
  }

  /// The web view with mermaid.html loaded, made again after a failure.
  Future<WebView2?> _ready() async {
    final loaded = _loaded ??= _load();
    if (await loaded) return _web;
    _web?.close();
    _web = null;
    _loaded = null;
    return null;
  }

  Future<bool> _load() async {
    final web = _web = WebView2(
      folder: webViewFolder(dataDir),
      background: ground(),
    );
    final finished = Completer<bool>();
    web.onFinished = (url, {required ok}) {
      if (url == diagramUrl && !finished.isCompleted) finished.complete(ok);
    };
    if (!await web.ready) return false;
    await web.map(diagramHost, mermaidFolder());
    await web.load(diagramUrl);
    return await finished.future.timeout(
      const Duration(seconds: 20),
      onTimeout: () => false,
    );
  }

  Future<DrawnDiagram> draw(
    String source, {
    required Map<String, Object?> opts,
    required double width,
  }) async {
    final web = await _ready();
    if (web == null) {
      return const DrawnDiagram(
        error: 'There’s no web view to draw diagrams in',
        retry: true,
      );
    }
    final scale = opts['scale']! as double;
    try {
      // Laid out at the width asked, hidden: the page fits the diagram to
      // it.
      await web.bounds(
        Rect.fromLTWH(0, 0, width * scale, 400 * scale),
        visible: false,
      );
      final job = ++_jobs;
      await web.script(
        '(window.__nushi ??= {}, png(${jsonEncode(source)}, '
        '${jsonEncode(opts)}).then((r) => __nushi[$job] = r, '
        '(e) => __nushi[$job] = { error: String((e && e.message) || e) }), '
        'void 0)',
      );
      for (var i = 0; i < 200; i++) {
        // design: a poll of the page's promise, not motion
        await Future<void>.delayed(const Duration(milliseconds: 50));
        final r = await web.script(
          '(() => { const r = window.__nushi?.[$job] ?? null; '
          'if (r) delete __nushi[$job]; return r; })()',
        );
        if (r is Map) return drawnFrom(r.cast<String, Object?>());
      }
      return const DrawnDiagram(
        error: 'The diagram took too long to draw',
        retry: true,
      );
    } on Object catch (e) {
      return DrawnDiagram(error: '$e', retry: true);
    }
  }
}

/// What mermaid.html's `png` answered, as a [DrawnDiagram]: its PNG (base64)
/// and size, or why it isn't drawn (an empty PNG, a canvas too big to
/// draw, is none).
DrawnDiagram drawnFrom(Map<String, Object?> r) {
  if (r['png'] case final String png when png.isNotEmpty) {
    return DrawnDiagram(
      png: base64Decode(png),
      width: (r['width']! as num).toDouble(),
      height: (r['height']! as num).toDouble(),
    );
  }
  return DrawnDiagram(error: '${r['error'] ?? 'Not drawn'}');
}
