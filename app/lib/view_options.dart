import 'package:nushi/app_actions.dart';
import 'package:nushi/chat.dart';
import 'package:nushi/daemon.dart';
import 'package:nushi/files_open.dart';
import 'package:nushi/folder_tree.dart';
import 'package:nushi/settings.dart';

// The Files pane's view options in one table (ADR 0123): each is one pref,
// and the pane's ⋯, the View menu, the palette and Settings › Appearance
// (where one has a row) all show and change it through here.

/// A view option that's on or off: its [pref], the one place it's kept,
/// and what each place that flips it says. The ⋯'s row says [label]
/// (Claude desktop's words), checked while it's on; the View menu's item
/// ([viewOn] while it's on, [viewOff] while off) and the palette's row
/// ([paletteOn], [paletteOff]) say what picking them does.
final class ViewToggle {
  const new({
    required this.act,
    required this.pref,
    required this.label,
    required this.viewOn,
    required this.viewOff,
    required this.paletteOn,
    required this.paletteOff,
  });

  final Act act;
  final Pref<bool> pref;
  final String label;
  final String viewOn;
  final String viewOff;
  final String paletteOn;
  final String paletteOff;

  /// The View menu's label as it is now.
  String get viewLabel => pref.value ? viewOn : viewOff;

  /// The palette's title as it is now.
  String get paletteTitle => pref.value ? paletteOn : paletteOff;

  /// Turns it on, or off.
  void toggle() => pref.value = !pref.value;
}

/// Word wrap: a source's long lines wrap, or run on (⌥Z in the viewer).
final wrapOption = ViewToggle(
  act: Act.wrap,
  pref: wrapLines,
  label: 'Word wrap',
  viewOn: 'Don’t Wrap Lines',
  viewOff: 'Wrap Lines',
  paletteOn: 'Don’t wrap lines in Files',
  paletteOff: 'Wrap lines in Files',
);

/// Show whitespace: a source's spaces and tabs marked (ADR 0096).
final whitespaceOption = ViewToggle(
  act: Act.whitespace,
  pref: showWhitespace,
  label: 'Show whitespace',
  viewOn: 'Hide Whitespace',
  viewOff: 'Show Whitespace',
  paletteOn: 'Hide whitespace in Files',
  paletteOff: 'Show whitespace in Files',
);

/// The view options that are on or off, in the ⋯'s order.
final List<ViewToggle> viewToggles = [wrapOption, whitespaceOption];

/// Shows [chat]'s Files pane with its tree, or, while the pane shows, the
/// tree or not ([filesTree]): ⇧⌘Y, the pane's button, its ⋯, View › Show
/// File Tree and the palette. From the Changes tab it's the Chat tab's.
void toggleFileTree(ChatState chat) {
  if (filesShown(chat)) {
    filesTree.value = !filesTree.value;
    return;
  }
  showOrHideFiles(chat);
  filesTree.value = true;
}

/// Whether [chat]'s Files pane shows its tree: the pane shows, the user
/// has it on, and the daemon lists folders.
bool fileTreeShown(ChatState chat, Daemon daemon) =>
    filesShown(chat) && filesTree.value && daemon.daemonApi >= filesApi;

/// Whether Collapse all folders has something to do in [chat]: its tree
/// shows with a folder open.
bool canCollapseFolders(ChatState chat, Daemon daemon) =>
    fileTreeShown(chat, daemon) && folderTreeOf(chat).anyOpen;

/// The View menu's label for [act] as it is now, [chat] being the one
/// shown; null for an action that isn't a view option's.
String? viewMenuLabel(Act act, ChatState? chat) {
  if (act == Act.fileTree) {
    final shown = chat != null && filesOpen(chat) && filesTree.value;
    return shown ? 'Hide File Tree' : 'Show File Tree';
  }
  for (final o in viewToggles) {
    if (o.act == act) return o.viewLabel;
  }
  return null;
}

/// The palette's rows of the view options, [shown] the chat that shows.
List<AppAction> viewActions(ChatState? shown, Daemon d) => [
  AppAction(
    .fileTree,
    shown != null && fileTreeShown(shown, d)
        ? 'Hide the file tree'
        : 'Show the file tree',
    enabled: shown != null && d.daemonApi >= filesApi,
    run: () => toggleFileTree(shown!),
  ),
  AppAction(
    .collapseFolders,
    'Collapse all folders',
    enabled: shown != null && canCollapseFolders(shown, d),
    run: () => folderTreeOf(shown!).collapseAll(),
  ),
  for (final o in viewToggles)
    AppAction(
      o.act,
      o.paletteTitle,
      // Whitespace where a chat shows, as its files do; wrapping always,
      // as it was.
      enabled: o.act == Act.wrap || shown != null,
      run: o.toggle,
    ),
];
