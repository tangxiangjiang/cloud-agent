import 'package:flutter/material.dart';

import '../auth/session.dart';
import '../workflow/diff_models.dart';
import '../workflow/workflow_api.dart';

/// Read-only unified diff viewer. No edit / save / apply-patch controls.
class NodeDiffPage extends StatefulWidget {
  const NodeDiffPage({
    super.key,
    required this.session,
    required this.workflowId,
    required this.nodeId,
    this.api,
  });

  final Session session;
  final String workflowId;
  final String nodeId;
  final WorkflowApi? api;

  @override
  State<NodeDiffPage> createState() => _NodeDiffPageState();
}

class _NodeDiffPageState extends State<NodeDiffPage> {
  late final WorkflowApi _api =
      widget.api ?? WorkflowApi(session: widget.session);

  NodeDiff? _diff;
  String? _error;
  bool _loading = true;
  int _fileIndex = 0;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final d = await _api.getNodeDiff(widget.workflowId, widget.nodeId);
      if (!mounted) return;
      setState(() {
        _diff = d;
        _fileIndex = 0;
        _loading = false;
      });
    } on WorkflowApiException catch (e) {
      if (!mounted) return;
      setState(() {
        _error = e.message;
        _loading = false;
      });
    } catch (_) {
      if (!mounted) return;
      setState(() {
        _error = 'Failed to load diff';
        _loading = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: Text('Diff · ${widget.nodeId}'),
        actions: [
          IconButton(
            tooltip: 'Reload',
            onPressed: _loading ? null : _load,
            icon: const Icon(Icons.refresh),
          ),
        ],
      ),
      body: _buildBody(),
    );
  }

  Widget _buildBody() {
    if (_loading) {
      return const Center(child: CircularProgressIndicator());
    }
    if (_error != null) {
      return Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              _error!,
              style: TextStyle(color: Theme.of(context).colorScheme.error),
            ),
            const SizedBox(height: 16),
            FilledButton(onPressed: _load, child: const Text('Retry')),
          ],
        ),
      );
    }
    final diff = _diff!;
    if (diff.files.isEmpty) {
      return const Padding(
        padding: EdgeInsets.all(24),
        child: Text('No file changes in this diff.'),
      );
    }
    final file = diff.files[_fileIndex.clamp(0, diff.files.length - 1)];
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 12, 16, 4),
          child: Text(
            'baseline: ${diff.baseline ?? '(none)'} · read-only',
            style: Theme.of(context).textTheme.labelSmall,
          ),
        ),
        SizedBox(
          height: 48,
          child: ListView.separated(
            scrollDirection: Axis.horizontal,
            padding: const EdgeInsets.symmetric(horizontal: 12),
            itemCount: diff.files.length,
            separatorBuilder: (_, __) => const SizedBox(width: 8),
            itemBuilder: (context, i) {
              final f = diff.files[i];
              final selected = i == _fileIndex;
              return FilterChip(
                selected: selected,
                label: Text(f.path),
                onSelected: (_) => setState(() => _fileIndex = i),
              );
            },
          ),
        ),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 4),
          child: Text(
            [
              if (file.status != null) file.status!,
              if (file.additions != null) '+${file.additions}',
              if (file.deletions != null) '-${file.deletions}',
            ].join(' · '),
            style: Theme.of(context).textTheme.labelMedium,
          ),
        ),
        const Divider(height: 1),
        Expanded(
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(12),
            child: _UnifiedDiffView(text: file.unifiedDiff ?? '(no patch)'),
          ),
        ),
      ],
    );
  }
}

/// Selectable, non-editable colored unified diff.
class _UnifiedDiffView extends StatelessWidget {
  const _UnifiedDiffView({required this.text});

  final String text;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final spans = <TextSpan>[];
    for (final line in text.split('\n')) {
      Color? color;
      if (line.startsWith('+') && !line.startsWith('+++')) {
        color = const Color(0xFF1B7F3A);
      } else if (line.startsWith('-') && !line.startsWith('---')) {
        color = scheme.error;
      } else if (line.startsWith('@@')) {
        color = scheme.primary;
      }
      spans.add(
        TextSpan(
          text: '$line\n',
          style: TextStyle(
            color: color ?? scheme.onSurface,
            fontFamily: 'monospace',
            fontSize: 12,
            height: 1.35,
          ),
        ),
      );
    }
    return SelectableText.rich(TextSpan(children: spans));
  }
}
