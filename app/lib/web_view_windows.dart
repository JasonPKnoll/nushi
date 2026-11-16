import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:ui' show PlatformDispatcher;

import 'package:flutter/rendering.dart';
import 'package:flutter/scheduler.dart';
import 'package:flutter/services.dart';
import 'package:flutter/widgets.dart';

import 'package:nushi/theme/tokens.dart';

/// The runner's web views (`windows/runner/web_views.cpp`, ADR 0135).
const _channel = MethodChannel('nushi/webview');

/// One of the runner's WebView2s: a child window over the Flutter view,
/// placed where its [view] is laid out, shown while that is (not on an
/// offstage route or a hidden tab), hidden when it goes. It shows one page
/// ([load]); a navigation elsewhere is cancelled, and one the user started
/// comes to [onLeave].
final class WebView2 {
  /// Asks the runner for a web view whose data is under [folder], filled
  /// with [background] until its page draws.
  new({required String folder, required Color background}) : id = _next++ {
    _views[id] = this;
    _listen();
    ready = _channel
        .invokeMethod<void>('make', {
          'id': id,
          'folder': folder,
          'background': background.toARGB32(),
        })
        .then((_) => true, onError: (Object _) => false);
  }

  static var _next = 1;
  static final _views = <int, WebView2>{};
  static var _listening = false;

  /// Its number, the runner's name for it.
  final int id;

  /// Whether the runner made it: false without the WebView2 runtime.
  late final Future<bool> ready;

  /// A navigation the user started (a click on a link, a new window) to an
  /// address this web view doesn't show, cancelled: for the app to open
  /// in the browser.
  void Function(String url)? onLeave;

  /// A main-frame navigation ended, at its address, loaded or not.
  void Function(String url, {required bool ok})? onFinished;

  bool _closed = false;
  Rect? _placed; // in the view's physical pixels, while shown

  /// The widget that holds its place: it fills what it's given, and the
  /// web view is put over it.
  // ignore: use_to_and_as_if_applicable, a view of it, not a conversion
  Widget view() => _Place(this);

  /// Serves [folder]'s files as `https://<host>/`, and none outside it.
  Future<void> map(String host, String folder) =>
      _call('map', {'host': host, 'folder': folder});

  /// Shows [url], the one page it shows from now on.
  Future<void> load(String url) => _call('load', {'url': url});

  Future<void> reload() => _call('reload');

  /// Runs [js] in the page; its result, decoded from JSON (a promise's is
  /// `{}`: WebView2 doesn't wait for one).
  Future<Object?> script(String js) async {
    final json = await _invoke<String>('script', {'js': js});
    return json == null ? null : jsonDecode(json);
  }

  /// Puts it at [r] (physical pixels of the Flutter view), shown when
  /// [visible]: a hidden one is still laid out at that size.
  Future<void> bounds(Rect r, {required bool visible}) => _call('bounds', {
    'x': r.left.round(),
    'y': r.top.round(),
    'width': r.width.round(),
    'height': r.height.round(),
    'visible': visible,
  });

  void _place(Rect? r) {
    if (_closed || r == _placed) return;
    final was = _placed;
    _placed = r;
    if (r != null) {
      unawaited(bounds(r, visible: true).catchError((Object _) {}));
    } else if (was != null) {
      unawaited(bounds(was, visible: false).catchError((Object _) {}));
    }
  }

  /// Closes it: its page stops, and it's gone from the window.
  void close() {
    if (_closed) return;
    _closed = true;
    _views.remove(id);
    unawaited(_call('close').catchError((Object _) {}));
  }

  Future<void> _call(String method, [Map<String, Object?>? args]) =>
      _invoke<void>(method, args);

  Future<T?> _invoke<T>(String method, Map<String, Object?>? args) async {
    if (!await ready || _closed && method != 'close') return null;
    return await _channel.invokeMethod<T>(method, {'id': id, ...?args});
  }

  static void _listen() {
    if (_listening) return;
    _listening = true;
    _channel.setMethodCallHandler((call) async {
      final args = (call.arguments as Map).cast<String, Object?>();
      final view = _views[args['id']];
      if (view == null) return;
      final url = '${args['url'] ?? ''}';
      switch (call.method) {
        case 'leave':
          view.onLeave?.call(url);
        case 'finished':
          view.onFinished?.call(url, ok: args['ok'] == true);
      }
    });
  }
}

/// Where a [WebView2] goes: a box that fills what it's given, and after
/// each frame puts the web view over itself, or hides it while its
/// subtree's tickers are off (an offstage route, a hidden tab) or once it's
/// gone.
class _Place extends LeafRenderObjectWidget {
  const new(this.web);

  final WebView2 web;

  @override
  RenderObject createRenderObject(BuildContext context) => _RenderPlace(
    web,
    ratio: View.of(context).devicePixelRatio,
    shown: TickerMode.valuesOf(context).enabled,
  );

  @override
  void updateRenderObject(BuildContext context, _RenderPlace place) => place
    ..ratio = View.of(context).devicePixelRatio
    ..shown = TickerMode.valuesOf(context).enabled;
}

class _RenderPlace extends RenderBox {
  new(this.web, {required this.ratio, required this.shown});

  final WebView2 web;
  double ratio;
  bool shown;
  bool _watching = false;

  @override
  bool get sizedByParent => true;

  @override
  Size computeDryLayout(BoxConstraints constraints) => constraints.biggest;

  @override
  void attach(PipelineOwner owner) {
    super.attach(owner);
    _watch();
  }

  @override
  void detach() {
    web._place(null);
    super.detach();
  }

  /// After each frame while attached, where it is now (a scroll or a
  /// slide moves it without a layout of its own).
  void _watch() {
    if (_watching) return;
    _watching = true;
    SchedulerBinding.instance.addPostFrameCallback((_) {
      _watching = false;
      if (!attached) return;
      _check();
      _watch();
    });
  }

  void _check() {
    if (!hasSize || !shown || size.isEmpty) return web._place(null);
    final at = localToGlobal(Offset.zero) * ratio;
    web._place(at & size * ratio);
  }
}

/// The folder where the web views keep their data: beside Nushi's data
/// folder (`%LOCALAPPDATA%\Nushi.WebView2`), not in it. Chromium gives its
/// cache folders access entries of their own, and the daemon won't start
/// on a data folder with such an entry (ADR 0124); nor is the app's
/// folder writable once installed.
String webViewFolder(String dataFolder) => '$dataFolder.WebView2';

/// The app's bundled mermaid assets (`assets/mermaid/`), as Flutter puts
/// them beside `nushi.exe`.
String mermaidFolder() =>
    '${File(Platform.resolvedExecutable).parent.path}'
    r'\data\flutter_assets\assets\mermaid';

/// The host the mermaid assets are served as, and their page's address.
const diagramHost = 'diagrams.nushi.example';
const diagramUrl = 'https://$diagramHost/mermaid.html';

/// What a web view shows before its page draws: the system's light or
/// dark ground, as the app starts in.
Color ground() =>
    (PlatformDispatcher.instance.platformBrightness == Brightness.dark
            ? Tokens.dark
            : Tokens.light)
        .base;
