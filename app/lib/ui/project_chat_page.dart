import 'dart:async';

import 'package:flutter/material.dart';

import '../auth/session.dart';
import '../chat/chat_api.dart';
import '../chat/models.dart';
import '../slaves/models.dart';
import '../ws/app_ws_client.dart';
import '../ws/task_log_buffer.dart';

class _Bubble {
  _Bubble({
    required this.role,
    required this.text,
    this.taskId,
    this.refs = const [],
    this.streaming = false,
    this.done = false,
    this.cancelled = false,
  });

  final String role;
  String text;
  String? taskId;
  List<ChatRef> refs;
  bool streaming;
  bool done;
  bool cancelled;
}

/// Project chat: modes/models + history drawer (M09-P04).
class ProjectChatPage extends StatefulWidget {
  const ProjectChatPage({
    super.key,
    required this.session,
    required this.slave,
    required this.project,
    this.api,
    this.wsFactory,
  });

  final Session session;
  final SlaveInfo slave;
  final ProjectInfo project;
  final ChatApi? api;

  /// Test seam for WS client.
  final AppWsClient Function(TaskLogBuffer buffer, String taskId)? wsFactory;

  @override
  State<ProjectChatPage> createState() => _ProjectChatPageState();
}

class _ProjectChatPageState extends State<ProjectChatPage> {
  late final ChatApi _api = widget.api ?? ChatApi(session: widget.session);
  final _input = TextEditingController();
  final _scroll = ScrollController();
  final List<_Bubble> _bubbles = [];

  ChatSession? _session;
  List<ChatSession> _history = [];
  ModelCatalog _catalog = ModelCatalog.fallback;
  String _mode = 'agent';
  String _model = 'auto';
  bool _starting = true;
  bool _sending = false;
  bool _running = false;
  bool _loadingHistory = false;
  String? _error;
  String? _activeTaskId;
  ChatRef? _pendingRef;

  TaskLogBuffer? _buffer;
  AppWsClient? _ws;
  VoidCallback? _bufferListener;

  bool get _online => widget.slave.online;

  String get _modeLabel {
    switch (_mode) {
      case 'ask':
        return 'Ask';
      case 'plan':
        return 'Plan';
      default:
        return 'Agent';
    }
  }

  String get _modelLabel => _labelForModel(_model);

  String _labelForModel(String id) {
    for (final m in _catalog.models) {
      if (m.id == id) return m.label;
    }
    return id;
  }

  @override
  void initState() {
    super.initState();
    unawaited(_bootstrap());
  }

  @override
  void dispose() {
    unawaited(_detachStream());
    _input.dispose();
    _scroll.dispose();
    super.dispose();
  }

  Future<void> _bootstrap() async {
    setState(() {
      _starting = true;
      _error = null;
    });
    try {
      final catalog = await _api.listModels();
      if (!mounted) return;
      setState(() {
        _catalog = catalog;
        if (!_catalog.models.any((m) => m.id == _model)) {
          _model = catalog.defaultId;
        }
      });
    } catch (_) {
      // Keep fallback catalog; chat still works with Auto.
    }
    await _resumeOrBlank();
  }

  void _applySession(ChatSession sess, {bool replaceBubbles = true}) {
    _session = sess;
    _mode = sess.mode;
    _model = sess.model;
    if (replaceBubbles) {
      _bubbles
        ..clear()
        ..addAll(
          sess.messages.map(
            (m) => _Bubble(
              role: m.role,
              text: m.content,
              taskId: m.taskId,
              refs: m.refs,
              done: true,
            ),
          ),
        );
    }
  }

  /// Open latest session for this project, or leave a blank composer (lazy create).
  Future<void> _resumeOrBlank() async {
    setState(() {
      _starting = true;
      _error = null;
    });
    try {
      final list = await _api.listChats(
        slaveId: widget.slave.id,
        repoId: widget.project.id,
      );
      if (!mounted) return;
      if (list.isEmpty) {
        setState(() {
          _session = null;
          _bubbles.clear();
          _history = list;
          _starting = false;
        });
        return;
      }
      final sess = await _api.getChat(list.first.id);
      if (!mounted) return;
      setState(() {
        _history = list;
        _applySession(sess);
        _starting = false;
      });
      _scrollToEnd();
      _maybeResumeInFlight();
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _starting = false;
        _error = e.toString();
      });
    }
  }

  /// Create Gateway session only when the user sends the first message.
  Future<ChatSession?> _ensureSessionForSend() async {
    final existing = _session;
    if (existing != null) return existing;
    try {
      final sess = await _api.createChat(
        slaveId: widget.slave.id,
        repoId: widget.project.id,
        mode: _mode,
        model: _model,
      );
      if (!mounted) return null;
      setState(() => _applySession(sess, replaceBubbles: false));
      return sess;
    } catch (e) {
      if (!mounted) return null;
      setState(() => _error = e.toString());
      return null;
    }
  }

  Future<void> _refreshSessionTitle() async {
    final id = _session?.id;
    if (id == null) return;
    try {
      final sess = await _api.getChat(id);
      if (!mounted || _session?.id != id) return;
      setState(() {
        _session = _session!.copyWith(title: sess.title);
      });
    } catch (_) {
      // Best-effort; provisional title remains.
    }
  }

  Future<void> _renameChat() async {
    final sess = _session;
    if (sess == null) return;
    final controller = TextEditingController(text: sess.displayTitle);
    final next = await showDialog<String>(
      context: context,
      builder: (ctx) {
        return AlertDialog(
          title: const Text('Rename chat'),
          content: TextField(
            controller: controller,
            autofocus: true,
            maxLength: 64,
            decoration: const InputDecoration(
              hintText: 'Session title',
            ),
            onSubmitted: (v) => Navigator.pop(ctx, v.trim()),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(ctx),
              child: const Text('Cancel'),
            ),
            FilledButton(
              onPressed: () => Navigator.pop(ctx, controller.text.trim()),
              child: const Text('Save'),
            ),
          ],
        );
      },
    );
    controller.dispose();
    if (next == null || next.isEmpty || !mounted) return;
    try {
      final updated = await _api.renameChat(sess.id, next);
      if (!mounted) return;
      setState(() {
        _session = sess.copyWith(title: updated.title ?? next);
      });
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Rename failed: $e')),
      );
    }
  }

  Future<void> _openHistoryDrawer() async {
    await _refreshHistory();
    if (!mounted) return;
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      builder: (ctx) {
        return StatefulBuilder(
          builder: (ctx, setSheet) {
            final theme = Theme.of(ctx);
            return DraggableScrollableSheet(
              expand: false,
              initialChildSize: 0.55,
              minChildSize: 0.35,
              maxChildSize: 0.9,
              builder: (ctx, scroll) {
                return Column(
                  children: [
                    ListTile(
                      title: Text('History', style: theme.textTheme.titleMedium),
                      subtitle: Text(
                        widget.project.name.isNotEmpty
                            ? widget.project.name
                            : widget.project.id,
                      ),
                      trailing: IconButton(
                        tooltip: 'Refresh',
                        onPressed: _loadingHistory
                            ? null
                            : () async {
                                await _refreshHistory();
                                setSheet(() {});
                              },
                        icon: const Icon(Icons.refresh),
                      ),
                    ),
                    const Divider(height: 1),
                    if (_loadingHistory) const LinearProgressIndicator(),
                    Expanded(
                      child: _history.isEmpty
                          ? Center(
                              child: Text(
                                _loadingHistory
                                    ? 'Loading…'
                                    : 'No saved chats yet',
                                style: theme.textTheme.bodyMedium?.copyWith(
                                  color: theme.colorScheme.onSurfaceVariant,
                                ),
                              ),
                            )
                          : ListView.builder(
                              controller: scroll,
                              itemCount: _history.length,
                              itemBuilder: (context, i) {
                                final c = _history[i];
                                final selected = c.id == _session?.id;
                                return ListTile(
                                  selected: selected,
                                  leading: Icon(
                                    selected
                                        ? Icons.chat_bubble
                                        : Icons.chat_bubble_outline,
                                  ),
                                  title: Text(
                                    c.displayTitle,
                                    maxLines: 2,
                                    overflow: TextOverflow.ellipsis,
                                  ),
                                  subtitle: Text(
                                    '${c.mode} · ${c.model}'
                                    '${c.messageCount > 0 ? ' · ${c.messageCount} msgs' : ''}',
                                  ),
                                  onTap: () {
                                    Navigator.pop(ctx);
                                    unawaited(_continueChat(c));
                                  },
                                );
                              },
                            ),
                    ),
                  ],
                );
              },
            );
          },
        );
      },
    );
  }

  Future<void> _refreshHistory() async {
    setState(() => _loadingHistory = true);
    try {
      final list = await _api.listChats(
        slaveId: widget.slave.id,
        repoId: widget.project.id,
      );
      if (!mounted) return;
      setState(() {
        _history = list;
        _loadingHistory = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _loadingHistory = false;
        _error = e.toString();
      });
    }
  }

  Future<void> _continueChat(ChatSession summary) async {
    await _detachStream();
    setState(() {
      _starting = true;
      _error = null;
      _activeTaskId = null;
      _running = false;
      _sending = false;
    });
    try {
      final sess = await _api.getChat(summary.id);
      if (!mounted) return;
      setState(() {
        _applySession(sess);
        _starting = false;
      });
      _scrollToEnd();
      _maybeResumeInFlight();
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _starting = false;
        _error = e.toString();
      });
    }
  }

  /// If Gateway still has status=running, re-subscribe to the pending task stream.
  void _maybeResumeInFlight() {
    final sess = _session;
    if (sess == null || sess.status != 'running') return;
    final assisted = <String>{};
    for (final m in sess.messages) {
      final tid = m.taskId;
      if (m.role == 'assistant' && tid != null && tid.isNotEmpty) {
        assisted.add(tid);
      }
    }
    String? pending;
    for (var i = sess.messages.length - 1; i >= 0; i--) {
      final m = sess.messages[i];
      final tid = m.taskId;
      if (m.role == 'user' && tid != null && tid.isNotEmpty) {
        if (!assisted.contains(tid)) pending = tid;
        break;
      }
    }
    if (pending == null) return;
    final taskId = pending;
    setState(() {
      _activeTaskId = taskId;
      _running = true;
      final hasBubble = _bubbles.any(
        (b) => b.role == 'assistant' && b.taskId == taskId,
      );
      if (!hasBubble) {
        _bubbles.add(
          _Bubble(role: 'assistant', text: '…', streaming: true, taskId: taskId),
        );
      }
    });
    unawaited(_attachStream(taskId));
  }

  Future<void> _detachStream() async {
    final buf = _buffer;
    final listener = _bufferListener;
    if (buf != null && listener != null) {
      buf.removeListener(listener);
    }
    _bufferListener = null;
    _buffer = null;
    final ws = _ws;
    _ws = null;
    await ws?.dispose();
  }

  Future<void> _persistAssistant(String taskId, String text) async {
    final chatId = _session?.id;
    final content = text.trim();
    if (chatId == null || content.isEmpty || content == '…') return;
    try {
      await _api.recordAssistant(
        chatId: chatId,
        content: content,
        taskId: taskId,
      );
      await _refreshSessionTitle();
    } catch (_) {
      // Best-effort; history still has user turns.
    }
  }

  Future<void> _attachStream(String taskId) async {
    await _detachStream();
    final buf = TaskLogBuffer();
    var finalized = false;
    void onBuf() {
      if (!mounted) return;
      final text = buf.assistant.toString();
      setState(() {
        for (var i = _bubbles.length - 1; i >= 0; i--) {
          if (_bubbles[i].role == 'assistant' && _bubbles[i].taskId == taskId) {
            _bubbles[i].text = text.isEmpty ? '…' : text;
            break;
          }
        }
        for (final e in buf.events.reversed) {
          if (e.kind == 'done' || e.kind == 'error') {
            _running = false;
            _sending = false;
            final status = e.payload['status']?.toString() ?? '';
            final wasCancelled = status == 'cancelled';
            for (var i = _bubbles.length - 1; i >= 0; i--) {
              if (_bubbles[i].taskId == taskId) {
                _bubbles[i].streaming = false;
                if (wasCancelled || _bubbles[i].cancelled) {
                  _bubbles[i].cancelled = true;
                  _bubbles[i].done = false;
                } else {
                  _bubbles[i].done = true;
                  _bubbles[i].cancelled = false;
                }
                if (e.kind == 'error' && _bubbles[i].text == '…') {
                  _bubbles[i].text =
                      e.payload['message']?.toString() ?? 'error';
                }
                if (!finalized && !_bubbles[i].cancelled) {
                  finalized = true;
                  unawaited(_persistAssistant(taskId, _bubbles[i].text));
                }
                break;
              }
            }
            break;
          }
        }
      });
      _scrollToEnd();
    }

    buf.addListener(onBuf);
    _buffer = buf;
    _bufferListener = onBuf;

    // HTTP snapshot first so late WS subscribe does not miss early deltas.
    try {
      final snap = await _api.getTaskEvents(taskId, afterSeq: 0);
      for (final m in snap.events) {
        buf.ingest(TaskLogEvent.fromJson(m));
      }
    } catch (_) {
      // Best-effort; WS replay still covers history when available.
    }

    final ws = widget.wsFactory?.call(buf, taskId) ??
        AppWsClient(session: widget.session, buffer: buf);
    _ws = ws;
    await ws.start(taskId: taskId);
  }

  void _scrollToEnd() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!_scroll.hasClients) return;
      _scroll.animateTo(
        _scroll.position.maxScrollExtent,
        duration: const Duration(milliseconds: 180),
        curve: Curves.easeOut,
      );
    });
  }

  Future<void> _send() async {
    final text = _input.text.trim();
    if (text.isEmpty || _sending || _running) return;
    if (!_online) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Slave offline')),
      );
      return;
    }

    final cite = _pendingRef;
    setState(() {
      _sending = true;
      _error = null;
      _bubbles.add(
        _Bubble(
          role: 'user',
          text: text,
          refs: cite == null ? const [] : [cite],
        ),
      );
      _bubbles.add(
        _Bubble(role: 'assistant', text: '…', streaming: true),
      );
      _input.clear();
      _pendingRef = null;
    });
    _scrollToEnd();

    final wasNew = _session == null;
    final sess = await _ensureSessionForSend();
    if (sess == null) {
      if (!mounted) return;
      setState(() {
        _sending = false;
        if (_bubbles.isNotEmpty && _bubbles.last.role == 'assistant') {
          _bubbles.last
            ..text = 'Failed: ${_error ?? 'create chat'}'
            ..streaming = false
            ..done = true;
        }
      });
      return;
    }

    try {
      final result = await _api.sendMessage(
        chatId: sess.id,
        text: text,
        mode: _mode,
        model: _model,
        refs: cite == null ? null : [cite],
      );
      if (!mounted) return;
      setState(() {
        _activeTaskId = result.taskId;
        _running = true;
        _sending = false;
        if (_bubbles.isNotEmpty && _bubbles.last.role == 'assistant') {
          _bubbles.last.taskId = result.taskId;
        }
      });
      await _attachStream(result.taskId);
      if (wasNew) {
        // Provisional title from Gateway; AI title arrives via Slave shortly.
        unawaited(_refreshSessionTitle());
        Future<void>.delayed(const Duration(seconds: 3), () {
          if (mounted) unawaited(_refreshSessionTitle());
        });
        Future<void>.delayed(const Duration(seconds: 8), () {
          if (mounted) unawaited(_refreshSessionTitle());
        });
      }
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _sending = false;
        _running = false;
        _error = e.toString();
        if (_bubbles.isNotEmpty && _bubbles.last.role == 'assistant') {
          _bubbles.last
            ..text = 'Failed: $e'
            ..streaming = false
            ..done = true;
        }
      });
    }
  }

  Future<void> _stop() async {
    final chatId = _session?.id;
    final taskId = _activeTaskId;
    try {
      if (chatId != null) {
        await _api.stopChat(chatId);
      } else if (taskId != null) {
        await _api.cancelTask(taskId);
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Stop failed: $e')),
        );
      }
    }
    if (!mounted) return;
    setState(() {
      _running = false;
      _sending = false;
      for (final b in _bubbles) {
        if (b.streaming || (b.taskId != null && b.taskId == taskId)) {
          b.streaming = false;
          b.done = false;
          b.cancelled = true;
        }
      }
    });
  }

  Future<void> _newChat() async {
    await _detachStream();
    setState(() {
      _bubbles.clear();
      _activeTaskId = null;
      _pendingRef = null;
      _running = false;
      _sending = false;
      _session = null;
      _error = null;
      _starting = false;
    });
    if (mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('New chat — send to start')),
      );
    }
  }

  String get _appBarTitle {
    final project = widget.project.name.isNotEmpty
        ? widget.project.name
        : widget.project.id;
    final sess = _session;
    if (sess == null) return 'New chat · $project';
    return '${sess.displayTitle} · $project';
  }

  Future<void> _openAttachSheet() async {
    if (_starting || !_online || _sending || _running) return;
    await showModalBottomSheet<void>(
      context: context,
      builder: (ctx) {
        final theme = Theme.of(ctx);
        return SafeArea(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              ListTile(
                title: Text('Add', style: theme.textTheme.titleMedium),
              ),
              const Divider(height: 1),
              ListTile(
                leading: const Icon(Icons.account_tree_outlined),
                title: const Text('Plan'),
                subtitle: const Text('Cite a milestone phase'),
                onTap: () {
                  Navigator.pop(ctx);
                  unawaited(_openPlanPicker());
                },
              ),
              const SizedBox(height: 8),
            ],
          ),
        );
      },
    );
  }

  Future<void> _openPlanPicker() async {
    final milestones = widget.project.milestones;
    final hasPhase = milestones.any((m) => m.phases.isNotEmpty);
    final selected = await showModalBottomSheet<ChatRef>(
      context: context,
      isScrollControlled: true,
      builder: (ctx) {
        final theme = Theme.of(ctx);
        return DraggableScrollableSheet(
          expand: false,
          initialChildSize: 0.6,
          minChildSize: 0.4,
          maxChildSize: 0.92,
          builder: (ctx, scroll) {
            if (!hasPhase) {
              return Column(
                children: [
                  ListTile(
                    title: Text('Plan', style: theme.textTheme.titleMedium),
                    subtitle: const Text('Milestone phases'),
                  ),
                  const Divider(height: 1),
                  Expanded(
                    child: Center(
                      child: Text(
                        '当前工程无 plan',
                        style: theme.textTheme.bodyMedium?.copyWith(
                          color: theme.colorScheme.onSurfaceVariant,
                        ),
                      ),
                    ),
                  ),
                ],
              );
            }
            return Column(
              children: [
                ListTile(
                  title: Text('Plan', style: theme.textTheme.titleMedium),
                  subtitle: const Text('Select a phase to cite'),
                ),
                const Divider(height: 1),
                Expanded(
                  child: ListView.builder(
                    controller: scroll,
                    itemCount: milestones.length,
                    itemBuilder: (context, mi) {
                      final ms = milestones[mi];
                      if (ms.phases.isEmpty) {
                        return const SizedBox.shrink();
                      }
                      return Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Padding(
                            padding: const EdgeInsets.fromLTRB(16, 12, 16, 4),
                            child: Text(
                              '${ms.id} · ${ms.title}',
                              style: theme.textTheme.labelLarge?.copyWith(
                                color: theme.colorScheme.primary,
                              ),
                            ),
                          ),
                          ...ms.phases.map((p) {
                            return ListTile(
                              leading: const Icon(Icons.description_outlined),
                              title: Text('${p.id} · ${p.title}'),
                              subtitle: p.phaseRef.isEmpty
                                  ? null
                                  : Text(
                                      p.phaseRef,
                                      maxLines: 1,
                                      overflow: TextOverflow.ellipsis,
                                    ),
                              onTap: () {
                                Navigator.pop(
                                  ctx,
                                  ChatRef(
                                    kind: 'phase',
                                    id: p.id,
                                    title: p.title,
                                    milestoneId: ms.id,
                                    phaseRef: p.phaseRef,
                                  ),
                                );
                              },
                            );
                          }),
                        ],
                      );
                    },
                  ),
                ),
              ],
            );
          },
        );
      },
    );
    if (selected == null || !mounted) return;
    setState(() => _pendingRef = selected);
  }

  Widget _refChip(ChatRef ref, {VoidCallback? onDeleted}) {
    return InputChip(
      avatar: const Icon(Icons.account_tree_outlined, size: 16),
      label: Text(ref.label, overflow: TextOverflow.ellipsis),
      onDeleted: onDeleted,
      materialTapTargetSize: MaterialTapTargetSize.shrinkWrap,
      visualDensity: VisualDensity.compact,
    );
  }

  Widget _assistantStatusLabel(BuildContext context, _Bubble b) {
    final theme = Theme.of(context);
    final style = theme.textTheme.labelSmall;
    if (b.streaming) {
      return Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          SizedBox(
            width: 12,
            height: 12,
            child: CircularProgressIndicator(
              strokeWidth: 1.8,
              color: theme.colorScheme.primary,
            ),
          ),
          const SizedBox(width: 6),
          Text('Think', style: style),
        ],
      );
    }
    if (b.cancelled) {
      return Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          const Icon(
            Icons.cancel,
            size: 14,
            color: Color(0xFFC62828),
          ),
          const SizedBox(width: 4),
          Text(
            'Cancel',
            style: style?.copyWith(color: const Color(0xFFC62828)),
          ),
        ],
      );
    }
    if (b.done) {
      return Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          const Icon(
            Icons.check_circle,
            size: 14,
            color: Color(0xFF2E7D32),
          ),
          const SizedBox(width: 4),
          Text(
            'Done',
            style: style?.copyWith(color: const Color(0xFF2E7D32)),
          ),
        ],
      );
    }
    return Text('Assistant', style: style);
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final modelIds = _catalog.models.map((m) => m.id).toList();
    if (!modelIds.contains(_model)) {
      modelIds.insert(0, _model);
    }

    return Scaffold(
      appBar: AppBar(
        title: InkWell(
          onTap: _session == null || _starting
              ? null
              : () => unawaited(_renameChat()),
          child: Text(
            _appBarTitle,
            style: const TextStyle(fontSize: 16),
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
          ),
        ),
        actions: [
          if (_session != null)
            IconButton(
              tooltip: 'Rename',
              onPressed: _starting ? null : () => unawaited(_renameChat()),
              icon: const Icon(Icons.edit_outlined),
            ),
          IconButton(
            tooltip: 'History',
            onPressed: _starting ? null : () => unawaited(_openHistoryDrawer()),
            icon: const Icon(Icons.history),
          ),
          IconButton(
            tooltip: 'New chat',
            onPressed: _starting ? null : () => unawaited(_newChat()),
            icon: const Icon(Icons.add_comment_outlined),
          ),
        ],
      ),
      body: Column(
        children: [
          Material(
            color: theme.colorScheme.surfaceContainerLow,
            child: Padding(
              padding: const EdgeInsets.fromLTRB(12, 8, 12, 8),
              child: Row(
                children: [
                  Expanded(
                    child: SegmentedButton<String>(
                      segments: const [
                        ButtonSegment(value: 'agent', label: Text('Agent')),
                        ButtonSegment(value: 'ask', label: Text('Ask')),
                        ButtonSegment(value: 'plan', label: Text('Plan')),
                      ],
                      selected: {_mode},
                      onSelectionChanged: _starting || _running || _sending
                          ? null
                          : (next) {
                              if (next.isEmpty) return;
                              setState(() => _mode = next.first);
                            },
                      style: const ButtonStyle(
                        visualDensity: VisualDensity.compact,
                        tapTargetSize: MaterialTapTargetSize.shrinkWrap,
                      ),
                    ),
                  ),
                  const SizedBox(width: 8),
                  DropdownButtonHideUnderline(
                    child: DropdownButton<String>(
                      value: modelIds.contains(_model) ? _model : modelIds.first,
                      items: [
                        for (final id in modelIds)
                          DropdownMenuItem(
                            value: id,
                            child: Text(_labelForModel(id)),
                          ),
                      ],
                      onChanged: _starting || _running || _sending
                          ? null
                          : (v) {
                              if (v == null) return;
                              setState(() => _model = v);
                            },
                    ),
                  ),
                ],
              ),
            ),
          ),
          if (!_online)
            Material(
              color: theme.colorScheme.errorContainer,
              child: const ListTile(
                dense: true,
                leading: Icon(Icons.cloud_off),
                title: Text('Slave offline — send disabled'),
              ),
            ),
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
          if (_starting) const LinearProgressIndicator(),
          Expanded(
            child: _bubbles.isEmpty
                ? Center(
                    child: Text(
                      _starting
                          ? 'Loading chat…'
                          : _session == null
                              ? 'New chat — send a message to start\n($_modeLabel · $_modelLabel)'
                              : 'Say what you want ($_modeLabel · $_modelLabel).',
                      style: theme.textTheme.bodyLarge?.copyWith(
                        color: theme.colorScheme.onSurfaceVariant,
                      ),
                      textAlign: TextAlign.center,
                    ),
                  )
                : ListView.builder(
                    controller: _scroll,
                    padding: const EdgeInsets.fromLTRB(12, 12, 12, 8),
                    itemCount: _bubbles.length,
                    itemBuilder: (context, i) {
                      final b = _bubbles[i];
                      final isUser = b.role == 'user';
                      return Align(
                        alignment: isUser
                            ? Alignment.centerRight
                            : Alignment.centerLeft,
                        child: ConstrainedBox(
                          constraints: BoxConstraints(
                            maxWidth: MediaQuery.sizeOf(context).width * 0.86,
                          ),
                          child: Card(
                            color: isUser
                                ? theme.colorScheme.primaryContainer
                                : theme.colorScheme.surfaceContainerHighest,
                            margin: const EdgeInsets.symmetric(vertical: 4),
                            child: Padding(
                              padding: const EdgeInsets.all(12),
                              child: Column(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  isUser
                                      ? Text('You', style: theme.textTheme.labelSmall)
                                      : _assistantStatusLabel(context, b),
                                  if (b.refs.isNotEmpty) ...[
                                    const SizedBox(height: 6),
                                    Wrap(
                                      spacing: 6,
                                      runSpacing: 4,
                                      children: [
                                        for (final r in b.refs) _refChip(r),
                                      ],
                                    ),
                                  ],
                                  const SizedBox(height: 4),
                                  SelectableText(b.text),
                                ],
                              ),
                            ),
                          ),
                        ),
                      );
                    },
                  ),
          ),
          SafeArea(
            top: false,
            child: Padding(
              padding: const EdgeInsets.fromLTRB(4, 4, 8, 8),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  if (_pendingRef != null)
                    Padding(
                      padding: const EdgeInsets.fromLTRB(8, 0, 0, 6),
                      child: Align(
                        alignment: Alignment.centerLeft,
                        child: _refChip(
                          _pendingRef!,
                          onDeleted: () => setState(() => _pendingRef = null),
                        ),
                      ),
                    ),
                  Row(
                    crossAxisAlignment: CrossAxisAlignment.end,
                    children: [
                      IconButton(
                        tooltip: 'Add',
                        onPressed: !_starting &&
                                _online &&
                                !_sending &&
                                !_running
                            ? () => unawaited(_openAttachSheet())
                            : null,
                        icon: const Icon(Icons.add_circle_outline),
                      ),
                      Expanded(
                        child: TextField(
                          controller: _input,
                          minLines: 1,
                          maxLines: 5,
                          enabled: !_starting && _online,
                          textInputAction: TextInputAction.send,
                          onSubmitted: (_) => unawaited(_send()),
                          decoration: InputDecoration(
                            hintText: 'Message ($_modeLabel · $_modelLabel)',
                            border: const OutlineInputBorder(),
                            isDense: true,
                          ),
                        ),
                      ),
                      const SizedBox(width: 8),
                      if (_running || _sending)
                        IconButton.filled(
                          tooltip: 'Stop',
                          onPressed: _stop,
                          icon: const Icon(Icons.stop),
                        )
                      else
                        IconButton.filled(
                          tooltip: 'Send',
                          onPressed: _online && !_starting
                              ? () => unawaited(_send())
                              : null,
                          icon: const Icon(Icons.send),
                        ),
                    ],
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}
