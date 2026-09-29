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

  test('createWorkflow posts nodes and bundleId', () async {
    final api = WorkflowApi(
      session: session,
      post: (uri, {headers, body}) async {
        expect(uri.path, '/v1/workflows');
        final map = jsonDecode(body as String) as Map<String, dynamic>;
        expect(map['bundleId'], 'milestone:M07');
        expect(map['repoId'], 'r_cloud_agent');
        expect(map['slaveId'], 'slave_devpc');
        expect((map['nodes'] as List).length, 1);
        return http.Response(
          jsonEncode({
            'id': 'wf_new',
            'bundleId': 'milestone:M07',
            'status': 'pending',
            'repoId': 'r_cloud_agent',
            'nodes': [
              {'id': 'M07-P01', 'status': 'ready', 'dependsOn': []},
            ],
            'createdAt': '2026-09-28T00:00:00Z',
          }),
          201,
        );
      },
    );
    final wf = await api.createWorkflow(
      bundleId: 'milestone:M07',
      repoId: 'r_cloud_agent',
      slaveId: 'slave_devpc',
      progressDoc: 'ai/progress.md',
      nodes: [
        {
          'id': 'M07-P01',
          'title': '审计',
          'phaseRef': 'doc/p.md',
          'dependsOn': <String>[],
          'prompt': {'mode': 'phase_file'},
        },
      ],
    );
    expect(wf.id, 'wf_new');
    expect(wf.bundleId, 'milestone:M07');
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

  test('WorkflowRun.canStart only when pending', () {
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

    final running = WorkflowRun.fromJson({
      'id': 'wf2',
      'bundleId': 'b',
      'status': 'running',
      'nodes': [
        {'id': 'A', 'status': 'ready'},
      ],
      'createdAt': '2026-09-28T00:00:00Z',
    });
    expect(running.canStart, isFalse);
    expect(running.isActive, isTrue);

    final pending = WorkflowRun.fromJson({
      'id': 'wf3',
      'bundleId': 'b',
      'status': 'pending',
      'nodes': [
        {'id': 'A', 'status': 'ready'},
      ],
      'createdAt': '2026-09-28T00:00:00Z',
    });
    expect(pending.canStart, isTrue);
  });

  test('awaiting_review flag on node', () {
    final n = WorkflowNode.fromJson({
      'id': 'N1',
      'status': 'awaiting_review',
    });
    expect(n.isAwaitingReview, isTrue);
  });

  test('NodePolicy defaults off and patchNode posts policy', () async {
    final n = WorkflowNode.fromJson({'id': 'A', 'status': 'ready'});
    expect(n.policy.autoApprove, isFalse);
    expect(n.policy.autoStartNext, isFalse);
    expect(n.model, 'auto');

    var patched = false;
    final api = WorkflowApi(
      session: session,
      patch: (uri, {headers, body}) async {
        patched = true;
        expect(uri.path, '/v1/workflows/wf_1/nodes/A');
        final map = jsonDecode(body as String) as Map;
        expect(map['model'], 'composer-2.5');
        expect(map['policy']['autoStartNext'], isTrue);
        expect(map['policy']['autoApprove'], isFalse);
        return http.Response(
          jsonEncode({
            'id': 'wf_1',
            'bundleId': 'b',
            'status': 'running',
            'nodes': [
              {
                'id': 'A',
                'status': 'ready',
                'model': 'composer-2.5',
                'policy': {'autoApprove': false, 'autoStartNext': true},
              },
            ],
            'createdAt': '2026-09-28T00:00:00Z',
          }),
          200,
        );
      },
    );
    final run = await api.patchNode(
      'wf_1',
      'A',
      model: 'composer-2.5',
      policy: const NodePolicy(autoStartNext: true),
    );
    expect(patched, isTrue);
    expect(run.nodes.first.policy.autoStartNext, isTrue);
    expect(run.nodes.first.model, 'composer-2.5');
  });

  test('continueWorkflow hits continue path', () async {
    final api = WorkflowApi(
      session: session,
      post: (uri, {headers, body}) async {
        expect(uri.path, '/v1/workflows/wf_1/continue');
        return http.Response(
          jsonEncode({
            'delivered': true,
            'workflow': {
              'id': 'wf_1',
              'bundleId': 'b',
              'status': 'running',
              'nodes': [
                {'id': 'B', 'status': 'running', 'dependsOn': ['A']},
              ],
              'createdAt': '2026-09-28T00:00:00Z',
            },
          }),
          200,
        );
      },
    );
    final r = await api.continueWorkflow('wf_1');
    expect(r.delivered, isTrue);
  });
}
