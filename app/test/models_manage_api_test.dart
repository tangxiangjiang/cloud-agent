import 'dart:convert';

import 'package:cloud_agent_app/auth/session.dart';
import 'package:cloud_agent_app/chat/chat_api.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;

void main() {
  final session = Session(
    gatewayBaseUrl: 'http://127.0.0.1:8080',
    token: 'tok',
  );

  test('manageModels parses selected and available', () async {
    final api = ChatApi(
      session: session,
      get: (uri, {headers}) async {
        expect(uri.path, '/v1/models/manage');
        return http.Response(
          jsonEncode({
            'default': 'auto',
            'selected': [
              {'id': 'auto', 'label': 'Auto'},
              {'id': 'composer-2.5', 'label': 'Composer'},
            ],
            'available': [
              {'id': 'default', 'label': 'Default'},
            ],
          }),
          200,
        );
      },
    );
    final v = await api.manageModels();
    expect(v.selected.length, 2);
    expect(v.available.single.id, 'default');
  });

  test('addModel posts id', () async {
    final api = ChatApi(
      session: session,
      post: (uri, {headers, body}) async {
        expect(uri.path, '/v1/models');
        final map = jsonDecode(body as String) as Map<String, dynamic>;
        expect(map['id'], 'composer-2.5');
        return http.Response(
          jsonEncode({
            'default': 'auto',
            'selected': [
              {'id': 'auto', 'label': 'Auto'},
              {'id': 'composer-2.5', 'label': 'Composer 2.5'},
            ],
            'available': <dynamic>[],
          }),
          200,
        );
      },
    );
    final v = await api.addModel(id: 'composer-2.5');
    expect(v.selected.any((m) => m.id == 'composer-2.5'), isTrue);
  });
}
