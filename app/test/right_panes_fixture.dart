import 'package:nushi/beside_fit.dart';
import 'package:nushi/chat_panes.dart';
import 'package:nushi/chat_view.dart';
import 'package:nushi/details_panel.dart';

export 'panes_app.dart';

/// A set of panes open beside a chat, and where its room (the chat area,
/// A) holds the details rail beside them and holds them beside the
/// transcript, at their default widths (ADR 0123's table). The numbers are
/// written out, so a changed constant fails loudly ([railFrom] and
/// [besideFrom] work them out from the constants).
typedef ThresholdRow = ({
  String name,
  List<PaneSpec> panes,
  double rail,
  double? beside,
});

const PaneSpec _side = ChatPanes.side;
const PaneSpec _files = ChatPanes.files;
const PaneSpec _sub = ChatPanes.subagent;
const PaneSpec _preview = ChatPanes.preview;

/// Every set of panes a chat can have open, left to right.
const List<ThresholdRow> thresholdRows = [
  (name: 'none', panes: [], rail: 804, beside: null),
  (name: 'side', panes: [_side], rail: 1184, beside: 936),
  (name: 'files', panes: [_files], rail: 1064, beside: 816),
  (name: 'side+files', panes: [_side, _files], rail: 1444, beside: 1196),
  (name: 'subagent', panes: [_sub], rail: 944, beside: 624),
  (name: 'side+subagent', panes: [_side, _sub], rail: 1324, beside: 1004),
  (name: 'files+subagent', panes: [_files, _sub], rail: 1204, beside: 884),
  (
    name: 'side+files+subagent',
    panes: [_side, _files, _sub],
    rail: 1584,
    beside: 1264,
  ),
  (name: 'preview', panes: [_preview], rail: 1240, beside: 920),
  (name: 'side+preview', panes: [_side, _preview], rail: 1620, beside: 1300),
  (name: 'files+preview', panes: [_files, _preview], rail: 1500, beside: 1180),
  (
    name: 'side+files+preview',
    panes: [_side, _files, _preview],
    rail: 1880,
    beside: 1560,
  ),
];

bool _wide(List<PaneSpec> panes) =>
    panes.contains(_sub) || panes.contains(_preview);

double _sum(List<PaneSpec> panes) => panes.fold(0.0, (w, p) => w + p.initial);

/// The least chat area that holds the rail beside [panes].
double railFrom(List<PaneSpec> panes) =>
    _sum(panes) +
    DetailsPanel.width +
    (_wide(panes) ? ChatView.leastBesideWide : ChatView.leastBesideDetails);

/// The least chat area that holds [panes] beside the transcript, or null
/// for none.
double? besideFrom(List<PaneSpec> panes) => panes.isEmpty
    ? null
    : _sum(panes) +
          (_wide(panes) ? ChatView.leastBesideWide : ChatView.minWidth);

/// The window width whose chat area is [area]: the sidebar (260 by
/// default) and the window's margins, 8 beside it and 8 at the right, or
/// 8 at each side with the sidebar hidden.
double windowFor(double area, {required bool sidebar, double width = 260}) =>
    area + (sidebar ? width + 8 : 16);
