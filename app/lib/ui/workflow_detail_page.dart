import 'dart:async';

import 'package:flutter/material.dart';

import '../auth/session.dart';
import '../chat/chat_api.dart';
import '../chat/models.dart';
import '../workflow/models.dart';
import '../workflow/workflow_api.dart';
import 'node_diff_page.dart';
import 'node_logs_page.dart';
import 'node_review_page.dart';
import 'node_status_chip.dart';

/// Workflow detail: nodes, policy controls, Start / Continue (M10-P02).
class WorkflowDetailPage extends StatefulWidget {
  const WorkflowDetailPage({
    super.key,
    required this.session,
    required this.workflowId,
    this.api,
    this.chatApi,
    this.initial,
    this.pollInterval = const Duration(seconds: 3),
  });

  final Session session;
  final String workflowId;
  final WorkflowApi? api;
  final ChatApi? chatApi;
  final WorkflowRun? initial;
  final Duration pollInterval;

  @override
  State<WorkflowDetailPage> createState() => _WorkflowDetailPageState();
}

class _WorkflowDetailPageState extends State<WorkflowDetailPage> {
  late final WorkflowApi _api =
      widget.api ?? WorkflowApi(session: widget.session);
  late final ChatApi _chatApi =
      widget.chatApi ?? ChatApi(session: widget.session);

  WorkflowRun? _run;
  ModelCatalog _catalog = ModelCatalog.fallback;
  String? _error;
  bool _loading = true;
  bool _busy = false;
  String? _patchingNodeId;
  Timer? _poll;

  @override
  void initState() {
    super.initState();
    _run = widget.initial;
    _loading = widget.initial == null;
    unawaited(_loadModels());
    _refresh();
    _poll = Timer.periodic(widget.pollInterval, (_) {
      if (!mounted || _busy) return;
      _refresh(silent: true);
    });
  }

  @override
  void dispose() {
    _poll?.cancel();
    super.dispose();
  }

  Future<void> _loadModels() async {
    try {
      final c = await _chatApi.listModels();
      if (!mounted) return;
      setState(() => _catalog = c);
    } catch (_) {
      // Keep fallback Auto list.
    }
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
    if (_busy) return;
    setState(() => _busy = true);
    try {
      final result = await _api.startWorkflow(widget.workflowId);
      if (!mounted) return;
      setState(() {
        _run = result.workflow;
        _busy = false;
      });
      final msg = result.delivered
          ? 'Started — delivered to Slave'
          : 'Started — Slave offline (queued when online)';
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(msg)));
    } on WorkflowApiException catch (e) {
      if (!mounted) return;
      setState(() => _busy = false);
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(e.message)),
      );
    } catch (_) {
      if (!mounted) return;
      setState(() => _busy = false);
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Start failed')),
      );
    }
  }

  Future<void> _continue() async {
    if (_busy) return;
    setState(() => _busy = true);
    try {
      final result = await _api.continueWorkflow(widget.workflowId);
      if (!mounted) return;
      setState(() {
        _run = result.workflow;
        _busy = false;
      });
      final msg = result.delivered
          ? 'Continue — delivered to Slave'
          : 'Continue — Slave offline';
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(msg)));
    } on WorkflowApiException catch (e) {
      if (!mounted) return;
      setState(() => _busy = false);
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(e.message)),
      );
    } catch (_) {
      if (!mounted) return;
      setState(() => _busy = false);
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Continue failed')),
      );
    }
  }

  Future<void> _startNode(String nodeId) async {
    if (_busy) return;
    setState(() => _busy = true);
    try {
      final result = await _api.startNode(widget.workflowId, nodeId);
      if (!mounted) return;
      setState(() {
        _run = result.workflow;
        _busy = false;
      });
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(
            result.delivered
                ? 'Node $nodeId started'
                : 'Node $nodeId — Slave offline',
          ),
        ),
      );
    } on WorkflowApiException catch (e) {
      if (!mounted) return;
      setState(() => _busy = false);
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(e.message)),
      );
    } catch (_) {
      if (!mounted) return;
      setState(() => _busy = false);
    }
  }

  Future<void> _patchPolicy(WorkflowNode node, {String? model, NodePolicy? policy}) async {
    if (_patchingNodeId != null) return;
    setState(() => _patchingNodeId = node.id);
    try {
      final run = await _api.patchNode(
        widget.workflowId,
        node.id,
        model: model,
        policy: policy,
      );
      if (!mounted) return;
      setState(() {
        _run = run;
        _patchingNodeId = null;
      });
    } on WorkflowApiException catch (e) {
      if (!mounted) return;
      setState(() => _patchingNodeId = null);
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(e.message)),
      );
      await _refresh(silent: true);
    } catch (_) {
      if (!mounted) return;
      setState(() => _patchingNodeId = null);
      await _refresh(silent: true);
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
            onPressed: _loading || _busy ? null : () => _refresh(),
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
                child: Row(
                  children: [
                    if (run.canStart)
                      Expanded(
                        child: FilledButton.icon(
                          onPressed: _busy ? null : _start,
                          icon: _busy
                              ? const SizedBox(
                                  width: 18,
                                  height: 18,
                                  child: CircularProgressIndicator(
                                    strokeWidth: 2,
                                  ),
                                )
                              : const Icon(Icons.play_arrow),
                          label: Text(_busy ? 'Starting…' : 'Start'),
                        ),
                      ),
                    if (run.canContinue) ...[
                      if (run.canStart) const SizedBox(width: 8),
                      Expanded(
                        child: FilledButton.icon(
                          onPressed: _busy ? null : _continue,
                          icon: const Icon(Icons.skip_next),
                          label: Text(_busy ? '…' : 'Continue'),
                        ),
                      ),
                    ],
                    if (!run.canStart && !run.canContinue)
                      Expanded(
                        child: FilledButton.icon(
                          onPressed: null,
                          icon: const Icon(Icons.play_arrow),
                          label: Text(
                            run.isTerminal
                                ? 'Terminal (${run.status})'
                                : '已下发 (${run.status})',
                          ),
                        ),
                      ),
                  ],
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
          const SizedBox(height: 4),
          Text(
            'Model + switches PATCH to Gateway (defaults off). '
            'Ready nodes: Start; after approve without auto-next: Continue.',
            style: Theme.of(context).textTheme.bodySmall?.copyWith(
                  color: Theme.of(context).colorScheme.onSurfaceVariant,
                ),
          ),
          const SizedBox(height: 8),
          ...run.nodes.map(
            (n) => _NodeTile(
              node: n,
              catalog: _catalog,
              patching: _patchingNodeId == n.id,
              busy: _busy,
              onModelChanged: n.policyEditable
                  ? (model) => unawaited(_patchPolicy(n, model: model))
                  : null,
              onAutoApproveChanged: n.policyEditable
                  ? (v) => unawaited(
                        _patchPolicy(
                          n,
                          policy: n.policy.copyWith(autoApprove: v),
                        ),
                      )
                  : null,
              onAutoStartNextChanged: n.policyEditable
                  ? (v) => unawaited(
                        _patchPolicy(
                          n,
                          policy: n.policy.copyWith(autoStartNext: v),
                        ),
                      )
                  : null,
              onStartNode: n.isReady && !_busy
                  ? () => unawaited(_startNode(n.id))
                  : null,
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
        ],
      ),
    );
  }
}

class _NodeTile extends StatelessWidget {
  const _NodeTile({
    required this.node,
    required this.catalog,
    required this.patching,
    required this.busy,
    this.onModelChanged,
    this.onAutoApproveChanged,
    this.onAutoStartNextChanged,
    this.onStartNode,
    this.onOpenLogs,
    this.onOpenDiff,
    this.onOpenReview,
  });

  final WorkflowNode node;
  final ModelCatalog catalog;
  final bool patching;
  final bool busy;
  final ValueChanged<String>? onModelChanged;
  final ValueChanged<bool>? onAutoApproveChanged;
  final ValueChanged<bool>? onAutoStartNextChanged;
  final VoidCallback? onStartNode;
  final VoidCallback? onOpenLogs;
  final VoidCallback? onOpenDiff;
  final VoidCallback? onOpenReview;

  String _labelFor(String id) {
    for (final m in catalog.models) {
      if (m.id == id) return m.label;
    }
    return id;
  }

  @override
  Widget build(BuildContext context) {
    final awaiting = node.isAwaitingReview;
    final scheme = Theme.of(context).colorScheme;
    final modelIds = catalog.models.map((m) => m.id).toList();
    if (!modelIds.contains(node.model)) {
      modelIds.insert(0, node.model);
    }
    final editable = onModelChanged != null && !patching && !busy;

    return Card(
      elevation: awaiting ? 2 : 0,
      color: awaiting ? scheme.tertiaryContainer.withValues(alpha: 0.55) : null,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(12),
        side: awaiting
            ? BorderSide(color: scheme.tertiary, width: 1.5)
            : BorderSide(color: scheme.outlineVariant),
      ),
      child: Padding(
        padding: const EdgeInsets.fromLTRB(12, 8, 8, 10),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        node.title?.isNotEmpty == true ? node.title! : node.id,
                        style: TextStyle(
                          fontWeight:
                              awaiting ? FontWeight.w700 : FontWeight.w500,
                          fontSize: 15,
                        ),
                      ),
                      if (node.title != null && node.title!.isNotEmpty)
                        Text(
                          node.id,
                          style: Theme.of(context).textTheme.bodySmall,
                        ),
                      if (node.dependsOn.isNotEmpty)
                        Text(
                          'depends: ${node.dependsOn.join(', ')}',
                          style: Theme.of(context).textTheme.bodySmall,
                        ),
                    ],
                  ),
                ),
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
            const SizedBox(height: 6),
            Wrap(
              crossAxisAlignment: WrapCrossAlignment.center,
              spacing: 8,
              runSpacing: 4,
              children: [
                DropdownButtonHideUnderline(
                  child: DropdownButton<String>(
                    key: ValueKey('model-${node.id}'),
                    value: modelIds.contains(node.model)
                        ? node.model
                        : modelIds.first,
                    isDense: true,
                    items: [
                      for (final id in modelIds)
                        DropdownMenuItem(
                          value: id,
                          child: Text(_labelFor(id)),
                        ),
                    ],
                    onChanged: editable
                        ? (v) {
                            if (v != null) onModelChanged!(v);
                          }
                        : null,
                  ),
                ),
                Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text('自动通过', style: Theme.of(context).textTheme.bodySmall),
                    Switch(
                      key: ValueKey('autoApprove-${node.id}'),
                      value: node.policy.autoApprove,
                      onChanged: editable ? onAutoApproveChanged : null,
                    ),
                  ],
                ),
                Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text('自动下个', style: Theme.of(context).textTheme.bodySmall),
                    Switch(
                      key: ValueKey('autoStartNext-${node.id}'),
                      value: node.policy.autoStartNext,
                      onChanged: editable ? onAutoStartNextChanged : null,
                    ),
                  ],
                ),
                if (onStartNode != null)
                  FilledButton.tonalIcon(
                    key: ValueKey('startNode-${node.id}'),
                    onPressed: onStartNode,
                    icon: const Icon(Icons.play_arrow, size: 18),
                    label: const Text('开始'),
                  ),
                if (patching)
                  const SizedBox(
                    width: 16,
                    height: 16,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}
