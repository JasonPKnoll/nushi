import 'package:flutter/material.dart';

import 'package:nushi/background_commands.dart';
import 'package:nushi/chat.dart';
import 'package:nushi/daemon.dart';
import 'package:nushi/subagent_pane.dart';
import 'package:nushi/workflow_pane.dart';
import 'package:nushi/workflows.dart';

/// The pane of a background task the chat's pane slot shows ([ChatState.
/// pane], a call's id): a background command's, a workflow's or a
/// subagent's. Keyed by the call: another task's pane is its own.
Widget taskPane({
  required ChatState chat,
  required Daemon daemon,
  required String id,
  required VoidCallback onClose,
}) {
  final call = chat.withRunning.where((it) => it.id == id).firstOrNull;
  if (call != null && call.hasBackgroundCommand()) {
    return BackgroundCommandPane(
      key: ValueKey(id),
      chat: chat,
      daemon: daemon,
      id: id,
      onClose: onClose,
    );
  }
  if (call != null && isWorkflow(call)) {
    return WorkflowPane(
      key: ValueKey(id),
      chat: chat,
      daemon: daemon,
      id: id,
      onClose: onClose,
    );
  }
  return SubagentPane(
    key: ValueKey(id),
    chat: chat,
    daemon: daemon,
    id: id,
    onClose: onClose,
  );
}

/// Whether the pane slot shows a workflow's pane: it has a width of its
/// own, wider than a subagent's (ADR 0127).
bool workflowPaneOpen(ChatState chat, String id) =>
    chat.withRunning.any((it) => it.id == id && isWorkflow(it));
