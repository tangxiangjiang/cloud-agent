import 'dart:convert';

import 'package:cloud_agent_app/auth/session.dart';
import 'package:cloud_agent_app/workflow/models.dart';
import 'package:cloud_agent_app/workflow/workflow_api.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;

void main() {
  const session = Session(
    gatewayBaseUrl: 'http://127.0.0.1:8080',
    token: 'tok_test_abcdefgh',
  );

  test('listWorkflows parses id and status', () async {
    final api = WorkflowApi(
      session: session,
      get: (uri, {headers}) async {
        expect(uri.path, '/v1/workflows');
        expect(headers?['Authorization'], 'Bearer tok_test_abcdefgh');
        return http.Response(
          jsonEncode({
            'workflows': [
              {
                'id': 'wf_1',
                'bundleId': 'b',
                'status': 'pending',
                'nodes': [
                  {'id': 'A', 'status': 'ready', 'dependsOn': []},
                ],
                'createdAt': '2026-09-28T00:00:00Z',
              },
            ],
          }),
          200,
        );
      },
    );
    final list = await api.listWorkflows();
    expect(list.length, 1);
    expect(list.first.id, 'wf_1');
    expect(list.first.status, 'pending');
    expect(list.first.nodes.first.status, 'ready');
  });

  test('startWorkflow returns delivered flag', () async {
    final api = WorkflowApi(
      session: session,
      post: (uri, {headers, body}) async {
        expect(uri.path, '/v1/workflows/wf_1/start');
        return http.Response(
          jsonEncode({
            'delivered': true,
            'workflow': {
              'id': 'wf_1',
              'bundleId': 'b',
              'status': 'running',
              'nodes': [
                {'id': 'A', 'status': 'ready', 'dependsOn': []},
              ],
              'createdAt': '2026-09-28T00:00:00Z',
            },
          }),
          200,
        );
      },
    );
    final r = await api.startWorkflow('wf_1');
    expect(r.delivered, isTrue);
    expect(r.workflow.status, 'running');
  });

  test('errors surface message for UI', () async {
    final api = WorkflowApi(
      session: session,
      get: (uri, {headers}) async =>
          http.Response(jsonEncode({'error': 'not found'}), 404),
    );
    expect(
      () => api.getWorkflow('missing'),
      throwsA(
        isA<WorkflowApiException>().having(
          (e) => e.message,
          'message',
          contains('not found'),
        ),
      ),
    );
  });

  test('WorkflowRun.canStart false when terminal', () {
    final done = WorkflowRun.fromJson({
      'id': 'wf',
      'bundleId': 'b',
      'status': 'completed',
      'nodes': [
        {'id': 'A', 'status': 'approved'},
      ],
      'createdAt': '2026-09-28T00:00:00Z',
    });
    expect(done.canStart, isFalse);
  });

  test('awaiting_review flag on node', () {
    final n = WorkflowNode.fromJson({
      'id': 'N1',
      'status': 'awaiting_review',
    });
    expect(n.isAwaitingReview, isTrue);
  });
}
