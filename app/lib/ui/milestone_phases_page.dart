import 'package:flutter/material.dart';

import '../auth/session.dart';
import '../slaves/models.dart';
import '../workflow/models.dart';
import '../workflow/workflow_api.dart';
import 'node_status_chip.dart';
import 'workflow_detail_page.dart';

/// Shows milestone phases; resume existing run or create+start once.
class MilestonePhasesPage extends StatefulWidget {
  const MilestonePhasesPage({
    super.key,
    required this.session,
    required this.slave,
    required this.project,
    required this.milestone,
    this.api,
  });

  final Session session;
  final SlaveInfo slave;
  final ProjectInfo project;
  final MilestoneInfo milestone;
  final WorkflowApi? api;

  @override
  State<MilestonePhasesPage> createState() => _MilestonePhasesPageState();
}

class _MilestonePhasesPageState extends State<MilestonePhasesPage> {
  late final WorkflowApi _api =
      widget.api ?? WorkflowApi(session: widget.session);

  bool _loading = true;
  bool _busy = false;
  String? _error;
  WorkflowRun? _existing;

  String get _bundleId => 'milestone:${widget.milestone.id}';

  @override
  void initState() {
    super.initState();
    _refreshExisting();
  }

  Future<void> _refreshExisting() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final list = await _api.listWorkflows(limit: 100);
      final match = findMilestoneRun(
        list,
        bundleId: _bundleId,
        repoId: widget.project.id,
        slaveId: widget.slave.id,
      );
      if (!mounted) return;
      setState(() {
        _existing = match;
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        _error = e.toString();
      });
    }
  }

  Future<void> _openRun(WorkflowRun run) async {
    await Navigator.of(context).push<void>(
      MaterialPageRoute(
        builder: (_) => WorkflowDetailPage(
          session: widget.session,
          workflowId: run.id,
          initial: run,
        ),
      ),
    );
    if (mounted) await _refreshExisting();
  }

  Future<void> _createAndStart({required bool forceNew}) async {
    final ms = widget.milestone;
    if (ms.phases.isEmpty) {
      setState(() => _error = 'Milestone has no phases');
      return;
    }
    if (!forceNew && _existing != null && _existing!.isActive) {
      await _openRun(_existing!);
      return;
    }
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final created = await _api.createWorkflow(
        bundleId: _bundleId,
        slaveId: widget.slave.id,
        repoId: widget.project.id,
        progressDoc: ms.progressDoc ?? 'ai/progress.md',
        nodes: ms.phases.map((p) => p.toWorkflowNodeJson()).toList(),
      );
      final started = await _api.startWorkflow(created.id);
      if (!mounted) return;
      setState(() {
        _busy = false;
        _existing = started.workflow;
      });
      await _openRun(started.workflow);
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _busy = false;
        _error = e.toString();
      });
    }
  }

  Future<void> _confirmRestart() async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('重新开始？'),
        content: const Text(
          '会新建一条 Workflow，之前的进度不会自动延续到新实例。'
          '已 Approve 的节点仍保留在旧 Workflow 中。',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('取消'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('新建并开始'),
          ),
        ],
      ),
    );
    if (ok == true) await _createAndStart(forceNew: true);
  }

  @override
  Widget build(BuildContext context) {
    final ms = widget.milestone;
    final existing = _existing;
    return Scaffold(
      appBar: AppBar(
        title: Text('${ms.id} · 执行 plan'),
        actions: [
          IconButton(
            tooltip: 'Refresh',
            onPressed: _loading || _busy ? null : _refreshExisting,
            icon: const Icon(Icons.refresh),
          ),
        ],
      ),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : ListView(
              padding: const EdgeInsets.all(16),
              children: [
                Text(ms.title, style: Theme.of(context).textTheme.titleMedium),
                Text(
                  '${widget.project.name} · '
                  '${widget.slave.name.isNotEmpty ? widget.slave.name : widget.slave.id}',
                  style: Theme.of(context).textTheme.bodySmall,
                ),
                if (ms.progressDoc != null) ...[
                  const SizedBox(height: 4),
                  Text(
                    'progress: ${ms.progressDoc}',
                    style: Theme.of(context).textTheme.bodySmall,
                  ),
                ],
                if (existing != null) ...[
                  const SizedBox(height: 16),
                  Card(
                    child: ListTile(
                      title: Text('当前: ${existing.id}'),
                      subtitle: Text(
                        'status=${existing.status} · 返回后请点「继续」勿重复新建',
                      ),
                      trailing: WorkflowStatusChip(status: existing.status),
                      onTap: _busy ? null : () => _openRun(existing),
                    ),
                  ),
                ],
                const SizedBox(height: 16),
                Text('Phases', style: Theme.of(context).textTheme.titleSmall),
                const SizedBox(height: 8),
                ...ms.phases.map((p) {
                  WorkflowNode? node;
                  final nodes = existing?.nodes;
                  if (nodes != null) {
                    for (final n in nodes) {
                      if (n.id == p.id) {
                        node = n;
                        break;
                      }
                    }
                  }
                  return ListTile(
                    contentPadding: EdgeInsets.zero,
                    title: Text('${p.id} · ${p.title}'),
                    subtitle: Text(
                      p.dependsOn.isEmpty
                          ? p.phaseRef
                          : '${p.phaseRef}\ndependsOn: ${p.dependsOn.join(', ')}',
                    ),
                    isThreeLine: p.dependsOn.isNotEmpty,
                    trailing: node != null
                        ? NodeStatusChip(status: node.status)
                        : null,
                  );
                }),
                if (_error != null) ...[
                  const SizedBox(height: 12),
                  Text(
                    _error!,
                    style: TextStyle(color: Theme.of(context).colorScheme.error),
                  ),
                ],
                const SizedBox(height: 24),
                if (!widget.slave.online)
                  const FilledButton(
                    onPressed: null,
                    child: Text('Slave offline'),
                  )
                else if (existing != null && existing.isActive) ...[
                  FilledButton.icon(
                    onPressed: _busy ? null : () => _openRun(existing),
                    icon: const Icon(Icons.play_arrow),
                    label: const Text('继续当前 Workflow'),
                  ),
                  const SizedBox(height: 8),
                  OutlinedButton(
                    onPressed: _busy ? null : _confirmRestart,
                    child: const Text('重新开始（新建）'),
                  ),
                ] else if (existing != null && existing.isTerminal) ...[
                  FilledButton.icon(
                    onPressed: _busy ? null : () => _openRun(existing),
                    icon: const Icon(Icons.visibility),
                    label: const Text('查看上次结果'),
                  ),
                  const SizedBox(height: 8),
                  OutlinedButton(
                    onPressed: _busy
                        ? null
                        : () => _createAndStart(forceNew: true),
                    child: Text(_busy ? 'Starting…' : '再开一轮'),
                  ),
                ] else
                  FilledButton.icon(
                    onPressed: _busy
                        ? null
                        : () => _createAndStart(forceNew: false),
                    icon: _busy
                        ? const SizedBox(
                            width: 16,
                            height: 16,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : const Icon(Icons.play_arrow),
                    label: Text(_busy ? 'Starting…' : '开始此 Milestone'),
                  ),
              ],
            ),
    );
  }
}

/// Prefer newest active run for this milestone; else newest matching run.
@visibleForTesting
WorkflowRun? findMilestoneRun(
  List<WorkflowRun> list, {
  required String bundleId,
  required String repoId,
  required String slaveId,
}) {
  final matched = list.where((w) {
    if (w.bundleId != bundleId) return false;
    if (w.repoId != repoId) return false;
    if (slaveId.isNotEmpty && (w.slaveId ?? '') != slaveId) return false;
    return true;
  }).toList();
  if (matched.isEmpty) return null;
  matched.sort((a, b) => b.createdAt.compareTo(a.createdAt));
  for (final w in matched) {
    if (w.isActive) return w;
  }
  return matched.first;
}
