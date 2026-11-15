import 'package:nushi/beside_fit.dart';
import 'package:nushi/chat_panes.dart';
import 'package:nushi/chat_view.dart';
import 'package:nushi/details_panel.dart';

export 'panes_app.dart';

/// A set of panes open beside a chat, and where its room (the chat area,
/// A) holds the details rail beside them at their default widths, holds
/// them at those widths beside the transcript (narrower, they narrow), and
/// holds them at their least beside it (narrower, they lie over it): ADR
/// 0123's table. The numbers are written out, so a changed constant fails
/// loudly ([railFrom], [narrowFrom] and [besideFrom] work them out from the
/// constants).
typedef ThresholdRow = ({
  String name,
  List<PaneSpec> panes,
  double rail,
  double? narrow,
  double? beside,
});

const PaneSpec _side = ChatPanes.side;
const PaneSpec _files = ChatPanes.files;
const PaneSpec _sub = ChatPanes.subagent;
const PaneSpec _preview = ChatPanes.preview;

/// Every set of panes a chat can have open, left to right.
const List<ThresholdRow> thresholdRows = [
  (name: 'none', panes: [], rail: 804, narrow: null, beside: null),
  (name: 'side', panes: [_side], rail: 1184, narrow: 936, beside: 856),
  (name: 'files', panes: [_files], rail: 1064, narrow: 816, beside: 756),
  (
    name: 'side+files',
    panes: [_side, _files],
    rail: 1444,
    narrow: 1196,
    beside: 1056,
  ),
  (name: 'subagent', panes: [_sub], rail: 944, narrow: 624, beside: 580),
  (
    name: 'side+subagent',
    panes: [_side, _sub],
    rail: 1324,
    narrow: 1004,
    beside: 880,
  ),
  (
    name: 'files+subagent',
    panes: [_files, _sub],
    rail: 1204,
    narrow: 884,
    beside: 780,
  ),
  (
    name: 'side+files+subagent',
    panes: [_side, _files, _sub],
    rail: 1584,
    narrow: 1264,
    beside: 1080,
  ),
  (name: 'preview', panes: [_preview], rail: 1240, narrow: 920, beside: 660),
  (
    name: 'side+preview',
    panes: [_side, _preview],
    rail: 1620,
    narrow: 1300,
    beside: 960,
  ),
  (
    name: 'files+preview',
    panes: [_files, _preview],
    rail: 1500,
    narrow: 1180,
    beside: 860,
  ),
  (
    name: 'side+files+preview',
    panes: [_side, _files, _preview],
    rail: 1880,
    narrow: 1560,
    beside: 1160,
  ),
];

bool _wide(List<PaneSpec> panes) => panes.any((p) => p.wide);

double _sum(List<PaneSpec> panes) => panes.fold(0.0, (w, p) => w + p.initial);

double _leastSum(List<PaneSpec> panes) => panes.fold(0.0, (w, p) => w + p.min);

/// The least the transcript keeps beside [panes].
double leastBeside(List<PaneSpec> panes) =>
    _wide(panes) ? ChatView.leastBesideWide : ChatView.minWidth;

/// The least chat area that holds the rail beside [panes].
double railFrom(List<PaneSpec> panes) =>
    _sum(panes) +
    DetailsPanel.width +
    (_wide(panes) ? ChatView.leastBesideWide : ChatView.leastBesideDetails);

/// The least chat area that holds [panes] at their default widths beside
/// the transcript, or null for none: narrower, they narrow.
double? narrowFrom(List<PaneSpec> panes) =>
    panes.isEmpty ? null : _sum(panes) + leastBeside(panes);

/// The least chat area that holds [panes] beside the transcript, each at
/// its least, or null for none: narrower, they lie over it.
double? besideFrom(List<PaneSpec> panes) =>
    panes.isEmpty ? null : _leastSum(panes) + leastBeside(panes);

/// The window width whose chat area is [area]: the sidebar (260 by
/// default) and the window's margins, 8 beside it and 8 at the right, or
/// 8 at each side with the sidebar hidden.
double windowFor(double area, {required bool sidebar, double width = 260}) =>
    area + (sidebar ? width + 8 : 16);
