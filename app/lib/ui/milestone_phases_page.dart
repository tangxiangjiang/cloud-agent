import 'dart:async';

import 'package:flutter/material.dart';

import '../auth/session.dart';
import '../chat/chat_api.dart';
import '../chat/models.dart';
import '../slaves/models.dart';
import '../workflow/models.dart';
import '../workflow/workflow_api.dart';
import 'node_status_chip.dart';
import 'workflow_detail_page.dart';

/// Shows milestone phases; resume existing run or create+start once.
/// Master「自动续跑」controls all phases (create default or PATCH existing).
class MilestonePhasesPage extends StatefulWidget {
  const MilestonePhasesPage({
    super.key,
    required this.session,
    required this.slave,
    required this.project,
    required this.milestone,
    this.api,
    this.chatApi,
  });

  final Session session;
  final SlaveInfo slave;
  final ProjectInfo project;
  final MilestoneInfo milestone;
  final WorkflowApi? api;
  final ChatApi? chatApi;

  @override
  State<MilestonePhasesPage> createState() => _MilestonePhasesPageState();
}

class _MilestonePhasesPageState extends State<MilestonePhasesPage> {
  late final WorkflowApi _api =
      widget.api ?? WorkflowApi(session: widget.session);
  late final ChatApi _chatApi =
      widget.chatApi ?? ChatApi(session: widget.session);

  bool _loading = true;
  bool _busy = false;
  String? _error;
  WorkflowRun? _existing;

  ModelCatalog _catalog = ModelCatalog.fallback;
  String _defaultModel = 'auto';
  bool _defaultAutoContinue = false;

  String get _bundleId => 'milestone:${widget.milestone.id}';

  @override
  void initState() {
    super.initState();
    unawaited(_loadModels());
    _refreshExisting();
  }

  Future<void> _loadModels() async {
    try {
      final c = await _chatApi.listModels();
      if (!mounted) return;
      setState(() {
        _catalog = c;
        if (!_catalog.models.any((m) => m.id == _defaultModel)) {
          _defaultModel = c.defaultId;
        }
      });
    } catch (_) {
      // Keep Auto fallback.
    }
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
        if (match != null && match.nodes.isNotEmpty) {
          _defaultAutoContinue =
              match.nodes.every((n) => n.policy.autoContinue);
        }
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

  Future<void> _createWorkflow({required bool forceNew}) async {
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
        defaultModel: _defaultModel,
        defaultPolicy: NodePolicy(
          autoApprove: _defaultAutoContinue,
          autoStartNext: _defaultAutoContinue,
        ),
        nodes: serialWorkflowNodesFromPhases(ms.phases),
      );
      if (!mounted) return;
      setState(() {
        _busy = false;
        _existing = created;
      });
      // First phase stays ready — user starts it explicitly on the detail page.
      await _openRun(created);
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
          '已 Approve 的节点仍保留在旧 Workflow 中。'
          '新建后首个任务为就绪，需手动点开始。',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('取消'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('新建'),
          ),
        ],
      ),
    );
    if (ok == true) await _createWorkflow(forceNew: true);
  }

  String _labelFor(String id) {
    for (final m in _catalog.models) {
      if (m.id == id) return m.label;
    }
    return id;
  }

  String _existingSubtitle(WorkflowRun existing) {
    switch (existing.status) {
      case 'completed':
        return 'status=completed · 本 Milestone 已全部通过';
      case 'failed':
      case 'cancelled':
        return 'status=${existing.status} · 可查看详情或再开一轮';
      default:
        return 'status=${existing.status} · 返回后请点「继续」勿重复新建';
    }
  }

  Future<void> _setAutoContinueMaster(bool enabled) async {
    final existing = _existing;
    // No active workflow yet — only remember for create / restart.
    if (existing == null ||
        existing.status == 'failed' ||
        existing.status == 'cancelled') {
      setState(() => _defaultAutoContinue = enabled);
      return;
    }
    if (existing.status == 'completed') {
      setState(() => _defaultAutoContinue = enabled);
      return;
    }

    final targets =
        existing.nodes.where((n) => n.policyEditable).toList(growable: false);
    if (targets.isEmpty) {
      setState(() => _defaultAutoContinue = enabled);
      return;
    }

    setState(() {
      _busy = true;
      _error = null;
      _defaultAutoContinue = enabled;
    });
    try {
      WorkflowRun? latest = existing;
      final policy = NodePolicy(
        autoApprove: enabled,
        autoStartNext: enabled,
      );
      for (final n in targets) {
        if (n.policy.autoApprove == enabled &&
            n.policy.autoStartNext == enabled) {
          continue;
        }
        latest = await _api.patchNode(
          existing.id,
          n.id,
          policy: policy,
        );
      }
      if (!mounted) return;
      setState(() {
        _busy = false;
        if (latest != null) {
          _existing = latest;
          _defaultAutoContinue =
              latest.nodes.every((n) => n.policy.autoContinue);
        }
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _busy = false;
        _error = e.toString();
      });
      await _refreshExisting();
    }
  }

  Widget _policyMasterCard(ThemeData theme) {
    final existing = _existing;
    final creating = existing == null ||
        existing.status == 'failed' ||
        existing.status == 'cancelled';
    final completed = existing?.status == 'completed';
    final canToggle = !_busy && !completed && widget.slave.online;
    final modelIds = _catalog.models.map((m) => m.id).toList();
    if (!modelIds.contains(_defaultModel)) {
      modelIds.insert(0, _defaultModel);
    }

    return Card(
      child: Padding(
        padding: const EdgeInsets.fromLTRB(12, 10, 12, 8),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('自动续跑（总开关）', style: theme.textTheme.titleSmall),
            Text(
              creating
                  ? '开启后，新建 Workflow 时全部阶段自动过审并开下个。'
                  : completed
                      ? '本 Milestone 已完成，策略只读。'
                      : '一键开关当前 Workflow 全部阶段（自动过审 + 自动开下个）。',
              style: theme.textTheme.bodySmall?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
            ),
            const SizedBox(height: 8),
            Wrap(
              crossAxisAlignment: WrapCrossAlignment.center,
              spacing: 12,
              runSpacing: 4,
              children: [
                if (creating)
                  DropdownButtonHideUnderline(
                    child: DropdownButton<String>(
                      key: const ValueKey('milestone-default-model'),
                      value: modelIds.contains(_defaultModel)
                          ? _defaultModel
                          : modelIds.first,
                      isDense: true,
                      items: [
                        for (final id in modelIds)
                          DropdownMenuItem(
                            value: id,
                            child: Text(_labelFor(id)),
                          ),
                      ],
                      onChanged: canToggle
                          ? (v) {
                              if (v == null) return;
                              setState(() => _defaultModel = v);
                            }
                          : null,
                    ),
                  ),
                Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text('全部开启', style: theme.textTheme.bodySmall),
                    Switch(
                      key: const ValueKey('milestone-default-autoContinue'),
                      value: _defaultAutoContinue,
                      onChanged: canToggle
                          ? (v) => unawaited(_setAutoContinueMaster(v))
                          : null,
                    ),
                  ],
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final ms = widget.milestone;
    final existing = _existing;
    final theme = Theme.of(context);
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
                Text(ms.title, style: theme.textTheme.titleMedium),
                Text(
                  '${widget.project.name} · '
                  '${widget.slave.name.isNotEmpty ? widget.slave.name : widget.slave.id}',
                  style: theme.textTheme.bodySmall,
                ),
                if (ms.progressDoc != null) ...[
                  const SizedBox(height: 4),
                  Text(
                    'progress: ${ms.progressDoc}',
                    style: theme.textTheme.bodySmall,
                  ),
                ],
                if (existing != null) ...[
                  const SizedBox(height: 16),
                  Card(
                    child: ListTile(
                      title: Text('当前: ${existing.id}'),
                      subtitle: Text(_existingSubtitle(existing)),
                      trailing: WorkflowStatusChip(status: existing.status),
                      onTap: _busy ? null : () => _openRun(existing),
                    ),
                  ),
                ],
                const SizedBox(height: 12),
                _policyMasterCard(theme),
                const SizedBox(height: 16),
                Text('Phases', style: theme.textTheme.titleSmall),
                const SizedBox(height: 8),
                ...List.generate(ms.phases.length, (i) {
                  final p = ms.phases[i];
                  final serialDeps =
                      i == 0 ? const <String>[] : <String>[ms.phases[i - 1].id];
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
                      serialDeps.isEmpty
                          ? p.phaseRef
                          : '${p.phaseRef}\ndependsOn: ${serialDeps.join(', ')}',
                    ),
                    isThreeLine: serialDeps.isNotEmpty,
                    trailing: node != null
                        ? NodeStatusChip(status: node.status)
                        : null,
                  );
                }),
                if (_error != null) ...[
                  const SizedBox(height: 12),
                  Text(
                    _error!,
                    style: TextStyle(color: theme.colorScheme.error),
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
                  // Completed = done; only failed/cancelled offer another round.
                  if (existing.status != 'completed') ...[
                    const SizedBox(height: 8),
                    OutlinedButton(
                      onPressed: _busy
                          ? null
                          : () => _createWorkflow(forceNew: true),
                      child: Text(_busy ? 'Creating…' : '再开一轮'),
                    ),
                  ],
                ] else
                  FilledButton.icon(
                    onPressed: _busy
                        ? null
                        : () => _createWorkflow(forceNew: false),
                    icon: _busy
                        ? const SizedBox(
                            width: 16,
                            height: 16,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : const Icon(Icons.playlist_add_check),
                    label: Text(_busy ? 'Creating…' : '创建此 Milestone'),
                  ),
              ],
            ),
    );
  }
}

/// Prefer newest active run for this milestone; else newest matching run.
///
/// Same [repoId]+[bundleId] wins even if [slaveId] changed (e.g. M11 1:1
/// remapping `slave_devpc` → `slave_nyralang`). Prefer exact slave match when
/// present; otherwise fall back so completed history is not lost.
@visibleForTesting
WorkflowRun? findMilestoneRun(
  List<WorkflowRun> list, {
  required String bundleId,
  required String repoId,
  required String slaveId,
}) {
  final sameRepo = list.where((w) {
    if (w.bundleId != bundleId) return false;
    if (w.repoId != repoId) return false;
    return true;
  }).toList();
  if (sameRepo.isEmpty) return null;

  List<WorkflowRun> matched = sameRepo;
  if (slaveId.isNotEmpty) {
    final sameSlave = sameRepo
        .where((w) => (w.slaveId ?? '') == slaveId)
        .toList();
    if (sameSlave.isNotEmpty) matched = sameSlave;
  }

  matched.sort((a, b) => b.createdAt.compareTo(a.createdAt));
  for (final w in matched) {
    if (w.isActive) return w;
  }
  return matched.first;
}
