import 'dart:math';

import 'package:flutter/foundation.dart';

import 'package:nushi/browser_address.dart' show isWebPage;
import 'package:nushi/preview_entry.dart';

/// A chat's browser tabs (ADR 0094): their tabs in the order opened and the
/// one that shows in the browser pane, in the app's memory. However the
/// pane closes ([shown] set to null), its tabs go with it; hidden with the
/// globe ([hide]) they're put away to come back.
final class BrowserTabs {
  new() {
    shown.addListener(() {
      if (shown.value == null) tabs.value = const [];
    });
  }

  /// The tabs, in the order opened.
  final tabs = ValueNotifier<List<BrowserTab>>(const []);

  /// The tab the browser pane shows; null while it's closed.
  final shown = ValueNotifier<BrowserTab?>(null);

  /// The tabs [hide] put away, and the one that showed: back as the browser
  /// shows again.
  ({List<BrowserTab> tabs, BrowserTab shown})? _hidden;

  /// Opens a new tab last, at [address] ('' for a new tab's page), and shows
  /// it. Browser tabs are never matched up: each open is its own.
  BrowserTab open([String address = '']) {
    final tab = BrowserTab(address);
    _restore();
    _show(tab);
    return tab;
  }

  /// Shows a server's address at [url] ('http://localhost:3000'): the tab
  /// already at that origin, loaded again if it showed that it couldn't load
  /// (the server wasn't up), else a new one (ADR 0098).
  void showServer(String url) {
    // Uri.origin throws for any page but http(s) to a host (about:blank).
    final server = Uri.tryParse(url);
    final origin = isWebPage(server) ? server!.origin : null;
    final at = tabs.value.where((b) {
      final at = Uri.tryParse(b.address.value);
      return isWebPage(at) && at!.origin == origin;
    }).lastOrNull;
    if (at == null) {
      open(url);
      return;
    }
    _show(at);
    at.view?.retry();
  }

  /// Shows a page its agent asked for: a server's address as [showServer]
  /// does; a page within a site in the tab already at it (loaded again if
  /// it didn't load), else a new one.
  void showPage(String url) {
    final at = Uri.tryParse(url);
    if (at == null || (at.path.isEmpty || at.path == '/') && !at.hasQuery) {
      showServer(url);
      return;
    }
    final tab = tabs.value.where((b) => b.address.value == url).lastOrNull;
    if (tab == null) {
      open(url);
      return;
    }
    _show(tab);
    tab.view?.retry();
  }

  /// Shows the tabs [hide] put away, the one that showed, else the last
  /// tab opened, or a new one if it has none.
  void show() {
    if (shown.value != null) return;
    final last = _restore() ?? tabs.value.lastOrNull;
    if (last == null) {
      open();
    } else {
      _show(last);
    }
  }

  /// Puts the tabs away, at their addresses, for [show] to bring back; the
  /// pane closes.
  void hide() {
    if (shown.value case final at?) {
      _hidden = (tabs: tabs.value, shown: at);
      shown.value = null;
    }
  }

  /// Adds the tabs [hide] put away back after the open ones, and gives the
  /// one that showed; null with none put away.
  BrowserTab? _restore() {
    final hidden = _hidden;
    if (hidden == null) return null;
    _hidden = null;
    tabs.value = [...tabs.value, ...hidden.tabs];
    return hidden.shown;
  }

  /// Shows [tab], one of the tabs, adding it last if it isn't.
  void showTab(BrowserTab tab) => _show(tab);

  void _show(BrowserTab tab) {
    if (!tabs.value.contains(tab)) tabs.value = [...tabs.value, tab];
    shown.value = tab;
  }

  /// Closes [tab]. If it showed, the one after it shows, or the last; with
  /// none left the pane closes.
  void close(BrowserTab tab) {
    final at = tabs.value.indexOf(tab);
    if (at < 0) return;
    final rest = [...tabs.value]..removeAt(at);
    if (rest.isEmpty) {
      shown.value = null;
      return;
    }
    tabs.value = rest;
    if (shown.value == tab) shown.value = rest[min(at, rest.length - 1)];
  }
}
