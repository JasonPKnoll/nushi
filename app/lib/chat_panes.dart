import 'dart:async';

import 'package:flutter/widgets.dart';
import 'package:nushi/background_commands.dart';
import 'package:nushi/beside.dart';
import 'package:nushi/beside_fit.dart';
import 'package:nushi/browser_pane.dart';
import 'package:nushi/chat.dart';
import 'package:nushi/chat_view.dart';
import 'package:nushi/daemon.dart';
import 'package:nushi/details_panel.dart';
import 'package:nushi/edit_bars.dart';
import 'package:nushi/file_view.dart';
import 'package:nushi/files_pane.dart';
import 'package:nushi/folder_tree.dart' show filesApi;
import 'package:nushi/fork_chat.dart';
import 'package:nushi/side_chat.dart';
import 'package:nushi/subagent_pane.dart';

/// The panes that open beside a chat's transcript, left to right, each at
/// the width it opens at until dragged and its least and most: the side
/// chat, the Files pane, the open files' view, a subagent's or a
/// background command's pane (named as the old Wider one was, so a width
/// dragged then stays), and the browser, in that one's place.
abstract final class ChatPanes {
  static const side = PaneSpec('side', initial: 380, min: 300, cap: 640);
  static const files = PaneSpec('files', initial: 260, min: 200, cap: 480);
  static const fileView = PaneSpec(
    'fileView',
    initial: 560,
    min: 300,
    cap: double.infinity,
    wide: true,
  );

  /// It opens at the details panel's width before that took its own (ADR
  /// 0092).
  static const subagent = PaneSpec(
    'subagent',
    initial: 264,
    min: 220,
    cap: double.infinity,
    wide: true,
  );
  static const browser = PaneSpec(
    'browser',
    initial: 560,
    min: 300,
    cap: double.infinity,
    wide: true,
  );
}

/// The panes open beside [chat]'s transcript, left to right, as the chat
/// view lays them out: a hidden side chat, or an archived chat's, isn't
/// placed, and a subagent's or a command's pane takes the browser's place.
/// [fileView] and [browser] count that pane as open whatever it shows (the
/// room it would take, when the agent asks for one).
List<PaneSpec> openPanes(
  ChatState chat,
  Daemon daemon, {
  bool fileView = false,
  bool browser = false,
}) {
  final pane = chat.pane.value != null;
  return [
    if (chat.side.value != null && !chat.archived && !chat.sideHidden.value)
      ChatPanes.side,
    if (filesOpen(chat) && daemon.daemonApi >= filesApi) ChatPanes.files,
    if (fileView || chat.files.shown.value != null) ChatPanes.fileView,
    if (pane && !browser) ChatPanes.subagent,
    if (browser || !pane && chat.browser.shown.value != null) ChatPanes.browser,
  ];
}

/// [besideFit] for a chat's room with [panes] open: beside a pane dragged
/// as wide as it's wanted (a subagent's, the preview) the transcript keeps
/// [ChatView.leastBesideWide]; beside the rest, [ChatView.minWidth], and
/// beside the details rail [ChatView.leastBesideDetails] (ADR 0092).
BesideFit chatFit({
  required double room,
  required List<PaneSpec> panes,
  required Map<String, double> saved,
  required bool wantRail,
  Map<String, double> live = const {},
}) {
  final wide = panes.any((p) => p.wide);
  return besideFit(
    room: room,
    panes: panes,
    saved: saved,
    live: live,
    wantRail: wantRail,
    least: wide ? ChatView.leastBesideWide : ChatView.minWidth,
    railLeast: wide ? ChatView.leastBesideWide : ChatView.leastBesideDetails,
    rail: DetailsPanel.width,
  );
}

/// The panes open beside [chat]'s transcript, left to right ([openPanes]),
/// each at its own remembered width and closed on its own: the side chat,
/// the Files pane, the open files' view, a subagent's or background
/// command's pane (or the browser in its place). The file view's tab row
/// is [fileTabsKey]'s and the browser's [browserTabsKey]'s, so each
/// outlives each tab's view; the browser is [expanded] over all of it (ADR
/// 0094) and [onExpand] toggles that.
List<SidePane> chatPanes(
  BuildContext context,
  ChatState chat,
  Daemon daemon, {
  required GlobalKey fileTabsKey,
  required GlobalKey browserTabsKey,
  required bool expanded,
  required VoidCallback onExpand,
}) {
  final side = chat.side.value;
  final id = chat.pane.value;
  return [
    for (final spec in openPanes(chat, daemon))
      SidePane.of(
        spec,
        child: switch (spec.name) {
          'side' => SideChat(
            key: ValueKey(side!.id),
            side: side,
            daemon: daemon,
            onHide: () => chat.sideHidden.value = true,
            onClose: () => unawaited(endSide(context, daemon, chat)),
            onKeep: () => unawaited(keepSide(context, daemon, chat)),
          ),
          'files' => FilesPane(
            chat: chat,
            daemon: daemon,
            onClose: () => toggleFiles(chat),
          ),
          'fileView' => FileView(
            key: ValueKey(chat.shownFile),
            chat: chat,
            daemon: daemon,
            path: chat.shownFile!,
            tabsKey: fileTabsKey,
            onClose: () => unawaited(closeFilesSaving(context, chat)),
          ),
          'subagent' => _taskPane(chat, daemon, id!),
          _ => BrowserPane(
            key: ObjectKey(chat.browser.shown.value),
            chat: chat,
            tab: chat.browser.shown.value!,
            daemon: daemon,
            tabsKey: browserTabsKey,
            onClose: () => closeBrowser(chat),
            expanded: expanded,
            onExpand: onExpand,
          ),
        },
      ),
  ];
}

/// A subagent's or background command's pane. Keyed: another subagent's
/// pane is its own, its unsent text too.
Widget _taskPane(ChatState chat, Daemon daemon, String id) {
  void onClose() => chat.pane.value = null;
  return chat.withRunning.any((it) => it.id == id && it.hasBackgroundCommand())
      ? BackgroundCommandPane(
          key: ValueKey(id),
          chat: chat,
          daemon: daemon,
          id: id,
          onClose: onClose,
        )
      : SubagentPane(
          key: ValueKey(id),
          chat: chat,
          daemon: daemon,
          id: id,
          onClose: onClose,
        );
}
