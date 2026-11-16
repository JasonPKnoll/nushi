import 'package:flutter/material.dart';

import 'package:nushi/background_commands.dart';
import 'package:nushi/chat.dart';
import 'package:nushi/daemon.dart';
import 'package:nushi/subagent_pane.dart';
import 'package:nushi/workflow_pane.dart';
import 'package:nushi/workflows.dart';

/// The pane of a background task the chat's pane slot shows ([ChatState.
/// pane], a call's id): a background command's, a workflow's or a
/// subagent's. Keyed by the call: another task's pane is its own. A pane
/// whose call went (a rewind, a reload without it) stays the kind it was
/// and says so.
Widget taskPane({
  required ChatState chat,
  required Daemon daemon,
  required String id,
  required VoidCallback onClose,
}) => switch (_kindOf(chat, id)) {
  _Kind.command => BackgroundCommandPane(
    key: ValueKey(id),
    chat: chat,
    daemon: daemon,
    id: id,
    onClose: onClose,
  ),
  _Kind.workflow => WorkflowPane(
    key: ValueKey(id),
    chat: chat,
    daemon: daemon,
    id: id,
    onClose: onClose,
  ),
  _Kind.subagent => SubagentPane(
    key: ValueKey(id),
    chat: chat,
    daemon: daemon,
    id: id,
    onClose: onClose,
  ),
};

/// Whether the pane slot shows a workflow's pane: it has a width of its
/// own, wider than a subagent's (ADR 0127).
bool workflowPaneOpen(ChatState chat, String id) =>
    _kindOf(chat, id) == _Kind.workflow;

enum _Kind { command, workflow, subagent }

/// The kind of the last pane each chat showed, by its call's id: what a
/// call that went was.
final _shown = Expando<(String, _Kind)>('pane kind');

/// The kind of call [id]'s pane: by the call, or, once it went, by what it
/// was when its pane last showed.
_Kind _kindOf(ChatState chat, String id) {
  final call = chat.withRunning.where((it) => it.id == id).firstOrNull;
  if (call == null) {
    if (_shown[chat] case (final was, final kind) when was == id) return kind;
    return _Kind.subagent;
  }
  final kind = call.hasBackgroundCommand()
      ? _Kind.command
      : isWorkflow(call)
      ? _Kind.workflow
      : _Kind.subagent;
  _shown[chat] = (id, kind);
  return kind;
}
