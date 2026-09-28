import 'package:flutter/material.dart';

import '../auth/session.dart';
import '../workflow/workflow_api.dart';
import '../ws/app_ws_client.dart';
import '../ws/task_log_buffer.dart';

/// Live task logs via App WS; HTTP `afterSeq` snapshot on enter / reconnect path.
class NodeLogsPage extends StatefulWidget {
  const NodeLogsPage({
    super.key,
    required this.session,
    required this.taskId,
    required this.nodeId,
    this.api,
    this.wsFactory,
  });

  final Session session;
  final String taskId;
  final String nodeId;
  final WorkflowApi? api;

  /// Test seam: provide a preconfigured client (already started or not).
  final AppWsClient Function(TaskLogBuffer buffer)? wsFactory;

  @override
  State<NodeLogsPage> createState() => _NodeLogsPageState();
}

class _NodeLogsPageState extends State<NodeLogsPage> {
  late final TaskLogBuffer _buffer = TaskLogBuffer();
  late final WorkflowApi _api =
      widget.api ?? WorkflowApi(session: widget.session);
  AppWsClient? _ws;
  String? _error;
  bool _loadingSnap = true;

  @override
  void initState() {
    super.initState();
    _bootstrap();
  }

  Future<void> _bootstrap() async {
    try {
      final snap = await _api.getTaskEvents(widget.taskId, afterSeq: 0);
      for (final m in snap.events) {
        _buffer.ingest(TaskLogEvent.fromJson(m));
      }
    } on WorkflowApiException catch (e) {
      _error = e.message;
    } catch (_) {
      _error = 'Failed to load event snapshot';
    }
    if (!mounted) return;
    setState(() => _loadingSnap = false);

    final ws = widget.wsFactory?.call(_buffer) ??
        AppWsClient(session: widget.session, buffer: _buffer);
    _ws = ws;
    await ws.start(taskId: widget.taskId);
  }

  @override
  void dispose() {
    // DoD: leaving the page cancels subscription / closes socket.
    final ws = _ws;
    _ws = null;
    ws?.dispose();
    _buffer.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: Text('Logs · ${widget.nodeId}'),
        actions: [
          IconButton(
            tooltip: 'HTTP snapshot',
            onPressed: _reloadSnapshot,
            icon: const Icon(Icons.cloud_download_outlined),
          ),
        ],
      ),
      body: Column(
        children: [
          if (_error != null)
            MaterialBanner(
              content: Text(_error!),
              actions: [
                TextButton(
                  onPressed: () => setState(() => _error = null),
                  child: const Text('Dismiss'),
                ),
              ],
            ),
          if (_loadingSnap) const LinearProgressIndicator(),
          Expanded(
            child: ListenableBuilder(
              listenable: _buffer,
              builder: (context, _) {
                final text = _buffer.assistant.toString();
                final lines = _buffer.events
                    .where((e) => e.kind != 'assistant.delta')
                    .toList();
                return ListView(
                  padding: const EdgeInsets.all(16),
                  children: [
                    Text(
                      'task ${widget.taskId} · lastSeq ${_buffer.lastSeq} · WS',
                      style: Theme.of(context).textTheme.labelSmall,
                    ),
                    const SizedBox(height: 12),
                    Text(
                      'Assistant',
                      style: Theme.of(context).textTheme.titleSmall,
                    ),
                    const SizedBox(height: 4),
                    SelectableText(
                      text.isEmpty ? '(waiting for assistant.delta…)' : text,
                      style: const TextStyle(
                        fontFamily: 'monospace',
                        fontSize: 13,
                        height: 1.35,
                      ),
                    ),
                    const SizedBox(height: 20),
                    Text(
                      'Events',
                      style: Theme.of(context).textTheme.titleSmall,
                    ),
                    const SizedBox(height: 4),
                    ...lines.map(
                      (e) => Padding(
                        padding: const EdgeInsets.only(bottom: 6),
                        child: Text(
                          '#${e.seq} ${e.displayText}',
                          style: const TextStyle(
                            fontFamily: 'monospace',
                            fontSize: 12,
                          ),
                        ),
                      ),
                    ),
                  ],
                );
              },
            ),
          ),
        ],
      ),
    );
  }

  Future<void> _reloadSnapshot() async {
    try {
      final snap =
          await _api.getTaskEvents(widget.taskId, afterSeq: _buffer.lastSeq);
      for (final m in snap.events) {
        _buffer.ingest(TaskLogEvent.fromJson(m));
      }
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Snapshot aligned')),
        );
      }
    } on WorkflowApiException catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(e.message)),
        );
      }
    }
  }
}
