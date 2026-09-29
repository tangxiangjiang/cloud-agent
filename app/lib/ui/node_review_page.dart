import 'package:flutter/material.dart';

import '../auth/session.dart';
import '../workflow/models.dart';
import '../workflow/workflow_api.dart';
import 'node_diff_page.dart';
import 'node_logs_page.dart';
import 'node_status_chip.dart';

/// Review gate: revise instruction + approve/reject. Does not write progress.md.
class NodeReviewPage extends StatefulWidget {
  const NodeReviewPage({
    super.key,
    required this.session,
    required this.workflowId,
    required this.node,
    this.api,
  });

  final Session session;
  final String workflowId;
  final WorkflowNode node;
  final WorkflowApi? api;

  @override
  State<NodeReviewPage> createState() => _NodeReviewPageState();
}

class _NodeReviewPageState extends State<NodeReviewPage> {
  late final WorkflowApi _api =
      widget.api ?? WorkflowApi(session: widget.session);
  final _instruction = TextEditingController();
  late WorkflowNode _node = widget.node;
  bool _busy = false;

  bool get _canReview => _node.isAwaitingReview && !_busy;

  @override
  void dispose() {
    _instruction.dispose();
    super.dispose();
  }

  WorkflowNode _nodeFrom(WorkflowRun wf) {
    for (final n in wf.nodes) {
      if (n.id == _node.id) return n;
    }
    return _node;
  }

  Future<void> _revise() async {
    final text = _instruction.text.trim();
    if (text.isEmpty || !_canReview) return;
    setState(() => _busy = true);
    try {
      final wf = await _api.reviseNode(
        widget.workflowId,
        _node.id,
        instruction: text,
      );
      final updated = _nodeFrom(wf);
      if (!mounted) return;
      setState(() {
        _node = updated;
        _busy = false;
      });
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Revise sent — node running')),
      );
      final tid = updated.taskId;
      if (tid != null && tid.isNotEmpty) {
        await Navigator.of(context).push<void>(
          MaterialPageRoute(
            builder: (_) => NodeLogsPage(
              session: widget.session,
              taskId: tid,
              nodeId: updated.id,
              api: _api,
            ),
          ),
        );
      }
      if (mounted) Navigator.of(context).pop(wf);
    } on WorkflowApiException catch (e) {
      if (!mounted) return;
      setState(() => _busy = false);
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(e.message)),
      );
    }
  }

  Future<void> _review(String decision) async {
    if (_busy || !_node.isAwaitingReview) return;
    setState(() => _busy = true);
    try {
      final wf = await _api.reviewNode(
        widget.workflowId,
        _node.id,
        decision: decision,
      );
      if (!mounted) return;
      setState(() => _busy = false);
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(
            decision == 'approve'
                ? 'Approved — progress updated by Slave'
                : 'Rejected — progress unchanged',
          ),
        ),
      );
      Navigator.of(context).pop(wf);
    } on WorkflowApiException catch (e) {
      if (!mounted) return;
      setState(() => _busy = false);
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(e.message)),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: Text('Review · ${_node.id}'),
        actions: [
          IconButton(
            tooltip: 'Diff',
            onPressed: () {
              Navigator.of(context).push<void>(
                MaterialPageRoute(
                  builder: (_) => NodeDiffPage(
                    session: widget.session,
                    workflowId: widget.workflowId,
                    nodeId: _node.id,
                    api: _api,
                  ),
                ),
              );
            },
            icon: const Icon(Icons.difference_outlined),
          ),
        ],
      ),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          Row(
            children: [
              Expanded(
                child: Text(
                  _node.title?.isNotEmpty == true ? _node.title! : _node.id,
                  style: Theme.of(context).textTheme.titleMedium,
                ),
              ),
              NodeStatusChip(status: _node.status),
            ],
          ),
          const SizedBox(height: 16),
          TextField(
            controller: _instruction,
            enabled: _canReview,
            minLines: 3,
            maxLines: 6,
            decoration: const InputDecoration(
              labelText: '修改意见 (revise)',
              hintText: '告诉 Slave 如何改；不会在 App 里改源码',
              border: OutlineInputBorder(),
            ),
            onChanged: (_) => setState(() {}),
          ),
          const SizedBox(height: 12),
          FilledButton.tonalIcon(
            onPressed:
                _canReview && _instruction.text.trim().isNotEmpty ? _revise : null,
            icon: const Icon(Icons.send),
            label: const Text('提交修改意见'),
          ),
          const SizedBox(height: 24),
          FilledButton.icon(
            onPressed: _canReview ? () => _review('approve') : null,
            icon: const Icon(Icons.check),
            label: const Text('通过 (approve · 写 progress + 本地 git commit)'),
          ),
          const SizedBox(height: 8),
          OutlinedButton.icon(
            onPressed: _canReview ? () => _review('reject') : null,
            icon: const Icon(Icons.close),
            label: const Text('驳回 (reject · 不写 progress / 不 commit)'),
          ),
          if (!_node.isAwaitingReview)
            Padding(
              padding: const EdgeInsets.only(top: 16),
              child: Text(
                '仅 awaiting_review 可审核；当前 ${_node.status}',
                style: TextStyle(color: Theme.of(context).colorScheme.error),
              ),
            ),
        ],
      ),
    );
  }
}
