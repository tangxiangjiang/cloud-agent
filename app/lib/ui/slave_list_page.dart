import 'dart:async';

import 'package:flutter/material.dart';

import '../auth/session.dart';
import '../slaves/models.dart';
import '../slaves/slaves_api.dart';
import 'models_page.dart';
import 'project_list_page.dart';

/// Step 1: Slave 列表 → 工程列表.
class SlaveListPage extends StatefulWidget {
  const SlaveListPage({
    super.key,
    required this.session,
    this.api,
    this.onSignOut,
    this.pollInterval = const Duration(seconds: 8),
  });

  final Session session;
  final SlavesApi? api;
  final VoidCallback? onSignOut;
  final Duration pollInterval;

  @override
  State<SlaveListPage> createState() => _SlaveListPageState();
}

class _SlaveListPageState extends State<SlaveListPage> {
  late final SlavesApi _api = widget.api ?? SlavesApi(session: widget.session);

  List<SlaveInfo>? _items;
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
      final list = await _api.listSlaves();
      if (!mounted) return;
      setState(() {
        _items = list;
        _loading = false;
        _error = null;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        if (!silent || _items == null) {
          _error = e.toString();
        }
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Slaves'),
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
      body: RefreshIndicator(
        onRefresh: () => _refresh(),
        child: _buildBody(),
      ),
    );
  }

  Widget _buildBody() {
    if (_loading && _items == null) {
      return ListView(
        physics: const AlwaysScrollableScrollPhysics(),
        children: const [
          SizedBox(height: 120),
          Center(child: CircularProgressIndicator()),
        ],
      );
    }
    if (_error != null && _items == null) {
      return ListView(
        physics: const AlwaysScrollableScrollPhysics(),
        padding: const EdgeInsets.all(24),
        children: [
          Text(_error!, style: TextStyle(color: Theme.of(context).colorScheme.error)),
          const SizedBox(height: 16),
          FilledButton(onPressed: () => _refresh(), child: const Text('Retry')),
        ],
      );
    }
    final items = _items ?? const <SlaveInfo>[];
    if (items.isEmpty) {
      return ListView(
        physics: const AlwaysScrollableScrollPhysics(),
        padding: const EdgeInsets.all(24),
        children: [
          Text(
            'No slaves registered.',
            style: Theme.of(context).textTheme.titleMedium,
          ),
          const SizedBox(height: 8),
          Text(
            'Gateway: ${widget.session.gatewayBaseUrl}\n'
            'Start a Local Slave with GATEWAY_TOKEN.',
            style: Theme.of(context).textTheme.bodySmall,
          ),
        ],
      );
    }
    return ListView.builder(
      physics: const AlwaysScrollableScrollPhysics(),
      itemCount: items.length,
      itemBuilder: (context, i) {
        final s = items[i];
        final projects = s.effectiveProjects;
        return ListTile(
          leading: Icon(
            s.online ? Icons.cloud_done : Icons.cloud_off,
            color: s.online
                ? Theme.of(context).colorScheme.primary
                : Theme.of(context).disabledColor,
          ),
          title: Text(s.name.isNotEmpty ? s.name : s.id),
          subtitle: Text(
            '${s.id} · ${s.online ? 'online' : 'offline'}'
            ' · ${projects.length} project${projects.length == 1 ? '' : 's'}',
          ),
          trailing: const Icon(Icons.chevron_right),
          onTap: () {
            Navigator.of(context).push<void>(
              MaterialPageRoute(
                builder: (_) => ProjectListPage(
                  session: widget.session,
                  slave: s,
                ),
              ),
            );
          },
        );
      },
    );
  }
}
