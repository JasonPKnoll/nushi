import 'dart:async';

import 'package:flutter/widgets.dart';

import 'package:nushi/app_actions.dart' show fileTreeKey;
import 'package:nushi/chat.dart';
import 'package:nushi/daemon.dart';
import 'package:nushi/folder_tree.dart';
import 'package:nushi/keys.dart';
import 'package:nushi/menu.dart';
import 'package:nushi/open_file.dart';
import 'package:nushi/settings.dart';
import 'package:nushi/view_options.dart';

/// The Files pane's ⋯ (ADR 0123), Claude desktop's rows in its order and
/// groups: Go to file or search (⌘P's box), Show file tree, Collapse all
/// folders, then how a source shows. Each option's row reads and flips its
/// one pref ([viewToggles]). [folded] are the second row's file actions
/// where it has no room for them, last after a line, so Claude's rows keep
/// their places.
List<MenuEntry> filesMenu(
  BuildContext context,
  ChatState chat,
  Daemon daemon, {
  List<MenuEntry> folded = const [],
}) {
  final tree = daemon.daemonApi >= filesApi;
  return [
    MenuEntry(
      'Go to file or search',
      shortcut: keyLabel(openFileKey),
      onPick: () => unawaited(openFile(context, daemon)),
    ),
    MenuEntry.divider,
    MenuEntry(
      'Show file tree',
      checked: tree && filesTree.value,
      shortcut: keyLabel(fileTreeKey),
      onPick: tree ? () => toggleFileTree(chat) : null,
    ),
    MenuEntry.divider,
    MenuEntry(
      'Collapse all folders',
      onPick: canCollapseFolders(chat, daemon)
          ? folderTreeOf(chat).collapseAll
          : null,
    ),
    MenuEntry.divider,
    for (final o in viewToggles)
      MenuEntry(o.label, checked: o.pref.value, onPick: o.toggle),
    if (folded.isNotEmpty) ...[MenuEntry.divider, ...folded],
  ];
}
