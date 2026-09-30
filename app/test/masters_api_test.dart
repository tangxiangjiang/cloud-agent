import 'dart:convert';

import 'package:cloud_agent_app/auth/session.dart';
import 'package:cloud_agent_app/masters/masters_api.dart';
import 'package:cloud_agent_app/masters/models.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;

const _session = Session(
  gatewayBaseUrl: 'http://127.0.0.1:8080',
  token: 'tok_masters_test',
);

Map<String, dynamic> _masterJson({String process = 'stopped'}) => {
      'masterId': 'master_dev',
      'name': 'PC',
      'online': true,
      'slaves': [
        {
          'id': 'slave_a',
          'name': 'cloud-agent',
          'enabled': true,
          'process': process,
          'gatewayOnline': process == 'running',
          'lastError': null,
          'project': {
            'id': 'r_a',
            'name': 'cloud-agent',
            'cwd': 'E:/workspace/cloud-agent',
          },
        },
      ],
    };

void main() {
  test('FleetSlave.canEnterProject requires running and gatewayOnline', () {
    final stopped = FleetSlave.fromJson({
      'id': 's',
      'enabled': true,
      'process': 'stopped',
      'gatewayOnline': false,
      'project': {'id': 'r', 'name': 'n', 'cwd': '/x'},
    });
    expect(stopped.canEnterProject, isFalse);

    final ready = FleetSlave.fromJson({
      'id': 's',
      'enabled': true,
      'process': 'running',
      'gatewayOnline': true,
      'project': {'id': 'r', 'name': 'n', 'cwd': '/x'},
    });
    expect(ready.canEnterProject, isTrue);
  });

  test('MastersApi.listMasters parses response', () async {
    final api = MastersApi(
      session: _session,
      get: (uri, {headers}) async {
        expect(uri.path, '/v1/masters');
        expect(headers?['Authorization'], 'Bearer tok_masters_test');
        return http.Response(
          jsonEncode({
            'masters': [_masterJson()],
          }),
          200,
        );
      },
    );
    final list = await api.listMasters();
    expect(list.length, 1);
    expect(list.first.masterId, 'master_dev');
    expect(list.first.slaves.first.process, 'stopped');
  });

  test('MastersApi.startSlave maps 504', () async {
    final api = MastersApi(
      session: _session,
      send: (method, uri, {headers, body}) async {
        expect(method, 'POST');
        expect(uri.path, '/v1/masters/master_dev/slaves/slave_a/start');
        return http.Response(
          jsonEncode({
            'error': 'master control timed out',
            'code': 'master_control_timeout',
          }),
          504,
        );
      },
    );
    await expectLater(
      api.startSlave('master_dev', 'slave_a'),
      throwsA(
        isA<MastersApiException>()
            .having((e) => e.statusCode, 'status', 504)
            .having((e) => e.code, 'code', 'master_control_timeout'),
      ),
    );
  });

  test('MastersApi.startSlave success returns master', () async {
    final api = MastersApi(
      session: _session,
      send: (method, uri, {headers, body}) async {
        return http.Response(
          jsonEncode({
            'ok': true,
            'master': _masterJson(process: 'running'),
          }),
          200,
        );
      },
    );
    final m = await api.startSlave('master_dev', 'slave_a');
    expect(m.slaves.first.process, 'running');
    expect(m.slaves.first.canEnterProject, isTrue);
  });
}
