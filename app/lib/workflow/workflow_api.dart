import 'dart:convert';

import 'package:http/http.dart' as http;

import '../auth/session.dart';
import 'diff_models.dart';
import 'models.dart';

class WorkflowApiException implements Exception {
  WorkflowApiException(this.message, {this.statusCode});
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

typedef HttpPatch = Future<http.Response> Function(
  Uri url, {
  Map<String, String>? headers,
  Object? body,
});

/// Authenticated workflow HTTP client (Bearer). No CURSOR_API_KEY.
class WorkflowApi {
  WorkflowApi({
    required this.session,
    HttpGet? get,
    HttpPost? post,
    HttpPatch? patch,
  })  : _get = get ?? http.get,
        _post = post ?? http.post,
        _patch = patch ?? http.patch;

  final Session session;
  final HttpGet _get;
  final HttpPost _post;
  final HttpPatch _patch;

  Map<String, String> get _headers => {
        'Authorization': 'Bearer ${session.token}',
        'Accept': 'application/json',
      };

  Uri _uri(String path, [Map<String, String>? query]) {
    final base = session.gatewayBaseUrl;
    return Uri.parse('$base$path').replace(queryParameters: query);
  }

  Future<List<WorkflowRun>> listWorkflows({int limit = 50}) async {
    final res = await _get(
      _uri('/v1/workflows', {'limit': '$limit'}),
      headers: _headers,
    );
    _throwIfBad(res, 'List workflows');
    final body = _decodeMap(res.body);
    final list = body['workflows'];
    if (list is! List) return const [];
    return list
        .whereType<Map>()
        .map((e) => WorkflowRun.fromJson(Map<String, dynamic>.from(e)))
        .toList(growable: false);
  }

  Future<WorkflowRun> getWorkflow(String id) async {
    final res = await _get(
      _uri('/v1/workflows/${Uri.encodeComponent(id)}'),
      headers: _headers,
    );
    _throwIfBad(res, 'Get workflow');
    return WorkflowRun.fromJson(_decodeMap(res.body));
  }

  /// `POST /v1/workflows` — create DAG snapshot from milestone phases.
  Future<WorkflowRun> createWorkflow({
    required String bundleId,
    required String repoId,
    required List<Map<String, dynamic>> nodes,
    String? slaveId,
    String? progressDoc,
    String? bundleRef,
  }) async {
    final res = await _post(
      _uri('/v1/workflows'),
      headers: {
        ..._headers,
        'Content-Type': 'application/json; charset=utf-8',
      },
      body: jsonEncode({
        'bundleId': bundleId,
        'repoId': repoId,
        'nodes': nodes,
        if (slaveId != null && slaveId.isNotEmpty) 'slaveId': slaveId,
        if (progressDoc != null && progressDoc.isNotEmpty)
          'progressDoc': progressDoc,
        if (bundleRef != null && bundleRef.isNotEmpty) 'bundleRef': bundleRef,
      }),
    );
    _throwIfBad(res, 'Create workflow');
    return WorkflowRun.fromJson(_decodeMap(res.body));
  }

  Future<StartWorkflowResult> startWorkflow(String id) async {
    final res = await _post(
      _uri('/v1/workflows/${Uri.encodeComponent(id)}/start'),
      headers: {
        ..._headers,
        'Content-Type': 'application/json; charset=utf-8',
      },
      body: '{}',
    );
    _throwIfBad(res, 'Start workflow');
    return _parseDelivered(res.body, 'Start');
  }

  /// Resume ready nodes after approve (when autoStartNext is off).
  Future<StartWorkflowResult> continueWorkflow(String id) async {
    final res = await _post(
      _uri('/v1/workflows/${Uri.encodeComponent(id)}/continue'),
      headers: {
        ..._headers,
        'Content-Type': 'application/json; charset=utf-8',
      },
      body: '{}',
    );
    _throwIfBad(res, 'Continue workflow');
    return _parseDelivered(res.body, 'Continue');
  }

  /// Start a single ready node.
  Future<StartWorkflowResult> startNode(String workflowId, String nodeId) async {
    final res = await _post(
      _uri(
        '/v1/workflows/${Uri.encodeComponent(workflowId)}/nodes/${Uri.encodeComponent(nodeId)}/start',
      ),
      headers: {
        ..._headers,
        'Content-Type': 'application/json; charset=utf-8',
      },
      body: '{}',
    );
    _throwIfBad(res, 'Start node');
    return _parseDelivered(res.body, 'Start node');
  }

  /// PATCH model / policy (and optional status fields used by Slave).
  Future<WorkflowRun> patchNode(
    String workflowId,
    String nodeId, {
    String? model,
    NodePolicy? policy,
    String? status,
    String? taskId,
  }) async {
    final body = <String, dynamic>{};
    if (model != null) body['model'] = model;
    if (policy != null) body['policy'] = policy.toJson();
    if (status != null) body['status'] = status;
    if (taskId != null) body['taskId'] = taskId;
    if (body.isEmpty) {
      throw WorkflowApiException('no fields to patch');
    }
    final res = await _patch(
      _uri(
        '/v1/workflows/${Uri.encodeComponent(workflowId)}/nodes/${Uri.encodeComponent(nodeId)}',
      ),
      headers: {
        ..._headers,
        'Content-Type': 'application/json; charset=utf-8',
      },
      body: jsonEncode(body),
    );
    _throwIfBad(res, 'Patch node');
    return WorkflowRun.fromJson(_decodeMap(res.body));
  }

  StartWorkflowResult _parseDelivered(String raw, String action) {
    final body = _decodeMap(raw);
    final wf = body['workflow'];
    if (wf is! Map) {
      throw WorkflowApiException('$action response missing workflow');
    }
    return StartWorkflowResult(
      workflow: WorkflowRun.fromJson(Map<String, dynamic>.from(wf)),
      delivered: body['delivered'] == true,
    );
  }

  /// HTTP fallback: `GET /v1/tasks/{id}/events?afterSeq=`
  Future<({List<Map<String, dynamic>> events, int latestSeq})> getTaskEvents(
    String taskId, {
    int afterSeq = 0,
  }) async {
    final res = await _get(
      _uri(
        '/v1/tasks/${Uri.encodeComponent(taskId)}/events',
        {'afterSeq': '$afterSeq'},
      ),
      headers: _headers,
    );
    _throwIfBad(res, 'Get task events');
    final body = _decodeMap(res.body);
    final raw = body['events'];
    final list = <Map<String, dynamic>>[];
    if (raw is List) {
      for (final e in raw) {
        if (e is Map) list.add(Map<String, dynamic>.from(e));
      }
    }
    final latest = (body['latestSeq'] as num?)?.toInt() ?? 0;
    return (events: list, latestSeq: latest);
  }

  /// Read-only diff: `GET /v1/workflows/{id}/nodes/{nodeId}/diff`
  Future<NodeDiff> getNodeDiff(String workflowId, String nodeId) async {
    final res = await _get(
      _uri(
        '/v1/workflows/${Uri.encodeComponent(workflowId)}/nodes/${Uri.encodeComponent(nodeId)}/diff',
      ),
      headers: _headers,
    );
    _throwIfBad(res, 'Get node diff');
    return NodeDiff.fromJson(_decodeMap(res.body));
  }

  /// `POST .../revise` — App instruction only (no source write-back).
  Future<WorkflowRun> reviseNode(
    String workflowId,
    String nodeId, {
    required String instruction,
  }) async {
    final res = await _post(
      _uri(
        '/v1/workflows/${Uri.encodeComponent(workflowId)}/nodes/${Uri.encodeComponent(nodeId)}/revise',
      ),
      headers: {
        ..._headers,
        'Content-Type': 'application/json; charset=utf-8',
      },
      body: jsonEncode({'instruction': instruction}),
    );
    _throwIfBad(res, 'Revise');
    final body = _decodeMap(res.body);
    final wf = body['workflow'];
    if (wf is Map) {
      return WorkflowRun.fromJson(Map<String, dynamic>.from(wf));
    }
    return getWorkflow(workflowId);
  }

  /// `POST .../review` decision approve|reject. App never writes progress.md.
  Future<WorkflowRun> reviewNode(
    String workflowId,
    String nodeId, {
    required String decision,
    String? comment,
  }) async {
    final res = await _post(
      _uri(
        '/v1/workflows/${Uri.encodeComponent(workflowId)}/nodes/${Uri.encodeComponent(nodeId)}/review',
      ),
      headers: {
        ..._headers,
        'Content-Type': 'application/json; charset=utf-8',
      },
      body: jsonEncode({
        'decision': decision,
        if (comment != null && comment.trim().isNotEmpty) 'comment': comment.trim(),
      }),
    );
    _throwIfBad(res, 'Review');
    final body = _decodeMap(res.body);
    final wf = body['workflow'];
    if (wf is Map) {
      return WorkflowRun.fromJson(Map<String, dynamic>.from(wf));
    }
    return getWorkflow(workflowId);
  }

  Map<String, dynamic> _decodeMap(String raw) {
    try {
      final v = jsonDecode(raw);
      if (v is Map<String, dynamic>) return v;
      if (v is Map) return Map<String, dynamic>.from(v);
    } catch (_) {}
    throw WorkflowApiException('Invalid JSON response');
  }

  void _throwIfBad(http.Response res, String action) {
    if (res.statusCode >= 200 && res.statusCode < 300) return;
    String detail = action;
    try {
      final m = jsonDecode(res.body);
      if (m is Map && m['error'] != null) {
        detail = '$action: ${m['error']}';
      }
    } catch (_) {
      detail = '$action failed (${res.statusCode})';
    }
    if (res.statusCode == 401) {
      throw WorkflowApiException('Unauthorized — pair again', statusCode: 401);
    }
    throw WorkflowApiException(detail, statusCode: res.statusCode);
  }
}
