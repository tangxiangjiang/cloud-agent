import 'dart:async';

import 'package:flutter/material.dart';

import '../auth/session.dart';
import '../workflow/models.dart';
import '../workflow/workflow_api.dart';
import 'node_diff_page.dart';
import 'node_logs_page.dart';
import 'node_review_page.dart';
import 'node_status_chip.dart';

/// Workflow detail: node statuses + Start. Polls Gateway; no diff/revise here.
class WorkflowDetailPage extends StatefulWidget {
  const WorkflowDetailPage({
    super.key,
    required this.session,
    required this.workflowId,
    this.api,
    this.initial,
    this.pollInterval = const Duration(seconds: 3),
  });

  final Session session;
  final String workflowId;
  final WorkflowApi? api;
  final WorkflowRun? initial;
  final Duration pollInterval;

  @override
  State<WorkflowDetailPage> createState() => _WorkflowDetailPageState();
}

class _WorkflowDetailPageState extends State<WorkflowDetailPage> {
  late final WorkflowApi _api =
      widget.api ?? WorkflowApi(session: widget.session);

  WorkflowRun? _run;
  String? _error;
  bool _loading = true;
  bool _starting = false;
  Timer? _poll;

  @override
  void initState() {
    super.initState();
    _run = widget.initial;
    _loading = widget.initial == null;
    _refresh();
    _poll = Timer.periodic(widget.pollInterval, (_) {
      if (!mounted || _starting) return;
      _refresh(silent: true);
    });
  }

  @override
  void dispose() {
    _poll?.cancel();
    super.dispose();
  }

  Future<void> _refresh({bool silent = false}) async {
    if (!silent && _run == null) {
      setState(() {
        _loading = true;
        _error = null;
      });
    }
    try {
      final run = await _api.getWorkflow(widget.workflowId);
      if (!mounted) return;
      setState(() {
        _run = run;
        _error = null;
        _loading = false;
      });
    } on WorkflowApiException catch (e) {
      if (!mounted) return;
      setState(() {
        _error = e.message;
        _loading = false;
      });
      if (!silent) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(e.message)),
        );
      }
    } catch (_) {
      if (!mounted) return;
      setState(() {
        _error = 'Failed to load workflow';
        _loading = false;
      });
    }
  }

  Future<void> _start() async {
    if (_starting) return;
    setState(() => _starting = true);
    try {
      final result = await _api.startWorkflow(widget.workflowId);
      if (!mounted) return;
      setState(() {
        _run = result.workflow;
        _starting = false;
      });
      final msg = result.delivered
          ? 'Started — delivered to Slave'
          : 'Started — Slave offline (queued when online)';
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(msg)));
    } on WorkflowApiException catch (e) {
      if (!mounted) return;
      setState(() => _starting = false);
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(e.message)),
      );
    } catch (_) {
      if (!mounted) return;
      setState(() => _starting = false);
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Start failed')),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final run = _run;
    return Scaffold(
      appBar: AppBar(
        title: Text(
          widget.workflowId,
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
        ),
        actions: [
          IconButton(
            tooltip: 'Refresh',
            onPressed: _loading || _starting ? null : () => _refresh(),
            icon: const Icon(Icons.refresh),
          ),
        ],
      ),
      body: _buildBody(run),
      bottomNavigationBar: run == null
          ? null
          : SafeArea(
              child: Padding(
                padding: const EdgeInsets.fromLTRB(16, 8, 16, 16),
                child: FilledButton.icon(
                  onPressed: run.canStart && !_starting ? _start : null,
                  icon: _starting
                      ? const SizedBox(
                          width: 18,
                          height: 18,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        )
                      : const Icon(Icons.play_arrow),
                  label: Text(
                    run.canStart ? 'Start' : 'Terminal (${run.status})',
                  ),
                ),
              ),
            ),
    );
  }

  Widget _buildBody(WorkflowRun? run) {
    if (_loading && run == null) {
      return const Center(child: CircularProgressIndicator());
    }
    if (run == null) {
      return ListView(
        padding: const EdgeInsets.all(24),
        children: [
          Text(
            _error ?? 'Not found',
            style: TextStyle(color: Theme.of(context).colorScheme.error),
          ),
          const SizedBox(height: 16),
          FilledButton(onPressed: _refresh, child: const Text('Retry')),
        ],
      );
    }

    return RefreshIndicator(
      onRefresh: () => _refresh(),
      child: ListView(
        padding: const EdgeInsets.fromLTRB(16, 12, 16, 100),
        children: [
          Row(
            children: [
              Expanded(
                child: Text(
                  'Status',
                  style: Theme.of(context).textTheme.titleMedium,
                ),
              ),
              WorkflowStatusChip(status: run.status),
            ],
          ),
          const SizedBox(height: 8),
          Text('bundle: ${run.bundleId}'),
          if (run.repoId != null && run.repoId!.isNotEmpty)
            Text('repo: ${run.repoId}'),
          if (run.slaveId != null && run.slaveId!.isNotEmpty)
            Text('slave: ${run.slaveId}'),
          if (_error != null) ...[
            const SizedBox(height: 8),
            Text(
              _error!,
              style: TextStyle(color: Theme.of(context).colorScheme.error),
            ),
          ],
          const SizedBox(height: 20),
          Text('Nodes', style: Theme.of(context).textTheme.titleMedium),
          const SizedBox(height: 8),
          ...run.nodes.map(
            (n) => _NodeTile(
              node: n,
              onOpenLogs: n.taskId != null && n.taskId!.isNotEmpty
                  ? () {
                      Navigator.of(context).push<void>(
                        MaterialPageRoute(
                          builder: (_) => NodeLogsPage(
                            session: widget.session,
                            taskId: n.taskId!,
                            nodeId: n.id,
                            api: _api,
                          ),
                        ),
                      );
                    }
                  : null,
              onOpenDiff: n.isAwaitingReview
                  ? () {
                      Navigator.of(context).push<void>(
                        MaterialPageRoute(
                          builder: (_) => NodeDiffPage(
                            session: widget.session,
                            workflowId: widget.workflowId,
                            nodeId: n.id,
                            api: _api,
                          ),
                        ),
                      );
                    }
                  : null,
              onOpenReview: n.isAwaitingReview
                  ? () async {
                      final updated = await Navigator.of(context)
                          .push<WorkflowRun>(
                        MaterialPageRoute(
                          builder: (_) => NodeReviewPage(
                            session: widget.session,
                            workflowId: widget.workflowId,
                            node: n,
                            api: _api,
                          ),
                        ),
                      );
                      if (updated != null && mounted) {
                        setState(() => _run = updated);
                      } else if (mounted) {
                        await _refresh(silent: true);
                      }
                    }
                  : null,
            ),
          ),
          const SizedBox(height: 16),
          Text(
            'Logs via WS when taskId is set. Diff + Review when awaiting_review.',
            style: Theme.of(context).textTheme.bodySmall?.copyWith(
                  color: Theme.of(context).colorScheme.onSurfaceVariant,
                ),
          ),
        ],
      ),
    );
  }
}

class _NodeTile extends StatelessWidget {
  const _NodeTile({
    required this.node,
    this.onOpenLogs,
    this.onOpenDiff,
    this.onOpenReview,
  });

  final WorkflowNode node;
  final VoidCallback? onOpenLogs;
  final VoidCallback? onOpenDiff;
  final VoidCallback? onOpenReview;

  @override
  Widget build(BuildContext context) {
    final awaiting = node.isAwaitingReview;
    final scheme = Theme.of(context).colorScheme;
    return Card(
      elevation: awaiting ? 2 : 0,
      color: awaiting ? scheme.tertiaryContainer.withValues(alpha: 0.55) : null,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(12),
        side: awaiting
            ? BorderSide(color: scheme.tertiary, width: 1.5)
            : BorderSide(color: scheme.outlineVariant),
      ),
      child: ListTile(
        title: Text(
          node.title?.isNotEmpty == true ? node.title! : node.id,
          style: TextStyle(
            fontWeight: awaiting ? FontWeight.w700 : FontWeight.w500,
          ),
        ),
        subtitle: Text(
          [
            if (node.title != null && node.title!.isNotEmpty) node.id,
            if (node.dependsOn.isNotEmpty)
              'depends: ${node.dependsOn.join(', ')}',
            if (node.taskId != null) 'task: ${node.taskId}',
          ].where((s) => s.isNotEmpty).join('\n'),
        ),
        isThreeLine: node.dependsOn.isNotEmpty || node.taskId != null,
        trailing: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            if (onOpenReview != null)
              IconButton(
                tooltip: 'Review',
                onPressed: onOpenReview,
                icon: const Icon(Icons.rate_review_outlined),
              ),
            if (onOpenDiff != null)
              IconButton(
                tooltip: 'Diff (read-only)',
                onPressed: onOpenDiff,
                icon: const Icon(Icons.difference_outlined),
              ),
            if (onOpenLogs != null)
              IconButton(
                tooltip: 'Logs',
                onPressed: onOpenLogs,
                icon: const Icon(Icons.terminal),
              ),
            NodeStatusChip(status: node.status),
          ],
        ),
      ),
    );
  }
}
