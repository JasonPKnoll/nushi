import 'package:flutter/material.dart';
import 'package:nushi/beside.dart';
import 'package:nushi/changes_view.dart';
import 'package:nushi/chat.dart';
import 'package:nushi/chat_column.dart';
import 'package:nushi/chat_header.dart';
import 'package:nushi/chat_panes.dart';
import 'package:nushi/chat_view.dart';
import 'package:nushi/daemon.dart';
import 'package:nushi/details_panel.dart';
import 'package:nushi/panes.dart' show Pane;
import 'package:nushi/preview.dart';
import 'package:nushi/preview_entry.dart';
import 'package:nushi/pull_link.dart';
import 'package:nushi/pull_tab.dart';
import 'package:nushi/settings.dart';
import 'package:nushi/terminal_panel.dart';
import 'package:nushi/type_to_box.dart';

/// A chat in the main area (ADR 0111, 0123): one pane of its transcript, its
/// folder's changes or its pull request, its details rail at the
/// transcript's right, its header over the two and its terminal under
/// them, and right of all of those its side panes ([chatPanes]), each the
/// pane's full height. The header and its buttons stay where they are as
/// the details show and hide; the rail tucks itself away when the panes
/// need its room. All of it is laid out by one [Beside], which keeps the
/// header and the terminal as the tabs change.
class ChatArea extends StatefulWidget {
  const new(this.chat, this.daemon, {super.key});

  final ChatState chat;
  final Daemon daemon;

  @override
  State<ChatArea> createState() => _ChatAreaState();
}

class _ChatAreaState extends State<ChatArea> {
  ValueNotifier<ChatTab> get _tab => widget.chat.tab;

  /// Its pull request, looked for while the chat shows (for the tab).
  late final ChatPull _pull;

  /// The area's width, as last built: what the preview's and the
  /// details' room is worked out from when asked.
  double _width = double.infinity;

  /// The preview's tabs: each file's view is made again as it shows, and
  /// the row moves to the new one with its place kept.
  final GlobalKey<State> _previewTabs = GlobalKey();

  /// Whether the browser tab showing takes the whole area, over the
  /// transcript (its Expand, ADR 0094). It lasts while a browser tab shows
  /// on the Chat tab: a file's tab, the browser hidden, a subagent's pane,
  /// another chat (another area) or another tab put it back.
  final _expanded = ValueNotifier<bool>(false);

  /// Where ⌃` and the keys stay when what had the focus goes.
  final _keys = FocusNode(skipTraversal: true);

  @override
  void initState() {
    super.initState();
    // A chat opens on the tab it was left on (ChatState.tab), but on its
    // transcript when it needs you: what it asks is there.
    if (widget.chat.phase == ChatPhase.waiting) _tab.value = ChatTab.chat;
    widget.chat.addListener(_showFile);
    widget.chat.preview.addListener(_showPreview);
    widget.chat.preview.addListener(_collapse);
    widget.chat.pane.addListener(_collapse);
    widget.chat.showAsked.addListener(_showAsked);
    _tab.addListener(_showAsked);
    _tab.addListener(_collapse);
    _terminalOpen = terminalChats.value.contains(widget.chat.id);
    terminalChats.addListener(_terminalShown);
    // After this frame: the panes beside the transcript aren't built yet.
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) _showAsked();
    });
    _pull = ChatPull.of(widget.daemon, widget.chat)..follow();
    // Where the focus goes when what had it goes (the terminal hidden, the
    // details tucked away): typing there still writes in the box (ADR 0121).
    typesThrough.add(_keys);
  }

  @override
  void dispose() {
    widget.chat.removeListener(_showFile);
    widget.chat.preview.removeListener(_showPreview);
    widget.chat.preview.removeListener(_collapse);
    widget.chat.pane.removeListener(_collapse);
    widget.chat.showAsked.removeListener(_showAsked);
    _tab.removeListener(_showAsked);
    _tab.removeListener(_collapse);
    terminalChats.removeListener(_terminalShown);
    // Gone, it has no room to say; the chat shows nowhere until it's back.
    if (widget.chat.previewFits == _previewFits) {
      widget.chat.previewFits = ChatState.fitsAlways;
    }
    if (widget.chat.panelFits == _panelFits) {
      widget.chat.panelFits = ChatState.fitsAlways;
    }
    _pull.unfollow();
    _expanded.dispose();
    typesThrough.remove(_keys);
    _keys.dispose();
    super.dispose();
  }

  /// Whether a browser tab shows beside the transcript, so it can expand.
  bool get _browsing =>
      widget.chat.shownTab == ChatTab.chat &&
      widget.chat.pane.value == null &&
      widget.chat.preview.value is BrowserTab;

  /// What shows is no longer a browser tab on the Chat tab: it's put back.
  void _collapse() {
    if (!_browsing) _expanded.value = false;
  }

  /// Whether its chat's terminal was open, as last heard.
  late bool _terminalOpen;

  /// Its chat's terminal opened, however (⌃`, the palette, a command's
  /// Open in terminal): over an expanded browser tab, which covers it, the
  /// tab is put back, or the terminal would open where nothing shows it.
  void _terminalShown() {
    final open = terminalChats.value.contains(widget.chat.id);
    if (open && !_terminalOpen) _expanded.value = false;
    _terminalOpen = open;
  }

  /// ⌃`: the terminal shows or hides. Over an expanded browser tab, which
  /// covers it, it only shows: the tab is put back, and a terminal open
  /// under it is shown, not closed unseen ([_terminalShown] puts the tab
  /// back for one that opens).
  void _toggleTerminal() {
    if (_expanded.value && terminalChats.value.contains(widget.chat.id)) {
      _expanded.value = false;
      return;
    }
    toggleTerminalOf(widget.chat, _keys);
  }

  /// A file's preview opened (from Artifacts, or ⌘P) shows beside the chat.
  void _showPreview() {
    if (widget.chat.preview.value != null) _tab.value = ChatTab.chat;
  }

  /// A file or page its agent asked to show opens while the transcript
  /// shows. This area lives only while its chat is the one showing, so
  /// another chat or a view in its place leaves the ask waiting, and on
  /// Changes it waits too: the agent never moves the user (ADR 0068). In a
  /// window too narrow for the pane beside the transcript it stays closed,
  /// or it would lie over the box and any card waiting; the row's Preview
  /// or Open opens it.
  void _showAsked() {
    final chat = widget.chat;
    final asked = chat.showAsked.value;
    if (asked == null || chat.shownTab != ChatTab.chat) return;
    chat.showAsked.value = null;
    if (!_previewFits()) return;
    switch (asked) {
      case ShowFile(:final path):
        chat.openPreview(path);
      case ShowPage(:final url):
        chat.showPage(url);
    }
  }

  /// Whether the preview opens beside the transcript and leaves it
  /// [ChatView.leastBesideWide], as [Beside] works it out ([chatFit]): the
  /// side chat and the Files pane take their widths (a subagent's pane
  /// doesn't: the preview takes its place), and the details rail tucks
  /// itself away for it. It is read when asked, since a side chat or the
  /// Files pane opens without building this area again.
  bool _previewFits() => !chatFit(
    room: _width,
    panes: openPanes(widget.chat, widget.daemon, preview: true),
    saved: paneWidths.value,
    wantRail: false,
  ).over;

  /// Whether the details rail has room to show beside the transcript and
  /// the panes open now, at their saved widths: the window is wide enough
  /// for it (ADR 0092), and the panes leave it room (ADR 0123).
  bool _panelFits() =>
      _width >= _railFrom &&
      chatFit(
        room: _width,
        panes: openPanes(widget.chat, widget.daemon),
        saved: paneWidths.value,
        wantRail: true,
      ).rail;

  /// The narrowest chat area with room for the details at all (ADR 0092).
  static const double _railFrom =
      DetailsPanel.width + ChatView.leastBesideDetails;

  /// A file to show (ChatState.showFile, from a review's finding) shows in
  /// the Changes view.
  void _showFile() {
    if (widget.chat.openPath != null) _tab.value = ChatTab.changes;
  }

  @override
  Widget build(BuildContext context) => LayoutBuilder(
    builder: (context, box) {
      _width = box.maxWidth;
      final chat = widget.chat;
      return Pane(
        child: TerminalKeys(
          focus: _keys,
          onToggle: _toggleTerminal,
          child: ListenableBuilder(
            listenable: Listenable.merge([showDetails, _tab]),
            builder: (context, _) {
              final show = showDetails.value;
              // The panel goes when the chat would get too narrow, and
              // its button with it; beside panes that need its room it's
              // tucked away, still wanted (ADR 0123).
              final fits = box.maxWidth >= _railFrom;
              chat
                ..panelFits = _panelFits
                ..previewFits = _previewFits;
              // The pull request's tab goes with its link.
              final tab = chat.shownTab;
              final onChat = tab == ChatTab.chat;
              final panel = show && fits && onChat;
              // Made here, once a change of these, so a pane opening or
              // closing doesn't build them again.
              final head = ListenableBuilder(
                listenable: chat,
                builder: (context, _) => ChatHeader(
                  chat,
                  widget.daemon,
                  tab: chat.shownTab,
                  onTab: (t) => _tab.value = t,
                  panel: panel,
                  onPanel: fits && onChat
                      ? () => showDetails.value = !show
                      : null,
                ),
              );
              final main = switch (tab) {
                ChatTab.changes => ChangesView(
                  chat: chat,
                  daemon: widget.daemon,
                ),
                ChatTab.pull => PullTab(
                  chat: chat,
                  daemon: widget.daemon,
                  onChat: () => _tab.value = ChatTab.chat,
                ),
                ChatTab.chat => ChatView(
                  chat: chat,
                  daemon: widget.daemon,
                  onPull: () => _tab.value = ChatTab.pull,
                ),
              };
              final foot = TerminalFoot(
                chat: chat,
                daemon: widget.daemon,
                onChanges: () => _tab.value = ChatTab.changes,
                onHide: _toggleTerminal,
              );
              final details = panel
                  ? SidePane.fixed(
                      'details',
                      width: DetailsPanel.width,
                      child: DetailsPanel(
                        // It follows the chat it was made for.
                        key: ObjectKey(chat),
                        chat: chat,
                        daemon: widget.daemon,
                        onChanges: () => _tab.value = ChatTab.changes,
                        onPull: () => _tab.value = ChatTab.pull,
                      ),
                    )
                  : null;
              return Previews(
                // Only the transcript's rows open a preview or a tab.
                open: onChat ? chat.openPreview : null,
                browse: onChat ? (url) => chat.openBrowserTab('$url') : null,
                browseFits: () => chat.previewFits(),
                current: chat.preview,
                name: (path) => relativeIn(chat.folder, path),
                child: ListenableBuilder(
                  listenable: Listenable.merge([
                    chat.pane,
                    chat.preview,
                    chat.side,
                    chat.sideHidden,
                    filesChats,
                    _expanded,
                  ]),
                  builder: (context, _) => _beside(
                    context,
                    tab: tab,
                    head: head,
                    main: main,
                    foot: foot,
                    details: details,
                  ),
                ),
              );
            },
          ),
        ),
      );
    },
  );

  /// [main] with the chat's panes and [details] beside it (on the Chat
  /// tab), [head] over them and [foot] under them.
  Widget _beside(
    BuildContext context, {
    required ChatTab tab,
    required Widget head,
    required Widget main,
    required Widget foot,
    required SidePane? details,
  }) {
    final chat = widget.chat;
    final onChat = tab == ChatTab.chat;
    final panes = onChat
        ? chatPanes(
            context,
            chat,
            widget.daemon,
            tabsKey: _previewTabs,
            expanded: _expanded.value && _browsing,
            onExpand: () => _expanded.value = !_expanded.value,
          )
        : const <SidePane>[];
    // Either is dragged as wide as it's wanted.
    final wide = panes.any(
      (p) =>
          p.name == ChatPanes.subagent.name || p.name == ChatPanes.preview.name,
    );
    return Beside(
      scene: tab,
      head: head,
      main: main,
      foot: foot,
      keep: _keys,
      // The text's column glides as a pane opens or closes beside it.
      columnSides: onChat ? (width) => chatSides(context, width) : null,
      // The least the transcript keeps beside the panes, and beside the
      // details too (ADR 0092): narrower, the rail tucks away, and
      // narrower than that the panes lie over it ([chatFit]).
      least: wide ? ChatView.leastBesideWide : ChatView.minWidth,
      railLeast: wide ? ChatView.leastBesideWide : ChatView.leastBesideDetails,
      cover: _expanded.value && _browsing ? 'preview' : null,
      panes: panes,
      details: details,
    );
  }
}
