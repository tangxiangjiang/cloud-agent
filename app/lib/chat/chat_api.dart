import 'dart:convert';

import 'package:http/http.dart' as http;

import '../auth/session.dart';
import 'models.dart';

class ChatApiException implements Exception {
  ChatApiException(this.message, {this.statusCode});
  final String message;
  final int? statusCode;

  @override
  String toString() => message;
}

typedef HttpGet = Future<http.Response> Function(
  Uri url, {
  Map<String, String>? headers,
});

typedef HttpPost = Future<http.Response> Function(
  Uri url, {
  Map<String, String>? headers,
  Object? body,
});

typedef HttpDelete = Future<http.Response> Function(
  Uri url, {
  Map<String, String>? headers,
});

typedef HttpPut = Future<http.Response> Function(
  Uri url, {
  Map<String, String>? headers,
  Object? body,
});

typedef HttpPatch = Future<http.Response> Function(
  Uri url, {
  Map<String, String>? headers,
  Object? body,
});

/// Bearer client for project chat (no API key / cwd).
class ChatApi {
  ChatApi({
    required this.session,
    HttpGet? get,
    HttpPost? post,
    HttpDelete? delete,
    HttpPut? put,
    HttpPatch? patch,
  })  : _get = get ?? http.get,
        _post = post ?? http.post,
        _delete = delete ?? http.delete,
        _put = put ?? http.put,
        _patch = patch ?? http.patch;

  final Session session;
  final HttpGet _get;
  final HttpPost _post;
  final HttpDelete _delete;
  final HttpPut _put;
  final HttpPatch _patch;

  Map<String, String> get _headers => {
        'Authorization': 'Bearer ${session.token}',
        'Accept': 'application/json',
      };

  Uri _uri(String path, [Map<String, String>? query]) {
    return Uri.parse('${session.gatewayBaseUrl}$path')
        .replace(queryParameters: query);
  }

  /// Allowed chat models for the App picker (editable Gateway selection).
  Future<ModelCatalog> listModels() async {
    final res = await _get(
      _uri('/v1/models'),
      headers: _headers,
    );
    _throwIfBad(res, 'List models');
    return ModelCatalog.fromJson(_decodeMap(res.body));
  }

  /// Selected + Slave-reported available models.
  Future<ModelManageView> manageModels() async {
    final res = await _get(
      _uri('/v1/models/manage'),
      headers: _headers,
    );
    _throwIfBad(res, 'Manage models');
    return ModelManageView.fromJson(_decodeMap(res.body));
  }

  /// Add a model id to the App picker list.
  Future<ModelManageView> addModel({
    required String id,
    String? label,
  }) async {
    final res = await _post(
      _uri('/v1/models'),
      headers: {
        ..._headers,
        'Content-Type': 'application/json; charset=utf-8',
      },
      body: jsonEncode({
        'id': id,
        if (label != null && label.isNotEmpty) 'label': label,
      }),
    );
    _throwIfBad(res, 'Add model');
    return ModelManageView.fromJson(_decodeMap(res.body));
  }

  /// Remove a model from the App picker (cannot remove auto).
  Future<ModelManageView> removeModel(String id) async {
    final res = await _delete(
      _uri('/v1/models/${Uri.encodeComponent(id)}'),
      headers: _headers,
    );
    _throwIfBad(res, 'Remove model');
    return ModelManageView.fromJson(_decodeMap(res.body));
  }

  /// Ask online Slave to refresh Cursor.models.list into available.
  Future<ModelManageView> refreshModels() async {
    final res = await _post(
      _uri('/v1/models/refresh'),
      headers: {
        ..._headers,
        'Content-Type': 'application/json; charset=utf-8',
      },
      body: '{}',
    );
    _throwIfBad(res, 'Refresh models');
    final map = _decodeMap(res.body);
    // Accepted payload mirrors manage fields; poll manage after short delay if empty.
    return ModelManageView.fromJson(map);
  }

  /// Set default model id (must already be in selected).
  Future<ModelManageView> setDefaultModel(String id) async {
    final res = await _put(
      _uri('/v1/models/default'),
      headers: {
        ..._headers,
        'Content-Type': 'application/json; charset=utf-8',
      },
      body: jsonEncode({'id': id}),
    );
    _throwIfBad(res, 'Set default model');
    return ModelManageView.fromJson(_decodeMap(res.body));
  }

  Future<ChatSession> createChat({
    required String slaveId,
    required String repoId,
    String mode = 'agent',
    String model = 'auto',
  }) async {
    final res = await _post(
      _uri('/v1/chats'),
      headers: {
        ..._headers,
        'Content-Type': 'application/json; charset=utf-8',
      },
      body: jsonEncode({
        'slaveId': slaveId,
        'repoId': repoId,
        'mode': mode,
        'model': model,
      }),
    );
    _throwIfBad(res, 'Create chat');
    return ChatSession.fromJson(_decodeMap(res.body));
  }

  Future<ChatSession> getChat(String chatId) async {
    final res = await _get(
      _uri('/v1/chats/${Uri.encodeComponent(chatId)}'),
      headers: _headers,
    );
    _throwIfBad(res, 'Get chat');
    return ChatSession.fromJson(_decodeMap(res.body));
  }

  /// Rename session (manual override of AI / provisional title).
  Future<ChatSession> renameChat(String chatId, String title) async {
    final res = await _patch(
      _uri('/v1/chats/${Uri.encodeComponent(chatId)}'),
      headers: {
        ..._headers,
        'Content-Type': 'application/json; charset=utf-8',
      },
      body: jsonEncode({'title': title}),
    );
    _throwIfBad(res, 'Rename chat');
    return ChatSession.fromJson(_decodeMap(res.body));
  }

  /// List sessions for a project (messages omitted; use [getChat] to continue).
  Future<List<ChatSession>> listChats({
    required String slaveId,
    required String repoId,
  }) async {
    final res = await _get(
      _uri('/v1/chats', {
        'slaveId': slaveId,
        'repoId': repoId,
      }),
      headers: _headers,
    );
    _throwIfBad(res, 'List chats');
    final map = _decodeMap(res.body);
    final raw = map['chats'];
    final out = <ChatSession>[];
    if (raw is List) {
      for (final c in raw) {
        if (c is Map) {
          out.add(ChatSession.fromJson(Map<String, dynamic>.from(c)));
        }
      }
    }
    return out;
  }

  /// Persist truncated assistant summary after a turn (no tool payloads).
  Future<ChatSession> recordAssistant({
    required String chatId,
    required String content,
    String? taskId,
  }) async {
    final res = await _post(
      _uri('/v1/chats/${Uri.encodeComponent(chatId)}/assistant'),
      headers: {
        ..._headers,
        'Content-Type': 'application/json; charset=utf-8',
      },
      body: jsonEncode({
        'content': content,
        if (taskId != null && taskId.isNotEmpty) 'taskId': taskId,
      }),
    );
    _throwIfBad(res, 'Record assistant');
    return ChatSession.fromJson(_decodeMap(res.body));
  }

  Future<ChatSendResult> sendMessage({
    required String chatId,
    required String text,
    String? mode,
    String? model,
  }) async {
    final res = await _post(
      _uri('/v1/chats/${Uri.encodeComponent(chatId)}/messages'),
      headers: {
        ..._headers,
        'Content-Type': 'application/json; charset=utf-8',
      },
      body: jsonEncode({
        'text': text,
        if (mode != null && mode.isNotEmpty) 'mode': mode,
        if (model != null && model.isNotEmpty) 'model': model,
      }),
    );
    _throwIfBad(res, 'Send message');
    return ChatSendResult.fromJson(_decodeMap(res.body));
  }

  /// Stop latest turn via Gateway chat stop (delegates to task cancel).
  Future<void> stopChat(String chatId) async {
    final res = await _post(
      _uri('/v1/chats/${Uri.encodeComponent(chatId)}/stop'),
      headers: {
        ..._headers,
        'Content-Type': 'application/json; charset=utf-8',
      },
      body: '{}',
    );
    _throwIfBad(res, 'Stop chat');
  }

  Future<void> cancelTask(String taskId) async {
    final res = await _post(
      _uri('/v1/tasks/${Uri.encodeComponent(taskId)}/cancel'),
      headers: {
        ..._headers,
        'Content-Type': 'application/json; charset=utf-8',
      },
      body: '{}',
    );
    _throwIfBad(res, 'Cancel task');
  }

  Map<String, dynamic> _decodeMap(String raw) {
    try {
      final v = jsonDecode(raw);
      if (v is Map<String, dynamic>) return v;
      if (v is Map) return Map<String, dynamic>.from(v);
    } catch (_) {}
    throw ChatApiException('Invalid JSON response');
  }

  void _throwIfBad(http.Response res, String action) {
    if (res.statusCode >= 200 && res.statusCode < 300) return;
    String detail = '$action failed (${res.statusCode})';
    try {
      final m = jsonDecode(res.body);
      if (m is Map && m['error'] != null) {
        detail = '$action: ${m['error']}';
      }
    } catch (_) {}
    if (res.statusCode == 401) {
      throw ChatApiException('Unauthorized — pair again', statusCode: 401);
    }
    throw ChatApiException(detail, statusCode: res.statusCode);
  }
}
