import 'package:flutter/foundation.dart';

/// One task.event line for the log UI.
class TaskLogEvent {
  const TaskLogEvent({
    required this.seq,
    required this.kind,
    required this.payload,
    this.at,
    this.taskId,
  });

  final int seq;
  final String kind;
  final Map<String, dynamic> payload;
  final String? at;
  final String? taskId;

  String get displayText {
    switch (kind) {
      case 'assistant.delta':
        return (payload['text'] as String?) ?? '';
      case 'status':
        return 'status: ${payload['status'] ?? payload}';
      case 'error':
        return 'error: ${payload['message'] ?? payload}';
      case 'done':
        return 'done: ${payload['status'] ?? ''}';
      case 'tool.started':
        return 'tool ▶ ${payload['name'] ?? ''} ${payload['summary'] ?? ''}';
      case 'tool.finished':
        return 'tool ■ ${payload['name'] ?? ''} ok=${payload['ok']}';
      default:
        return '$kind $payload';
    }
  }

  /// Parse App WS envelope or HTTP snapshot map.
  factory TaskLogEvent.fromJson(Map<String, dynamic> json) {
    if (json['event'] is Map) {
      final ev = Map<String, dynamic>.from(json['event'] as Map);
      final payload = ev['payload'];
      return TaskLogEvent(
        seq: (json['seq'] as num?)?.toInt() ?? 0,
        kind: ev['kind'] as String? ?? '',
        payload: payload is Map
            ? Map<String, dynamic>.from(payload)
            : <String, dynamic>{},
        at: json['at'] as String?,
        taskId: json['taskId'] as String?,
      );
    }
    final payload = json['payload'];
    return TaskLogEvent(
      seq: (json['seq'] as num?)?.toInt() ?? 0,
      kind: json['kind'] as String? ?? '',
      payload: payload is Map
          ? Map<String, dynamic>.from(payload)
          : <String, dynamic>{},
      at: json['at'] as String?,
      taskId: json['taskId'] as String?,
    );
  }
}

/// One tool invocation derived from task events (deduped by callId/name).
class TaskToolStep {
  TaskToolStep({
    required this.name,
    this.detail,
    this.callId,
    this.done = false,
    this.ok = true,
  });

  final String name;
  final String? detail;
  final String? callId;
  bool done;
  bool ok;

  String get label {
    final d = detail?.trim();
    if (d == null || d.isEmpty) return name;
    return '$name · $d';
  }
}

/// Live activity snapshot for chat / logs UI.
class TaskActivity {
  const TaskActivity({
    this.phase = 'running',
    this.phaseDetail,
    this.tools = const [],
  });

  /// creating | running | thinking | tool | writing | cancelling | finished | error
  final String phase;
  final String? phaseDetail;
  final List<TaskToolStep> tools;

  String get phaseLabel {
    switch (phase) {
      case 'creating':
        return '准备中';
      case 'thinking':
        return '思考中';
      case 'tool':
        final d = phaseDetail?.trim();
        return (d == null || d.isEmpty) ? '调用工具' : '调用工具 · $d';
      case 'writing':
        return '生成回复';
      case 'cancelling':
        return '正在取消';
      case 'finished':
        return '完成';
      case 'error':
        return '出错';
      case 'cancelled':
        return '已取消';
      default:
        return '处理中';
    }
  }
}

/// Build activity from ordered events (callId-deduped tool steps).
TaskActivity activityFromEvents(Iterable<TaskLogEvent> events) {
  var phase = 'running';
  String? phaseDetail;
  final tools = <TaskToolStep>[];
  final byCall = <String, int>{};

  void upsertStarted(String name, String? detail, String? callId) {
    if (callId != null && callId.isNotEmpty) {
      final idx = byCall[callId];
      if (idx != null) {
        final prev = tools[idx];
        tools[idx] = TaskToolStep(
          name: name,
          detail: detail ?? prev.detail,
          callId: callId,
          done: prev.done,
          ok: prev.ok,
        );
        return;
      }
      byCall[callId] = tools.length;
    }
    tools.add(TaskToolStep(name: name, detail: detail, callId: callId));
  }

  void markFinished(String name, String? callId, bool ok) {
    if (callId != null && callId.isNotEmpty) {
      final idx = byCall[callId];
      if (idx != null) {
        tools[idx].done = true;
        tools[idx].ok = ok;
        return;
      }
    }
    for (var i = tools.length - 1; i >= 0; i--) {
      if (!tools[i].done && tools[i].name == name) {
        tools[i].done = true;
        tools[i].ok = ok;
        return;
      }
    }
    tools.add(TaskToolStep(name: name, callId: callId, done: true, ok: ok));
    if (callId != null && callId.isNotEmpty) {
      byCall[callId] = tools.length - 1;
    }
  }

  for (final e in events) {
    switch (e.kind) {
      case 'status':
        final s = e.payload['status']?.toString() ?? '';
        if (s.isEmpty) break;
        phase = s;
        final name = e.payload['name']?.toString();
        final preview = e.payload['preview']?.toString();
        if (s == 'tool' && name != null && name.isNotEmpty) {
          phaseDetail = name;
        } else if (s == 'thinking' && preview != null && preview.isNotEmpty) {
          phaseDetail = preview;
        } else if (s != 'tool') {
          phaseDetail = null;
        }
        break;
      case 'tool.started':
        final name = e.payload['name']?.toString() ?? 'tool';
        final summary = e.payload['summary']?.toString();
        final callId = e.payload['callId']?.toString();
        upsertStarted(name, summary, callId);
        phase = 'tool';
        phaseDetail = summary == null || summary.isEmpty ? name : '$name · $summary';
        break;
      case 'tool.finished':
        final name = e.payload['name']?.toString() ?? 'tool';
        final callId = e.payload['callId']?.toString();
        final ok = e.payload['ok'] != false;
        markFinished(name, callId, ok);
        if (phase == 'tool') {
          phase = 'running';
          phaseDetail = null;
        }
        break;
      case 'assistant.delta':
        if (phase != 'tool' && phase != 'cancelling') {
          phase = 'writing';
          phaseDetail = null;
        }
        break;
      case 'error':
        phase = 'error';
        phaseDetail = e.payload['message']?.toString();
        break;
      case 'done':
        final st = e.payload['status']?.toString() ?? '';
        if (st == 'cancelled') {
          phase = 'cancelled';
        } else if (st == 'error') {
          phase = 'error';
        } else {
          phase = 'finished';
        }
        phaseDetail = null;
        break;
    }
  }

  return TaskActivity(phase: phase, phaseDetail: phaseDetail, tools: tools);
}

/// Dedup by seq; builds assistant transcript.
class TaskLogBuffer extends ChangeNotifier {
  final List<TaskLogEvent> events = [];
  final StringBuffer assistant = StringBuffer();
  int lastSeq = 0;

  TaskActivity get activity => activityFromEvents(events);

  void ingest(TaskLogEvent e) {
    if (e.seq > 0 && e.seq <= lastSeq) return;
    if (e.seq > lastSeq) lastSeq = e.seq;
    events.add(e);
    if (e.kind == 'assistant.delta') {
      final t = e.payload['text'] as String?;
      if (t != null && t.isNotEmpty) assistant.write(t);
    }
    notifyListeners();
  }

  void ingestAll(Iterable<TaskLogEvent> list) {
    for (final e in list) {
      ingest(e);
    }
  }

  void clear() {
    events.clear();
    assistant.clear();
    lastSeq = 0;
    notifyListeners();
  }
}
