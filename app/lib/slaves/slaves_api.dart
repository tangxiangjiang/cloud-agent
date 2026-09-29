import 'dart:convert';

import 'package:http/http.dart' as http;

import '../auth/session.dart';
import 'models.dart';

class SlavesApiException implements Exception {
  SlavesApiException(this.message, {this.statusCode});
  final String message;
  final int? statusCode;

  @override
  String toString() => message;
}

typedef HttpGet = Future<http.Response> Function(
  Uri url, {
  Map<String, String>? headers,
});

/// Authenticated slaves list client (Bearer).
class SlavesApi {
  SlavesApi({
    required this.session,
    HttpGet? get,
  }) : _get = get ?? http.get;

  final Session session;
  final HttpGet _get;

  Map<String, String> get _headers => {
        'Authorization': 'Bearer ${session.token}',
        'Accept': 'application/json',
      };

  Future<List<SlaveInfo>> listSlaves() async {
    final uri = Uri.parse('${session.gatewayBaseUrl}/v1/slaves');
    final res = await _get(uri, headers: _headers);
    if (res.statusCode == 401) {
      throw SlavesApiException('Unauthorized — pair again', statusCode: 401);
    }
    if (res.statusCode < 200 || res.statusCode >= 300) {
      throw SlavesApiException(
        'List slaves failed (${res.statusCode})',
        statusCode: res.statusCode,
      );
    }
    late final Map<String, dynamic> body;
    try {
      final v = jsonDecode(res.body);
      if (v is Map<String, dynamic>) {
        body = v;
      } else if (v is Map) {
        body = Map<String, dynamic>.from(v);
      } else {
        throw FormatException('not a map');
      }
    } catch (_) {
      throw SlavesApiException('Invalid JSON response');
    }
    final list = body['slaves'];
    if (list is! List) return const [];
    return list
        .whereType<Map>()
        .map((e) => SlaveInfo.fromJson(Map<String, dynamic>.from(e)))
        .toList(growable: false);
  }
}
