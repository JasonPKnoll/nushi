import 'dart:math';

import 'package:flutter/foundation.dart';

import 'package:nushi/chat_paths.dart';
import 'package:nushi/preview_entry.dart';

/// A chat's open files (ADR 0070): their tabs in the order opened and the
/// one that shows, in the app's memory. With none showing the Files pane's
/// viewer is blank (ADR 0123); the tabs outlive the pane hiding. A file
/// opened without keeping it (a click in the tree, with preview tabs on)
/// is [provisional], as VS Code's preview tab: the next such open takes its
/// place, until it's kept.
final class FileTabs {
  /// The tabs, in the order opened.
  final tabs = ValueNotifier<List<PreviewFile>>(const []);

  /// The tab that shows; null with none open.
  final shown = ValueNotifier<PreviewFile?>(null);

  /// The tab the next open without keeping takes the place of, shown in
  /// italics; at most one.
  final provisional = ValueNotifier<PreviewFile?>(null);

  /// The files whose view shows its Source, while the app runs: a file's
  /// view is made again each time it shows.
  final sources = <String>{};

  /// Shows [path]: the tab of the same file if it has one, however each is
  /// named ([relativeIn] in [folder]: relative or absolute, with `.`, `..`
  /// and doubled slashes), kept if [keep]; else a new one, last, or without
  /// [keep] in the [provisional] tab's place, provisional itself.
  void open(String path, {required String folder, bool keep = true}) {
    final file = relativeIn(folder, path);
    final open = tabs.value
        .where((f) => relativeIn(folder, f.path) == file)
        .firstOrNull;
    if (open != null) {
      if (keep) this.keep(open);
      return show(open);
    }
    final tab = PreviewFile(path);
    final old = provisional.value;
    final at = old == null ? -1 : tabs.value.indexOf(old);
    if (!keep && at >= 0) {
      sources.remove(old!.path);
      provisional.value = tab;
      tabs.value = [...tabs.value]..[at] = tab;
      shown.value = tab;
      return;
    }
    show(tab);
    if (!keep) provisional.value = tab;
  }

  /// Keeps [tab]: no later open takes its place.
  void keep(PreviewFile tab) {
    if (provisional.value == tab) provisional.value = null;
  }

  /// Shows [tab], adding it last if it isn't open.
  void show(PreviewFile tab) {
    if (!tabs.value.contains(tab)) tabs.value = [...tabs.value, tab];
    shown.value = tab;
  }

  /// Closes [tab]. If it showed, the one after it shows, or the last; with
  /// none left none shows.
  void close(PreviewFile tab) {
    final at = tabs.value.indexOf(tab);
    if (at < 0) return;
    final rest = [...tabs.value]..removeAt(at);
    keep(tab);
    tabs.value = rest;
    sources.remove(tab.path);
    if (rest.isEmpty) {
      shown.value = null;
    } else if (shown.value == tab) {
      shown.value = rest[min(at, rest.length - 1)];
    }
  }

  /// Closes every tab.
  void closeAll() {
    tabs.value = const [];
    shown.value = null;
    provisional.value = null;
    sources.clear();
  }
}
