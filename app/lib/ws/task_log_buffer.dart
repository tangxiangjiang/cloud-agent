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

/// Dedup by seq; builds assistant transcript.
class TaskLogBuffer extends ChangeNotifier {
  final List<TaskLogEvent> events = [];
  final StringBuffer assistant = StringBuffer();
  int lastSeq = 0;

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
