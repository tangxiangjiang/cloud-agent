import 'dart:async';

import 'package:flutter/material.dart';

import '../auth/session.dart';
import '../masters/masters_api.dart';
import '../masters/models.dart';
import 'models_page.dart';

/// Fleet management: Master → Slave config + process control.
class FleetPage extends StatefulWidget {
  const FleetPage({
    super.key,
    required this.session,
    this.api,
    this.onSignOut,
    this.pollInterval = const Duration(seconds: 8),
  });

  final Session session;
  final MastersApi? api;
  final VoidCallback? onSignOut;
  final Duration pollInterval;

  @override
  State<FleetPage> createState() => _FleetPageState();
}

class _FleetPageState extends State<FleetPage> {
  late final MastersApi _api =
      widget.api ?? MastersApi(session: widget.session);

  List<MasterInfo>? _masters;
  String? _error;
  bool _loading = true;
  String? _busyKey;
  Timer? _poll;

  @override
  void initState() {
    super.initState();
    _refresh();
    _poll = Timer.periodic(widget.pollInterval, (_) {
      if (!mounted || _loading || _busyKey != null) return;
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
      final list = await _api.listMasters();
      if (!mounted) return;
      setState(() {
        _masters = list;
        _loading = false;
        _error = null;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        if (!silent || _masters == null) {
          _error = e.toString();
        }
      });
    }
  }

  Future<void> _runControl(
    String masterId,
    String slaveId,
    Future<MasterInfo> Function() op,
  ) async {
    final key = '$masterId/$slaveId';
    setState(() => _busyKey = key);
    try {
      await op();
      final fresh = await _api.getMaster(masterId);
      if (!mounted) return;
      setState(() {
        _masters = [
          for (final m in _masters ?? const <MasterInfo>[])
            if (m.masterId == masterId) fresh else m,
        ];
        if (!(_masters?.any((m) => m.masterId == masterId) ?? false)) {
          _masters = [...?_masters, fresh];
        }
      });
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(e.toString())),
      );
    } finally {
      if (mounted) setState(() => _busyKey = null);
    }
  }

  Future<void> _showSlaveForm({
    required String masterId,
    FleetSlave? existing,
  }) async {
    final idCtrl = TextEditingController(text: existing?.id ?? '');
    final nameCtrl = TextEditingController(text: existing?.name ?? '');
    final cwdCtrl = TextEditingController(text: existing?.project.cwd ?? '');
    final indexCtrl =
        TextEditingController(text: existing?.project.index ?? '');
    final projectIdCtrl =
        TextEditingController(text: existing?.project.id ?? '');
    final projectNameCtrl =
        TextEditingController(text: existing?.project.name ?? '');
    var enabled = existing?.enabled ?? true;
    final create = existing == null;

    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) {
        return StatefulBuilder(
          builder: (ctx, setLocal) {
            return AlertDialog(
              title: Text(create ? '添加 Slave' : '编辑 Slave'),
              content: SingleChildScrollView(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    TextField(
                      controller: idCtrl,
                      decoration: const InputDecoration(labelText: 'slaveId'),
                      enabled: create,
                    ),
                    TextField(
                      controller: nameCtrl,
                      decoration: const InputDecoration(labelText: '名称'),
                    ),
                    TextField(
                      controller: projectIdCtrl,
                      decoration: const InputDecoration(labelText: 'project.id'),
                    ),
                    TextField(
                      controller: projectNameCtrl,
                      decoration:
                          const InputDecoration(labelText: 'project.name'),
                    ),
                    TextField(
                      controller: cwdCtrl,
                      decoration: const InputDecoration(
                        labelText: 'cwd（绝对路径）',
                      ),
                    ),
                    TextField(
                      controller: indexCtrl,
                      decoration: const InputDecoration(
                        labelText: 'index（相对路径，可选）',
                      ),
                    ),
                    SwitchListTile(
                      contentPadding: EdgeInsets.zero,
                      title: const Text('enabled（允许 Start）'),
                      value: enabled,
                      onChanged: (v) => setLocal(() => enabled = v),
                    ),
                  ],
                ),
              ),
              actions: [
                TextButton(
                  onPressed: () => Navigator.pop(ctx, false),
                  child: const Text('取消'),
                ),
                FilledButton(
                  onPressed: () => Navigator.pop(ctx, true),
                  child: const Text('保存'),
                ),
              ],
            );
          },
        );
      },
    );
    if (ok != true || !mounted) return;

    final cwd = cwdCtrl.text.trim();
    if (cwd.isEmpty || cwd.contains('..')) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('cwd 不能为空且不可含 ..')),
      );
      return;
    }
    final slave = <String, dynamic>{
      'id': idCtrl.text.trim(),
      'name': nameCtrl.text.trim(),
      'enabled': enabled,
      'project': {
        'id': projectIdCtrl.text.trim(),
        'name': projectNameCtrl.text.trim().isEmpty
            ? projectIdCtrl.text.trim()
            : projectNameCtrl.text.trim(),
        'cwd': cwd,
        if (indexCtrl.text.trim().isNotEmpty) 'index': indexCtrl.text.trim(),
      },
    };
    if ((slave['id'] as String).isEmpty ||
        (slave['project'] as Map)['id'].toString().isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('slaveId 与 project.id 必填')),
      );
      return;
    }

    setState(() => _busyKey = '$masterId/${slave['id']}');
    try {
      await _api.upsertSlave(masterId, slave, create: create);
      await _refresh();
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(e.toString())),
      );
    } finally {
      if (mounted) setState(() => _busyKey = null);
    }
  }

  Future<void> _delete(String masterId, FleetSlave s) async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('删除 Slave？'),
        content: Text('将 Stop 并删除 ${s.id} 的配置。'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('取消'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('删除'),
          ),
        ],
      ),
    );
    if (ok != true) return;
    setState(() => _busyKey = '$masterId/${s.id}');
    try {
      await _api.deleteSlave(masterId, s.id);
      await _refresh();
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(e.toString())),
      );
    } finally {
      if (mounted) setState(() => _busyKey = null);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('舰队'),
        actions: [
          IconButton(
            tooltip: '模型列表',
            onPressed: () {
              Navigator.of(context).push(
                MaterialPageRoute<void>(
                  builder: (_) => ModelsPage(session: widget.session),
                ),
              );
            },
            icon: const Icon(Icons.smart_toy_outlined),
          ),
          IconButton(
            tooltip: 'Refresh',
            onPressed: () => _refresh(),
            icon: const Icon(Icons.refresh),
          ),
          if (widget.onSignOut != null)
            IconButton(
              tooltip: 'Sign out',
              onPressed: widget.onSignOut,
              icon: const Icon(Icons.logout),
            ),
        ],
      ),
      floatingActionButton: (_masters != null && _masters!.isNotEmpty)
          ? FloatingActionButton(
              tooltip: '添加 Slave',
              onPressed: _busyKey != null
                  ? null
                  : () => _showSlaveForm(masterId: _masters!.first.masterId),
              child: const Icon(Icons.add),
            )
          : null,
      body: RefreshIndicator(
        onRefresh: () => _refresh(),
        child: _buildBody(),
      ),
    );
  }

  Widget _buildBody() {
    if (_loading && _masters == null) {
      return ListView(
        physics: const AlwaysScrollableScrollPhysics(),
        children: const [
          SizedBox(height: 120),
          Center(child: CircularProgressIndicator()),
        ],
      );
    }
    if (_error != null && _masters == null) {
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
    final masters = _masters ?? const <MasterInfo>[];
    if (masters.isEmpty) {
      return ListView(
        physics: const AlwaysScrollableScrollPhysics(),
        padding: const EdgeInsets.all(24),
        children: [
          Text('暂无 Master', style: Theme.of(context).textTheme.titleMedium),
          const SizedBox(height: 8),
          Text(
            '在本机运行: npm run master -- serve\n'
            'Gateway: ${widget.session.gatewayBaseUrl}',
            style: Theme.of(context).textTheme.bodySmall,
          ),
        ],
      );
    }

    final tiles = <Widget>[];
    for (final m in masters) {
      tiles.add(
        ListTile(
          title: Text(m.name.isNotEmpty ? m.name : m.masterId),
          subtitle: Text(
            '${m.masterId} · ${m.online ? 'online' : 'offline'}',
          ),
          leading: Icon(
            m.online ? Icons.dns : Icons.dns_outlined,
            color: m.online
                ? Theme.of(context).colorScheme.primary
                : Theme.of(context).disabledColor,
          ),
        ),
      );
      for (final s in m.slaves) {
        final busy = _busyKey == '${m.masterId}/${s.id}';
        tiles.add(
          Card(
            margin: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
            child: Padding(
              padding: const EdgeInsets.all(12),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    s.name.isNotEmpty ? s.name : s.id,
                    style: Theme.of(context).textTheme.titleMedium,
                  ),
                  Text('${s.id} · ${s.project.cwd}',
                      style: Theme.of(context).textTheme.bodySmall),
                  const SizedBox(height: 8),
                  Wrap(
                    spacing: 8,
                    runSpacing: 4,
                    children: [
                      Chip(
                        label: Text(s.enabled ? 'desired:on' : 'desired:off'),
                        visualDensity: VisualDensity.compact,
                      ),
                      Chip(
                        label: Text('process:${s.process}'),
                        visualDensity: VisualDensity.compact,
                      ),
                      Chip(
                        label: Text(
                          s.gatewayOnline ? 'gateway:online' : 'gateway:offline',
                        ),
                        visualDensity: VisualDensity.compact,
                      ),
                    ],
                  ),
                  if (s.lastError != null && s.lastError!.isNotEmpty)
                    Padding(
                      padding: const EdgeInsets.only(top: 6),
                      child: Text(
                        s.lastError!,
                        style: TextStyle(
                          color: Theme.of(context).colorScheme.error,
                          fontSize: 12,
                        ),
                      ),
                    ),
                  const SizedBox(height: 8),
                  if (busy)
                    const LinearProgressIndicator()
                  else
                    Wrap(
                      spacing: 8,
                      children: [
                        TextButton(
                          onPressed: () => _runControl(
                            m.masterId,
                            s.id,
                            () => _api.startSlave(m.masterId, s.id),
                          ),
                          child: const Text('Start'),
                        ),
                        TextButton(
                          onPressed: () => _runControl(
                            m.masterId,
                            s.id,
                            () => _api.stopSlave(m.masterId, s.id),
                          ),
                          child: const Text('Stop'),
                        ),
                        TextButton(
                          onPressed: () => _runControl(
                            m.masterId,
                            s.id,
                            () => _api.restartSlave(m.masterId, s.id),
                          ),
                          child: const Text('Restart'),
                        ),
                        TextButton(
                          onPressed: () =>
                              _showSlaveForm(masterId: m.masterId, existing: s),
                          child: const Text('编辑'),
                        ),
                        TextButton(
                          onPressed: () => _delete(m.masterId, s),
                          child: const Text('删除'),
                        ),
                      ],
                    ),
                ],
              ),
            ),
          ),
        );
      }
    }

    return ListView(
      physics: const AlwaysScrollableScrollPhysics(),
      children: tiles,
    );
  }
}
