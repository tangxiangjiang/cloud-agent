import 'package:flutter/material.dart';

import '../auth/session.dart';
import '../slaves/models.dart';
import '../sync/models.dart';
import '../sync/project_sync_api.dart';
import 'milestone_phases_page.dart';
import 'project_chat_page.dart';
import 'workflow_detail_page.dart';

/// Step 3: 工程页 — Milestone 列表 + 同步（M08-P04）.
class MilestoneListPage extends StatefulWidget {
  const MilestoneListPage({
    super.key,
    required this.session,
    required this.slave,
    required this.project,
    this.syncApi,
    this.pollAfterSync = const Duration(milliseconds: 400),
    this.syncTimeout = const Duration(seconds: 20),
  });

  final Session session;
  final SlaveInfo slave;
  final ProjectInfo project;
  final ProjectSyncApi? syncApi;
  final Duration pollAfterSync;
  final Duration syncTimeout;

  @override
  State<MilestoneListPage> createState() => _MilestoneListPageState();
}

class _MilestoneListPageState extends State<MilestoneListPage> {
  late final ProjectSyncApi _syncApi =
      widget.syncApi ?? ProjectSyncApi(session: widget.session);

  ProjectSyncSnapshot? _snapshot;
  bool _loadingSnap = true;
  bool _syncing = false;
  String? _error;

  bool get _online => widget.slave.online;

  @override
  void initState() {
    super.initState();
    _loadSnapshot(silent: true);
  }

  Future<void> _loadSnapshot({bool silent = false}) async {
    if (!silent) {
      setState(() {
        _loadingSnap = true;
        _error = null;
      });
    }
    try {
      final snap = await _syncApi.getSync(
        slaveId: widget.slave.id,
        repoId: widget.project.id,
      );
      if (!mounted) return;
      setState(() {
        _snapshot = snap;
        _loadingSnap = false;
        _error = null;
      });
    } on ProjectSyncApiException catch (e) {
      if (!mounted) return;
      setState(() {
        _loadingSnap = false;
        if (e.statusCode != 404) {
          _error = e.message;
        }
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _loadingSnap = false;
        _error = e.toString();
      });
    }
  }

  Future<void> _runSync() async {
    if (!_online) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Slave offline — sync disabled')),
      );
      return;
    }
    setState(() {
      _syncing = true;
      _error = null;
    });
    try {
      final snap = await _syncApi.syncAndWait(
        slaveId: widget.slave.id,
        repoId: widget.project.id,
        timeout: widget.syncTimeout,
        interval: widget.pollAfterSync,
      );
      if (!mounted) return;
      setState(() {
        _snapshot = snap;
        _syncing = false;
      });
      final n = snap.warnings.length;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(
            n == 0
                ? 'Sync ok'
                : 'Sync ok — $n warning${n == 1 ? '' : 's'}',
          ),
        ),
      );
    } on ProjectSyncApiException catch (e) {
      if (!mounted) return;
      setState(() {
        _syncing = false;
        _error = e.message;
      });
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Sync failed: ${e.message}')),
      );
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _syncing = false;
        _error = e.toString();
      });
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Sync failed: $e')),
      );
    }
  }

  void _openWorkflow(String workflowId) {
    Navigator.of(context).push<void>(
      MaterialPageRoute(
        builder: (_) => WorkflowDetailPage(
          session: widget.session,
          workflowId: workflowId,
        ),
      ),
    );
  }

  Future<void> _alignProgress() async {
    if (!_online) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Slave offline — align disabled')),
      );
      return;
    }
    setState(() {
      _syncing = true;
      _error = null;
    });
    try {
      final snap = await _syncApi.alignProgressAndSync(
        slaveId: widget.slave.id,
        repoId: widget.project.id,
        timeout: widget.syncTimeout,
        interval: widget.pollAfterSync,
      );
      if (!mounted) return;
      setState(() {
        _snapshot = snap;
        _syncing = false;
      });
      final n = snap.warnings.where((w) => w.canAlignProgress).length;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(
            n == 0
                ? 'Progress aligned'
                : 'Aligned — $n gateway_ahead warning${n == 1 ? '' : 's'} left',
          ),
        ),
      );
    } on ProjectSyncApiException catch (e) {
      if (!mounted) return;
      setState(() {
        _syncing = false;
        _error = e.message;
      });
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Align failed: ${e.message}')),
      );
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _syncing = false;
        _error = e.toString();
      });
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Align failed: $e')),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    // Prefer latest disk index from sync; register catalog can be stale
    // until Slave restart (e.g. M11 → M12 while process still running).
    final milestones =
        _snapshot?.milestonesFromIndex ?? widget.project.milestones;
    final theme = Theme.of(context);

    return Scaffold(
      appBar: AppBar(
        title: Text(
          widget.project.name.isNotEmpty
              ? widget.project.name
              : widget.project.id,
        ),
        actions: [
          IconButton(
            tooltip: 'Project chat',
            onPressed: () {
              Navigator.of(context).push<void>(
                MaterialPageRoute(
                  builder: (_) => ProjectChatPage(
                    session: widget.session,
                    slave: widget.slave,
                    project: widget.project,
                  ),
                ),
              );
            },
            icon: const Icon(Icons.chat_outlined),
          ),
          if (_syncing)
            const Padding(
              padding: EdgeInsets.symmetric(horizontal: 16),
              child: Center(
                child: SizedBox(
                  width: 22,
                  height: 22,
                  child: CircularProgressIndicator(strokeWidth: 2),
                ),
              ),
            )
          else
            IconButton(
              tooltip: _online ? 'Sync project state' : 'Slave offline',
              onPressed: _online && !_syncing ? _runSync : null,
              icon: const Icon(Icons.sync),
            ),
        ],
      ),
      body: ListView(
        padding: const EdgeInsets.only(bottom: 24),
        children: [
          if (!_online)
            Material(
              color: theme.colorScheme.errorContainer,
              child: const ListTile(
                leading: Icon(Icons.cloud_off),
                title: Text('Slave offline'),
                subtitle: Text('Connect the Slave before syncing.'),
              ),
            ),
          if (_error != null)
            ListTile(
              leading: Icon(Icons.error_outline, color: theme.colorScheme.error),
              title: Text(_error!, style: TextStyle(color: theme.colorScheme.error)),
            ),
          _SyncResultPanel(
            loading: _loadingSnap && _snapshot == null,
            snapshot: _snapshot,
            syncing: _syncing,
            onRefresh: _online ? _runSync : null,
            onContinueWorkflow: _openWorkflow,
            onAlignProgress: _online ? _alignProgress : null,
          ),
          const Divider(height: 1),
          ListTile(
            leading: const Icon(Icons.chat_outlined),
            title: const Text('Chat'),
            subtitle: const Text('Agent · Auto — Local Agent via Gateway'),
            trailing: const Icon(Icons.chevron_right),
            onTap: () {
              Navigator.of(context).push<void>(
                MaterialPageRoute(
                  builder: (_) => ProjectChatPage(
                    session: widget.session,
                    slave: widget.slave,
                    project: widget.project,
                  ),
                ),
              );
            },
          ),
          const Divider(height: 1),
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 12, 16, 4),
            child: Text('Milestones', style: theme.textTheme.titleSmall),
          ),
          if (milestones.isEmpty)
            Padding(
              padding: const EdgeInsets.all(24),
              child: Text(
                widget.project.index == null
                    ? 'No milestone index (set projects[].index).'
                    : 'No milestones in ${widget.project.index}.',
              ),
            )
          else
            ...milestones.map((m) {
              return ListTile(
                leading: const Icon(Icons.flag_outlined),
                title: Text('${m.id} · ${m.title}'),
                subtitle: Text(
                  '${m.phases.length} phase${m.phases.length == 1 ? '' : 's'}'
                  '${m.progressDoc != null ? ' · ${m.progressDoc}' : ''}',
                ),
                trailing: const Icon(Icons.chevron_right),
                onTap: () {
                  final project = ProjectInfo(
                    id: widget.project.id,
                    name: widget.project.name,
                    cwd: widget.project.cwd,
                    index: widget.project.index,
                    milestones: milestones,
                  );
                  Navigator.of(context).push<void>(
                    MaterialPageRoute(
                      builder: (_) => MilestonePhasesPage(
                        session: widget.session,
                        slave: widget.slave,
                        project: project,
                        milestone: m,
                      ),
                    ),
                  );
                },
              );
            }),
        ],
      ),
    );
  }
}

class _SyncResultPanel extends StatelessWidget {
  const _SyncResultPanel({
    required this.loading,
    required this.snapshot,
    required this.syncing,
    this.onRefresh,
    required this.onContinueWorkflow,
    this.onAlignProgress,
  });

  final bool loading;
  final ProjectSyncSnapshot? snapshot;
  final bool syncing;
  final VoidCallback? onRefresh;
  final void Function(String workflowId) onContinueWorkflow;
  final VoidCallback? onAlignProgress;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    if (loading) {
      return const Padding(
        padding: EdgeInsets.all(16),
        child: Center(child: CircularProgressIndicator(strokeWidth: 2)),
      );
    }
    final snap = snapshot;
    if (snap == null) {
      return ListTile(
        leading: const Icon(Icons.info_outline),
        title: const Text('Not synced yet'),
        subtitle: const Text('Tap sync to collect git / progress from Slave.'),
        trailing: onRefresh == null
            ? null
            : TextButton(onPressed: syncing ? null : onRefresh, child: const Text('Sync')),
      );
    }

    final dirty = snap.dirty;
    final summary = snap.summary;
    final branch = snap.branch;

    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 8, 16, 8),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('Last sync', style: theme.textTheme.titleSmall),
          const SizedBox(height: 4),
          Text(
            snap.syncedAt,
            style: theme.textTheme.bodySmall,
          ),
          const SizedBox(height: 6),
          Wrap(
            spacing: 8,
            runSpacing: 4,
            children: [
              if (branch != null && branch.isNotEmpty)
                Chip(
                  visualDensity: VisualDensity.compact,
                  label: Text('branch: $branch'),
                ),
              if (dirty != null)
                Chip(
                  visualDensity: VisualDensity.compact,
                  avatar: Icon(
                    dirty ? Icons.warning_amber : Icons.check_circle_outline,
                    size: 16,
                  ),
                  label: Text(dirty ? 'dirty' : 'clean'),
                ),
              if (snap.report?.workflowId != null)
                Chip(
                  visualDensity: VisualDensity.compact,
                  label: Text(
                    snap.report!.active ? 'wf active' : 'wf idle',
                  ),
                ),
            ],
          ),
          if (summary != null && summary.isNotEmpty) ...[
            const SizedBox(height: 8),
            Text(summary, style: theme.textTheme.bodyMedium),
          ],
          if (snap.warnings.isNotEmpty) ...[
            const SizedBox(height: 12),
            Row(
              children: [
                Expanded(
                  child: Text(
                    'Warnings (${snap.warnings.length})',
                    style: theme.textTheme.titleSmall,
                  ),
                ),
                if (onAlignProgress != null &&
                    snap.warnings.any((w) => w.canAlignProgress))
                  TextButton(
                    onPressed: syncing ? null : onAlignProgress,
                    child: const Text('Align progress'),
                  ),
              ],
            ),
            const SizedBox(height: 4),
            ...snap.warnings.map((w) {
              final wfId = w.canContinueWorkflow
                  ? w.workflowId
                  : (w.suggestion == 'continue_workflow' ||
                          w.code == 'progress_ahead'
                      ? snap.report?.workflowId
                      : null);
              Widget? trailing;
              if (w.canAlignProgress && onAlignProgress != null) {
                trailing = TextButton(
                  onPressed: syncing ? null : onAlignProgress,
                  child: const Text('Align'),
                );
              } else if (wfId != null && wfId.isNotEmpty) {
                trailing = TextButton(
                  onPressed: () => onContinueWorkflow(wfId),
                  child: const Text('Continue'),
                );
              }
              return Card(
                margin: const EdgeInsets.only(bottom: 8),
                child: ListTile(
                  dense: true,
                  title: Text(w.message),
                  subtitle: Text(
                    [
                      if (w.code.isNotEmpty) w.code,
                      if (w.phaseId != null) w.phaseId!,
                    ].join(' · '),
                  ),
                  trailing: trailing,
                ),
              );
            }),
          ],
          if (snap.report != null && snap.report!.phases.isNotEmpty) ...[
            const SizedBox(height: 8),
            Text('Phases', style: theme.textTheme.titleSmall),
            const SizedBox(height: 4),
            ...snap.report!.phases.take(12).map((p) {
              return Padding(
                padding: const EdgeInsets.symmetric(vertical: 2),
                child: Text(
                  '${p.id}: progress=${p.progressStatus}'
                  '${p.nodeStatus != null ? ' · node=${p.nodeStatus}' : ''}',
                  style: theme.textTheme.bodySmall,
                ),
              );
            }),
          ],
        ],
      ),
    );
  }
}
