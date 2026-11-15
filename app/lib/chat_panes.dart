import 'package:nushi/beside_fit.dart';
import 'package:nushi/chat.dart';
import 'package:nushi/chat_view.dart';
import 'package:nushi/daemon.dart';
import 'package:nushi/details_panel.dart';
import 'package:nushi/files_pane.dart';
import 'package:nushi/folder_tree.dart' show filesApi;

/// The panes that open beside a chat's transcript, left to right, each at
/// the width it opens at until dragged and its least and most: the side
/// chat, the Files pane, a subagent's or a background command's pane
/// (named as the old Wider one was, so a width dragged then stays), and
/// the preview, a file or a browser tab, in that one's place.
abstract final class ChatPanes {
  static const side = PaneSpec('side', initial: 380, min: 300, cap: 640);
  static const files = PaneSpec('files', initial: 260, min: 200, cap: 480);

  /// It opens at the details panel's width before that took its own (ADR
  /// 0092).
  static const subagent = PaneSpec(
    'subagent',
    initial: 264,
    min: 220,
    cap: double.infinity,
  );
  static const preview = PaneSpec(
    'preview',
    initial: 560,
    min: 300,
    cap: double.infinity,
  );
}

/// The panes open beside [chat]'s transcript, left to right, as the chat
/// view lays them out: a hidden side chat, or an archived chat's, isn't
/// placed, and a subagent's or a command's pane takes the preview's place.
/// [preview] counts the preview as open whatever it shows (the room it
/// would take, when the agent asks for one).
List<PaneSpec> openPanes(ChatState chat, Daemon daemon, {bool? preview}) {
  final pane = chat.pane.value != null;
  return [
    if (chat.side.value != null && !chat.archived && !chat.sideHidden.value)
      ChatPanes.side,
    if (filesOpen(chat) && daemon.daemonApi >= filesApi) ChatPanes.files,
    if (pane && preview == null) ChatPanes.subagent,
    if (preview ?? (!pane && chat.preview.value != null)) ChatPanes.preview,
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
  final wide = panes.any(
    (p) =>
        p.name == ChatPanes.subagent.name || p.name == ChatPanes.preview.name,
  );
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
