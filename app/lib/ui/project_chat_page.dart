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
    this.streaming = false,
    this.done = false,
  });

  final String role;
  String text;
  String? taskId;
  bool streaming;
  bool done;
}

/// Project chat: Agent mode + Auto model (M09-P02).
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
  bool _starting = true;
  bool _sending = false;
  bool _running = false;
  String? _error;
  String? _activeTaskId;

  TaskLogBuffer? _buffer;
  AppWsClient? _ws;
  VoidCallback? _bufferListener;

  bool get _online => widget.slave.online;

  @override
  void initState() {
    super.initState();
    unawaited(_ensureSession());
  }

  @override
  void dispose() {
    unawaited(_detachStream());
    _input.dispose();
    _scroll.dispose();
    super.dispose();
  }

  Future<void> _ensureSession() async {
    setState(() {
      _starting = true;
      _error = null;
    });
    try {
      final sess = await _api.createChat(
        slaveId: widget.slave.id,
        repoId: widget.project.id,
        mode: 'agent',
        model: 'auto',
      );
      if (!mounted) return;
      setState(() {
        _session = sess;
        _starting = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _starting = false;
        _error = e.toString();
      });
    }
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

  Future<void> _attachStream(String taskId) async {
    await _detachStream();
    final buf = TaskLogBuffer();
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
            for (var i = _bubbles.length - 1; i >= 0; i--) {
              if (_bubbles[i].taskId == taskId) {
                _bubbles[i].streaming = false;
                _bubbles[i].done = true;
                if (e.kind == 'error' && _bubbles[i].text == '…') {
                  _bubbles[i].text =
                      e.payload['message']?.toString() ?? 'error';
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
    var sess = _session;
    if (sess == null) {
      await _ensureSession();
      sess = _session;
      if (sess == null) return;
    }

    setState(() {
      _sending = true;
      _error = null;
      _bubbles.add(_Bubble(role: 'user', text: text));
      _bubbles.add(
        _Bubble(role: 'assistant', text: '…', streaming: true),
      );
      _input.clear();
    });
    _scrollToEnd();

    try {
      final result = await _api.sendMessage(
        chatId: sess.id,
        text: text,
        mode: 'agent',
        model: 'auto',
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
        if (b.streaming) {
          b.streaming = false;
          b.done = true;
        }
      }
    });
  }

  Future<void> _newChat() async {
    await _detachStream();
    setState(() {
      _bubbles.clear();
      _activeTaskId = null;
      _running = false;
      _sending = false;
      _session = null;
      _error = null;
    });
    await _ensureSession();
    if (mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('New chat')),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final title = widget.project.name.isNotEmpty
        ? widget.project.name
        : widget.project.id;

    return Scaffold(
      appBar: AppBar(
        title: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('Chat · $title', style: const TextStyle(fontSize: 16)),
            Text(
              'Agent · Auto',
              style: theme.textTheme.bodySmall?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
            ),
          ],
        ),
        actions: [
          IconButton(
            tooltip: 'New chat',
            onPressed: _starting ? null : _newChat,
            icon: const Icon(Icons.add_comment_outlined),
          ),
        ],
      ),
      body: Column(
        children: [
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
                          ? 'Starting chat…'
                          : 'Say what you want to change in this project.',
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
                                  Text(
                                    isUser
                                        ? 'You'
                                        : (b.streaming
                                            ? 'Assistant…'
                                            : 'Assistant'),
                                    style: theme.textTheme.labelSmall,
                                  ),
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
              padding: const EdgeInsets.fromLTRB(8, 4, 8, 8),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.end,
                children: [
                  Expanded(
                    child: TextField(
                      controller: _input,
                      minLines: 1,
                      maxLines: 5,
                      enabled: !_starting && _online,
                      textInputAction: TextInputAction.send,
                      onSubmitted: (_) => unawaited(_send()),
                      decoration: const InputDecoration(
                        hintText: 'Message (Agent · Auto)',
                        border: OutlineInputBorder(),
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
                      onPressed: _online && !_starting ? () => unawaited(_send()) : null,
                      icon: const Icon(Icons.send),
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
