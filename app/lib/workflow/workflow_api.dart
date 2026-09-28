import 'dart:convert';

import 'package:http/http.dart' as http;

import '../auth/session.dart';
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

/// Authenticated workflow HTTP client (Bearer). No CURSOR_API_KEY.
class WorkflowApi {
  WorkflowApi({
    required this.session,
    HttpGet? get,
    HttpPost? post,
  })  : _get = get ?? http.get,
        _post = post ?? http.post;

  final Session session;
  final HttpGet _get;
  final HttpPost _post;

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
    final body = _decodeMap(res.body);
    final wf = body['workflow'];
    if (wf is! Map) {
      throw WorkflowApiException('Start response missing workflow');
    }
    return StartWorkflowResult(
      workflow: WorkflowRun.fromJson(Map<String, dynamic>.from(wf)),
      delivered: body['delivered'] == true,
    );
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
