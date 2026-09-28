import 'dart:async';

import 'package:flutter/material.dart';

import '../auth/session.dart';
import '../workflow/models.dart';
import '../workflow/workflow_api.dart';
import 'node_status_chip.dart';
import 'workflow_detail_page.dart';

/// Lists Gateway workflows; pull-to-refresh + light polling.
class WorkflowListPage extends StatefulWidget {
  const WorkflowListPage({
    super.key,
    required this.session,
    this.api,
    this.pollInterval = const Duration(seconds: 5),
  });

  final Session session;
  final WorkflowApi? api;
  final Duration pollInterval;

  @override
  State<WorkflowListPage> createState() => _WorkflowListPageState();
}

class _WorkflowListPageState extends State<WorkflowListPage> {
  late final WorkflowApi _api =
      widget.api ?? WorkflowApi(session: widget.session);

  List<WorkflowRun>? _items;
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
      final list = await _api.listWorkflows();
      if (!mounted) return;
      setState(() {
        _items = list;
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
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _error = 'Failed to load workflows';
        _loading = false;
      });
    }
  }

  Future<void> _openDetail(WorkflowRun run) async {
    await Navigator.of(context).push<void>(
      MaterialPageRoute(
        builder: (_) => WorkflowDetailPage(
          session: widget.session,
          workflowId: run.id,
          api: _api,
          initial: run,
        ),
      ),
    );
    if (mounted) await _refresh(silent: true);
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Workflows'),
        actions: [
          IconButton(
            tooltip: 'Refresh',
            onPressed: _loading ? null : () => _refresh(),
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
    if (_loading && _items == null) {
      return ListView(
        physics: const AlwaysScrollableScrollPhysics(),
        children: const [
          SizedBox(height: 120),
          Center(child: CircularProgressIndicator()),
        ],
      );
    }

    if (_error != null && (_items == null || _items!.isEmpty)) {
      return ListView(
        physics: const AlwaysScrollableScrollPhysics(),
        padding: const EdgeInsets.all(24),
        children: [
          Text(_error!, style: TextStyle(color: Theme.of(context).colorScheme.error)),
          const SizedBox(height: 16),
          FilledButton(onPressed: _refresh, child: const Text('Retry')),
        ],
      );
    }

    final items = _items ?? const <WorkflowRun>[];
    if (items.isEmpty) {
      return ListView(
        physics: const AlwaysScrollableScrollPhysics(),
        padding: const EdgeInsets.all(24),
        children: const [
          Text(
            'No workflows yet.\n'
            'Create one via Gateway POST /v1/workflows (App does not edit DAG JSON).',
          ),
        ],
      );
    }

    return ListView.separated(
      physics: const AlwaysScrollableScrollPhysics(),
      padding: const EdgeInsets.symmetric(vertical: 8),
      itemCount: items.length,
      separatorBuilder: (_, __) => const Divider(height: 1),
      itemBuilder: (context, i) {
        final w = items[i];
        final awaiting =
            w.nodes.where((n) => n.isAwaitingReview).length;
        return ListTile(
          title: Text(w.id, maxLines: 1, overflow: TextOverflow.ellipsis),
          subtitle: Text(
            [
              'bundle ${w.bundleId}',
              if (awaiting > 0) '$awaiting awaiting review',
            ].join(' · '),
          ),
          trailing: WorkflowStatusChip(status: w.status),
          onTap: () => _openDetail(w),
        );
      },
    );
  }
}
