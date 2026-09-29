import 'dart:convert';

import 'package:http/http.dart' as http;

import '../auth/session.dart';
import 'models.dart';

class ProjectSyncApiException implements Exception {
  ProjectSyncApiException(this.message, {this.statusCode});
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

/// Bearer client for project sync trigger / snapshot (no API key).
class ProjectSyncApi {
  ProjectSyncApi({
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

  Uri _syncUri(String slaveId, String repoId, {String suffix = ''}) {
    final path =
        '/v1/slaves/${Uri.encodeComponent(slaveId)}/projects/${Uri.encodeComponent(repoId)}/sync$suffix';
    return Uri.parse('${session.gatewayBaseUrl}$path');
  }

  /// `POST .../sync` → 202 accepted (Slave collects asynchronously).
  Future<SyncTriggerResult> triggerSync({
    required String slaveId,
    required String repoId,
    String? requestId,
  }) async {
    final res = await _post(
      _syncUri(slaveId, repoId),
      headers: {
        ..._headers,
        'Content-Type': 'application/json; charset=utf-8',
      },
      body: jsonEncode({
        if (requestId != null && requestId.isNotEmpty) 'requestId': requestId,
      }),
    );
    if (res.statusCode == 409) {
      throw ProjectSyncApiException(
        _errorMessage(res, 'Slave offline'),
        statusCode: 409,
      );
    }
    if (res.statusCode == 503) {
      throw ProjectSyncApiException(
        _errorMessage(res, 'Slave connection unavailable'),
        statusCode: 503,
      );
    }
    _throwIfBad(res, 'Trigger sync');
    return SyncTriggerResult.fromJson(_decodeMap(res.body));
  }

  /// `GET .../sync` — latest snapshot + warnings/report.
  Future<ProjectSyncSnapshot> getSync({
    required String slaveId,
    required String repoId,
  }) async {
    final res = await _get(
      _syncUri(slaveId, repoId),
      headers: _headers,
    );
    if (res.statusCode == 404) {
      throw ProjectSyncApiException('No sync snapshot yet', statusCode: 404);
    }
    _throwIfBad(res, 'Get sync');
    return ProjectSyncSnapshot.fromJson(_decodeMap(res.body));
  }

  /// Trigger then poll until `syncedAt` advances (or first snapshot appears).
  Future<ProjectSyncSnapshot> syncAndWait({
    required String slaveId,
    required String repoId,
    Duration timeout = const Duration(seconds: 25),
    Duration interval = const Duration(milliseconds: 800),
    DateTime Function()? clock,
  }) async {
    final now = clock ?? DateTime.now;
    String? before;
    try {
      final prev = await getSync(slaveId: slaveId, repoId: repoId);
      before = prev.syncedAt;
    } on ProjectSyncApiException catch (e) {
      if (e.statusCode != 404) rethrow;
    }

    await triggerSync(slaveId: slaveId, repoId: repoId);

    final deadline = now().add(timeout);
    ProjectSyncApiException? lastErr;
    while (now().isBefore(deadline)) {
      await Future<void>.delayed(interval);
      try {
        final snap = await getSync(slaveId: slaveId, repoId: repoId);
        if (before == null || snap.syncedAt != before) {
          return snap;
        }
      } on ProjectSyncApiException catch (e) {
        lastErr = e;
        if (e.statusCode != 404) rethrow;
      }
    }
    throw ProjectSyncApiException(
      lastErr?.message ?? 'Sync timed out waiting for Slave report',
      statusCode: lastErr?.statusCode,
    );
  }

  Map<String, dynamic> _decodeMap(String raw) {
    try {
      final v = jsonDecode(raw);
      if (v is Map<String, dynamic>) return v;
      if (v is Map) return Map<String, dynamic>.from(v);
    } catch (_) {}
    throw ProjectSyncApiException('Invalid JSON response');
  }

  String _errorMessage(http.Response res, String fallback) {
    try {
      final m = jsonDecode(res.body);
      if (m is Map && m['error'] != null) {
        return m['error'].toString();
      }
    } catch (_) {}
    return fallback;
  }

  void _throwIfBad(http.Response res, String action) {
    if (res.statusCode >= 200 && res.statusCode < 300) return;
    if (res.statusCode == 401) {
      throw ProjectSyncApiException('Unauthorized — pair again', statusCode: 401);
    }
    throw ProjectSyncApiException(
      _errorMessage(res, '$action failed (${res.statusCode})'),
      statusCode: res.statusCode,
    );
  }
}
