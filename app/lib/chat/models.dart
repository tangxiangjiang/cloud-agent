/// Models for Gateway chat API (M09).
class ChatSession {
  const ChatSession({
    required this.id,
    required this.slaveId,
    required this.repoId,
    this.mode = 'agent',
    this.model = 'auto',
    this.status = 'idle',
    this.title,
    this.messages = const [],
    this.createdAt,
    this.updatedAt,
    this.preview,
    this.messageCount = 0,
  });

  final String id;
  final String slaveId;
  final String repoId;
  final String mode;
  final String model;
  final String status;
  final String? title;
  final List<ChatMessageSummary> messages;
  final String? createdAt;
  final String? updatedAt;
  final String? preview;
  final int messageCount;

  /// Prefer AI/user title, then preview, then id.
  String get displayTitle {
    final t = title?.trim();
    if (t != null && t.isNotEmpty) return t;
    final p = preview?.trim();
    if (p != null && p.isNotEmpty) return p;
    return id;
  }

  factory ChatSession.fromJson(Map<String, dynamic> json) {
    final raw = json['messages'];
    final messages = <ChatMessageSummary>[];
    if (raw is List) {
      for (final m in raw) {
        if (m is Map) {
          messages.add(ChatMessageSummary.fromJson(Map<String, dynamic>.from(m)));
        }
      }
    }
    return ChatSession(
      id: json['id'] as String? ?? '',
      slaveId: json['slaveId'] as String? ?? '',
      repoId: json['repoId'] as String? ?? '',
      mode: json['mode'] as String? ?? 'agent',
      model: json['model'] as String? ?? 'auto',
      status: json['status'] as String? ?? 'idle',
      title: json['title'] as String?,
      messages: messages,
      createdAt: json['createdAt'] as String?,
      updatedAt: json['updatedAt'] as String?,
      preview: json['preview'] as String?,
      messageCount: json['messageCount'] as int? ?? messages.length,
    );
  }

  ChatSession copyWith({String? title}) {
    return ChatSession(
      id: id,
      slaveId: slaveId,
      repoId: repoId,
      mode: mode,
      model: model,
      status: status,
      title: title ?? this.title,
      messages: messages,
      createdAt: createdAt,
      updatedAt: updatedAt,
      preview: preview,
      messageCount: messageCount,
    );
  }
}

/// Cited workspace artifact on a chat message (plan phase for corrections).
class ChatRef {
  const ChatRef({
    required this.kind,
    required this.id,
    this.title,
    this.milestoneId,
    this.phaseRef,
  });

  final String kind;
  final String id;
  final String? title;
  final String? milestoneId;
  final String? phaseRef;

  String get label {
    final t = title?.trim();
    if (t != null && t.isNotEmpty) return '$id $t';
    return id;
  }

  factory ChatRef.fromJson(Map<String, dynamic> json) {
    return ChatRef(
      kind: json['kind'] as String? ?? '',
      id: json['id'] as String? ?? '',
      title: json['title'] as String?,
      milestoneId: json['milestoneId'] as String?,
      phaseRef: json['phaseRef'] as String?,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'kind': kind,
      'id': id,
      if (title != null && title!.isNotEmpty) 'title': title,
      if (milestoneId != null && milestoneId!.isNotEmpty)
        'milestoneId': milestoneId,
      if (phaseRef != null && phaseRef!.isNotEmpty) 'phaseRef': phaseRef,
    };
  }
}

class ChatMessageSummary {
  const ChatMessageSummary({
    required this.id,
    required this.role,
    required this.content,
    this.taskId,
    this.mode,
    this.model,
    this.refs = const [],
    this.at,
  });

  final String id;
  final String role;
  final String content;
  final String? taskId;
  final String? mode;
  final String? model;
  final List<ChatRef> refs;
  final String? at;

  factory ChatMessageSummary.fromJson(Map<String, dynamic> json) {
    final rawRefs = json['refs'];
    final refs = <ChatRef>[];
    if (rawRefs is List) {
      for (final r in rawRefs) {
        if (r is Map) {
          refs.add(ChatRef.fromJson(Map<String, dynamic>.from(r)));
        }
      }
    }
    return ChatMessageSummary(
      id: json['id'] as String? ?? '',
      role: json['role'] as String? ?? '',
      content: json['content'] as String? ?? '',
      taskId: json['taskId'] as String?,
      mode: json['mode'] as String?,
      model: json['model'] as String?,
      refs: refs,
      at: json['at'] as String?,
    );
  }
}

class ChatSendResult {
  const ChatSendResult({
    required this.taskId,
    required this.chatId,
    this.mode,
    this.model,
  });

  final String taskId;
  final String chatId;
  final String? mode;
  final String? model;

  factory ChatSendResult.fromJson(Map<String, dynamic> json) {
    return ChatSendResult(
      taskId: json['taskId'] as String? ?? '',
      chatId: json['chatId'] as String? ?? '',
      mode: json['mode'] as String?,
      model: json['model'] as String?,
    );
  }
}

/// Entry from GET /v1/models.
class ModelEntry {
  const ModelEntry({required this.id, required this.label});

  final String id;
  final String label;

  factory ModelEntry.fromJson(Map<String, dynamic> json) {
    final id = json['id'] as String? ?? '';
    return ModelEntry(
      id: id,
      label: json['label'] as String? ?? id,
    );
  }
}

/// Catalog from GET /v1/models (M09-P03).
class ModelCatalog {
  const ModelCatalog({
    this.defaultId = 'auto',
    this.models = const [
      ModelEntry(id: 'auto', label: 'Auto'),
    ],
  });

  final String defaultId;
  final List<ModelEntry> models;

  factory ModelCatalog.fromJson(Map<String, dynamic> json) {
    final raw = json['models'];
    final models = <ModelEntry>[];
    if (raw is List) {
      for (final m in raw) {
        if (m is Map) {
          models.add(ModelEntry.fromJson(Map<String, dynamic>.from(m)));
        }
      }
    }
    if (models.isEmpty) {
      models.add(const ModelEntry(id: 'auto', label: 'Auto'));
    }
    return ModelCatalog(
      defaultId: json['default'] as String? ?? 'auto',
      models: models,
    );
  }

  static const fallback = ModelCatalog();
}

/// GET /v1/models/manage — selected picker + Slave-reported available.
class ModelManageView {
  const ModelManageView({
    this.defaultId = 'auto',
    this.selected = const [],
    this.available = const [],
  });

  final String defaultId;
  final List<ModelEntry> selected;
  final List<ModelEntry> available;

  factory ModelManageView.fromJson(Map<String, dynamic> json) {
    List<ModelEntry> parseList(dynamic raw) {
      final out = <ModelEntry>[];
      if (raw is List) {
        for (final m in raw) {
          if (m is Map) {
            out.add(ModelEntry.fromJson(Map<String, dynamic>.from(m)));
          }
        }
      }
      return out;
    }

    return ModelManageView(
      defaultId: json['default'] as String? ?? 'auto',
      selected: parseList(json['selected']),
      available: parseList(json['available']),
    );
  }
}
