import 'dart:convert';

import 'package:http/http.dart' as http;

import '../util/redact.dart';
import 'session.dart';

class PairException implements Exception {
  PairException(this.message, {this.statusCode});
  final String message;
  final int? statusCode;

  @override
  String toString() => message;
}

/// Normalize user-entered Gateway base URL.
/// Keeps path prefix (e.g. `/gateway` behind nginx); strips trailing slashes.
String normalizeGatewayBaseUrl(String raw) {
  var s = raw.trim();
  if (s.isEmpty) {
    throw PairException('Gateway URL is required');
  }
  if (!s.contains('://')) {
    s = 'http://$s';
  }
  final uri = Uri.tryParse(s);
  if (uri == null || !uri.hasScheme || uri.host.isEmpty) {
    throw PairException('Invalid Gateway URL');
  }
  if (uri.scheme != 'http' && uri.scheme != 'https') {
    throw PairException('Gateway URL must be http or https');
  }
  var path = uri.path;
  while (path.endsWith('/')) {
    path = path.substring(0, path.length - 1);
  }
  if (path == '/') {
    path = '';
  }
  final port = uri.hasPort ? ':${uri.port}' : '';
  return '${uri.scheme}://${uri.host}$port$path';
}

typedef HttpPost = Future<http.Response> Function(
  Uri url, {
  Map<String, String>? headers,
  Object? body,
});

/// Thin HTTP client for pairing. Does not hold CURSOR_API_KEY.
class GatewayApi {
  GatewayApi({HttpPost? post}) : _post = post ?? http.post;

  final HttpPost _post;

  /// `POST {base}/v1/auth/pair` → [Session].
  Future<Session> pair({
    required String gatewayBaseUrl,
    required String pairCode,
  }) async {
    final base = normalizeGatewayBaseUrl(gatewayBaseUrl);
    final code = pairCode.trim();
    if (code.isEmpty) {
      throw PairException('Pair code is required');
    }

    final uri = Uri.parse('$base/v1/auth/pair');
    final http.Response res;
    try {
      res = await _post(
        uri,
        headers: {'Content-Type': 'application/json; charset=utf-8'},
        body: jsonEncode({
          'gatewayUrl': base,
          'pairCode': code,
        }),
      );
    } catch (e) {
      throw PairException('Network error: $e');
    }

    if (res.statusCode == 401) {
      throw PairException('Invalid pair code', statusCode: 401);
    }
    if (res.statusCode < 200 || res.statusCode >= 300) {
      throw PairException(
        'Pair failed (${res.statusCode})',
        statusCode: res.statusCode,
      );
    }

    final Map<String, dynamic> body;
    try {
      body = jsonDecode(res.body) as Map<String, dynamic>;
    } catch (_) {
      throw PairException('Invalid pair response');
    }
    final token = (body['token'] as String?)?.trim() ?? '';
    if (token.isEmpty) {
      throw PairException('Pair response missing token');
    }
    // Intentionally do not log [token]; callers may log redactSecret(token).
    assert(() {
      // ignore: avoid_print
      print('pair ok token=${redactSecret(token)} base=$base');
      return true;
    }());

    DateTime? expiresAt;
    final exp = body['expiresAt'] as String?;
    if (exp != null && exp.isNotEmpty) {
      expiresAt = DateTime.tryParse(exp);
    }

    return Session(
      gatewayBaseUrl: base,
      token: token,
      expiresAt: expiresAt,
    );
  }
}
