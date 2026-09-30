import 'dart:convert';

import 'package:http/http.dart' as http;

import '../auth/session.dart';
import 'models.dart';

class MastersApiException implements Exception {
  MastersApiException(this.message, {this.statusCode, this.code});
  final String message;
  final int? statusCode;
  final String? code;

  @override
  String toString() => message;
}

typedef HttpGet = Future<http.Response> Function(
  Uri url, {
  Map<String, String>? headers,
});

typedef HttpSend = Future<http.Response> Function(
  String method,
  Uri url, {
  Map<String, String>? headers,
  Object? body,
});

/// Fleet control client: GET/POST /v1/masters* (sync wait on Gateway).
class MastersApi {
  MastersApi({
    required this.session,
    HttpGet? get,
    HttpSend? send,
  })  : _get = get ?? http.get,
        _send = send ?? _defaultSend;

  final Session session;
  final HttpGet _get;
  final HttpSend _send;

  static Future<http.Response> _defaultSend(
    String method,
    Uri url, {
    Map<String, String>? headers,
    Object? body,
  }) {
    switch (method) {
      case 'POST':
        return http.post(url, headers: headers, body: body);
      case 'PUT':
        return http.put(url, headers: headers, body: body);
      case 'PATCH':
        return http.patch(url, headers: headers, body: body);
      case 'DELETE':
        return http.delete(url, headers: headers);
      default:
        throw ArgumentError('unsupported $method');
    }
  }

  Map<String, String> get _headers => {
        'Authorization': 'Bearer ${session.token}',
        'Accept': 'application/json',
        'Content-Type': 'application/json; charset=utf-8',
      };

  Uri _uri(String path) => Uri.parse('${session.gatewayBaseUrl}$path');

  Future<List<MasterInfo>> listMasters() async {
    final res = await _get(_uri('/v1/masters'), headers: _headers);
    _throwIfBad(res, 'List masters');
    final body = _decodeMap(res.body);
    final list = body['masters'];
    if (list is! List) return const [];
    return list
        .whereType<Map>()
        .map((e) => MasterInfo.fromJson(Map<String, dynamic>.from(e)))
        .toList(growable: false);
  }

  Future<MasterInfo> getMaster(String masterId) async {
    final res = await _get(
      _uri('/v1/masters/${Uri.encodeComponent(masterId)}'),
      headers: _headers,
    );
    _throwIfBad(res, 'Get master');
    return MasterInfo.fromJson(_decodeMap(res.body));
  }

  Future<MasterInfo> startSlave(String masterId, String slaveId) =>
      _control(masterId, slaveId, 'start');

  Future<MasterInfo> stopSlave(String masterId, String slaveId) =>
      _control(masterId, slaveId, 'stop');

  Future<MasterInfo> restartSlave(String masterId, String slaveId) =>
      _control(masterId, slaveId, 'restart');

  Future<MasterInfo> _control(
    String masterId,
    String slaveId,
    String action,
  ) async {
    final res = await _send(
      'POST',
      _uri(
        '/v1/masters/${Uri.encodeComponent(masterId)}'
        '/slaves/${Uri.encodeComponent(slaveId)}/$action',
      ),
      headers: _headers,
    );
    _throwIfBad(res, action);
    final body = _decodeMap(res.body);
    final master = body['master'];
    if (master is Map) {
      return MasterInfo.fromJson(Map<String, dynamic>.from(master));
    }
    return getMaster(masterId);
  }

  Future<MasterInfo> upsertSlave(
    String masterId,
    Map<String, dynamic> slave, {
    bool create = false,
  }) async {
    final id = slave['id'] as String? ?? '';
    final Uri uri;
    final String method;
    if (create) {
      uri = _uri('/v1/masters/${Uri.encodeComponent(masterId)}/slaves');
      method = 'POST';
    } else {
      uri = _uri(
        '/v1/masters/${Uri.encodeComponent(masterId)}'
        '/slaves/${Uri.encodeComponent(id)}',
      );
      method = 'PUT';
    }
    final res = await _send(
      method,
      uri,
      headers: _headers,
      body: jsonEncode(slave),
    );
    _throwIfBad(res, create ? 'Create slave' : 'Update slave');
    final body = _decodeMap(res.body);
    final master = body['master'];
    if (master is Map) {
      return MasterInfo.fromJson(Map<String, dynamic>.from(master));
    }
    return getMaster(masterId);
  }

  Future<MasterInfo> deleteSlave(String masterId, String slaveId) async {
    final res = await _send(
      'DELETE',
      _uri(
        '/v1/masters/${Uri.encodeComponent(masterId)}'
        '/slaves/${Uri.encodeComponent(slaveId)}',
      ),
      headers: _headers,
    );
    _throwIfBad(res, 'Delete slave');
    final body = _decodeMap(res.body);
    final master = body['master'];
    if (master is Map) {
      return MasterInfo.fromJson(Map<String, dynamic>.from(master));
    }
    return getMaster(masterId);
  }

  void _throwIfBad(http.Response res, String action) {
    if (res.statusCode == 401) {
      throw MastersApiException('Unauthorized — pair again', statusCode: 401);
    }
    if (res.statusCode >= 200 && res.statusCode < 300) return;
    final parsed = _tryDecode(res.body);
    final code = parsed?['code']?.toString();
    final err = parsed?['error']?.toString();
    final msg = _friendlyError(res.statusCode, code, err) ??
        '$action failed (${res.statusCode})';
    throw MastersApiException(msg, statusCode: res.statusCode, code: code);
  }

  static String? _friendlyError(int status, String? code, String? err) {
    switch (status) {
      case 409:
        return err ?? 'Master 离线 (409)';
      case 503:
        return err ?? '无法送达 Master (503)';
      case 504:
        return err ?? 'Master 控制超时 (504)';
    }
    switch (code) {
      case 'slave_disabled':
        return 'Slave 已禁用，无法 Start';
      case 'slave_limit':
        return '已达同时运行上限';
      case 'start_busy':
        return '启动并发已满，请稍后';
      case 'cwd_denied':
        return 'cwd 不在 allowedRoots 或不合法';
      case 'repo_conflict':
        return 'repoId 或 cwd 与已有 Slave 冲突';
      case 'id_immutable':
        return '不能修改 slaveId';
      case 'not_stopped':
        return '请先 Stop 再改配置';
    }
    return err;
  }

  Map<String, dynamic> _decodeMap(String body) {
    try {
      final v = jsonDecode(body);
      if (v is Map<String, dynamic>) return v;
      if (v is Map) return Map<String, dynamic>.from(v);
    } catch (_) {}
    throw MastersApiException('Invalid JSON response');
  }

  Map<String, dynamic>? _tryDecode(String body) {
    try {
      final v = jsonDecode(body);
      if (v is Map<String, dynamic>) return v;
      if (v is Map) return Map<String, dynamic>.from(v);
    } catch (_) {}
    return null;
  }
}
