/// Models for Gateway chat API (M09).
class ChatSession {
  const ChatSession({
    required this.id,
    required this.slaveId,
    required this.repoId,
    this.mode = 'agent',
    this.model = 'auto',
    this.status = 'idle',
    this.messages = const [],
    this.createdAt,
    this.updatedAt,
  });

  final String id;
  final String slaveId;
  final String repoId;
  final String mode;
  final String model;
  final String status;
  final List<ChatMessageSummary> messages;
  final String? createdAt;
  final String? updatedAt;

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
      messages: messages,
      createdAt: json['createdAt'] as String?,
      updatedAt: json['updatedAt'] as String?,
    );
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
    this.at,
  });

  final String id;
  final String role;
  final String content;
  final String? taskId;
  final String? mode;
  final String? model;
  final String? at;

  factory ChatMessageSummary.fromJson(Map<String, dynamic> json) {
    return ChatMessageSummary(
      id: json['id'] as String? ?? '',
      role: json['role'] as String? ?? '',
      content: json['content'] as String? ?? '',
      taskId: json['taskId'] as String?,
      mode: json['mode'] as String?,
      model: json['model'] as String?,
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
