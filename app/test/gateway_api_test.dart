import 'dart:convert';

import 'package:cloud_agent_app/auth/gateway_api.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;

void main() {
  test('normalizeGatewayBaseUrl strips path and trailing slash', () {
    expect(
      normalizeGatewayBaseUrl('http://127.0.0.1:8080/'),
      'http://127.0.0.1:8080',
    );
    expect(
      normalizeGatewayBaseUrl('10.0.2.2:8080'),
      'http://10.0.2.2:8080',
    );
  });

  test('pair stores token from response without throwing', () async {
    final api = GatewayApi(
      post: (uri, {headers, body}) async {
        expect(uri.path, '/v1/auth/pair');
        final map = jsonDecode(body as String) as Map<String, dynamic>;
        expect(map['pairCode'], 'ABCD-EFGH');
        return http.Response(
          jsonEncode({
            'token': 'tok_super_secret_value_abcdef',
            'expiresAt': '2026-10-28T00:00:00.000Z',
          }),
          200,
          headers: {'content-type': 'application/json'},
        );
      },
    );

    final session = await api.pair(
      gatewayBaseUrl: 'http://127.0.0.1:8080',
      pairCode: 'ABCD-EFGH',
    );
    expect(session.token, 'tok_super_secret_value_abcdef');
    expect(session.gatewayBaseUrl, 'http://127.0.0.1:8080');
  });

  test('pair maps 401 to PairException', () async {
    final api = GatewayApi(
      post: (uri, {headers, body}) async =>
          http.Response('{"error":"invalid pair code"}', 401),
    );
    expect(
      () => api.pair(gatewayBaseUrl: 'http://x:8080', pairCode: 'BAD'),
      throwsA(isA<PairException>()),
    );
  });
}
