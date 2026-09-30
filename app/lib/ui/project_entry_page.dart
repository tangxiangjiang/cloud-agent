import 'dart:async';

import 'package:flutter/material.dart';

import '../auth/session.dart';
import '../masters/masters_api.dart';
import '../masters/models.dart';
import '../slaves/models.dart';
import '../slaves/slaves_api.dart';
import 'milestone_list_page.dart';

/// Project entry: only slaves with process=running AND gatewayOnline.
class ProjectEntryPage extends StatefulWidget {
  const ProjectEntryPage({
    super.key,
    required this.session,
    this.mastersApi,
    this.slavesApi,
    this.pollInterval = const Duration(seconds: 8),
  });

  final Session session;
  final MastersApi? mastersApi;
  final SlavesApi? slavesApi;
  final Duration pollInterval;

  @override
  State<ProjectEntryPage> createState() => _ProjectEntryPageState();
}

class _ProjectEntryPageState extends State<ProjectEntryPage> {
  late final MastersApi _masters =
      widget.mastersApi ?? MastersApi(session: widget.session);
  late final SlavesApi _slaves =
      widget.slavesApi ?? SlavesApi(session: widget.session);

  List<_EntryRow>? _rows;
  String? _error;
  bool _loading = true;
  Timer? _poll;

  @override
  void initState() {
    super.initState();
    _refresh();
    _poll = Timer.periodic(widget.pollInterval, (_) {
      if (!mounted || _loading) return;
      _refresh(silent: true);
    });
  }

  @override
  void dispose() {
    _poll?.cancel();
    super.dispose();
  }

  Future<void> _refresh({bool silent = false}) async {
    if (!silent) {
      setState(() {
        _loading = true;
        _error = null;
      });
    }
    try {
      final masters = await _masters.listMasters();
      Map<String, SlaveInfo> byId = {};
      try {
        final dataPlane = await _slaves.listSlaves();
        for (final s in dataPlane) {
          byId[s.id] = s;
        }
      } catch (_) {
        // milestones enrich optional
      }
      final rows = <_EntryRow>[];
      for (final m in masters) {
        for (final s in m.slaves) {
          rows.add(_EntryRow(
            masterId: m.masterId,
            fleet: s,
            enrich: byId[s.id],
          ));
        }
      }
      if (!mounted) return;
      setState(() {
        _rows = rows;
        _loading = false;
        _error = null;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        if (!silent || _rows == null) {
          _error = e.toString();
        }
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('工程'),
        actions: [
          IconButton(
            tooltip: 'Refresh',
            onPressed: () => _refresh(),
            icon: const Icon(Icons.refresh),
          ),
        ],
      ),
      body: RefreshIndicator(
        onRefresh: () => _refresh(),
        child: _buildBody(),
      ),
    );
  }

  Widget _buildBody() {
    if (_loading && _rows == null) {
      return ListView(
        physics: const AlwaysScrollableScrollPhysics(),
        children: const [
          SizedBox(height: 120),
          Center(child: CircularProgressIndicator()),
        ],
      );
    }
    if (_error != null && _rows == null) {
      return ListView(
        physics: const AlwaysScrollableScrollPhysics(),
        padding: const EdgeInsets.all(24),
        children: [
          Text(_error!,
              style: TextStyle(color: Theme.of(context).colorScheme.error)),
          const SizedBox(height: 16),
          FilledButton(onPressed: () => _refresh(), child: const Text('Retry')),
        ],
      );
    }

    final enterable =
        (_rows ?? const <_EntryRow>[]).where((r) => r.fleet.canEnterProject);
    final hidden =
        (_rows ?? const <_EntryRow>[]).where((r) => !r.fleet.canEnterProject);

    if (enterable.isEmpty && hidden.isEmpty) {
      return ListView(
        physics: const AlwaysScrollableScrollPhysics(),
        padding: const EdgeInsets.all(24),
        children: [
          Text('暂无工程', style: Theme.of(context).textTheme.titleMedium),
          const SizedBox(height: 8),
          Text(
            '在「舰队」页 Start Slave，并确认 Gateway 数据面 online。',
            style: Theme.of(context).textTheme.bodySmall,
          ),
        ],
      );
    }

    return ListView(
      physics: const AlwaysScrollableScrollPhysics(),
      children: [
        for (final r in enterable)
          ListTile(
            leading: Icon(
              Icons.folder_open,
              color: Theme.of(context).colorScheme.primary,
            ),
            title: Text(
              r.fleet.project.name.isNotEmpty
                  ? r.fleet.project.name
                  : r.fleet.id,
            ),
            subtitle: Text(
              '${r.fleet.id} · ${r.fleet.project.cwd}',
            ),
            trailing: const Icon(Icons.chevron_right),
            onTap: () {
              final slave = r.fleet.toSlaveInfo(enrichFrom: r.enrich);
              final projects = slave.effectiveProjects;
              if (projects.isEmpty) {
                ScaffoldMessenger.of(context).showSnackBar(
                  const SnackBar(content: Text('该 Slave 无绑定工程')),
                );
                return;
              }
              // M11: slave ↔ project 1:1 — skip project list.
              Navigator.of(context).push<void>(
                MaterialPageRoute(
                  builder: (_) => MilestoneListPage(
                    session: widget.session,
                    slave: slave,
                    project: projects.first,
                  ),
                ),
              );
            },
          ),
        if (hidden.isNotEmpty) ...[
          const Divider(),
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 8, 16, 4),
            child: Text(
              '未就绪（不可进入）',
              style: Theme.of(context).textTheme.labelLarge,
            ),
          ),
          for (final r in hidden)
            ListTile(
              enabled: false,
              leading: Icon(
                Icons.folder_off,
                color: Theme.of(context).disabledColor,
              ),
              title: Text(
                r.fleet.project.name.isNotEmpty
                    ? r.fleet.project.name
                    : r.fleet.id,
              ),
              subtitle: Text(
                '${r.fleet.process}'
                '${r.fleet.gatewayOnline ? '' : ' · gateway offline'}',
              ),
            ),
        ],
      ],
    );
  }
}

class _EntryRow {
  _EntryRow({
    required this.masterId,
    required this.fleet,
    this.enrich,
  });

  final String masterId;
  final FleetSlave fleet;
  final SlaveInfo? enrich;
}
