import 'dart:convert';

import 'package:cloud_agent_app/auth/session.dart';
import 'package:cloud_agent_app/ui/workflow_detail_page.dart';
import 'package:cloud_agent_app/ui/workflow_list_page.dart';
import 'package:cloud_agent_app/workflow/models.dart';
import 'package:cloud_agent_app/workflow/workflow_api.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;

const _session = Session(
  gatewayBaseUrl: 'http://127.0.0.1:8080',
  token: 'tok_ui_test_xxxx',
);

WorkflowRun _sample({
  String status = 'pending',
  String nodeStatus = 'ready',
}) {
  return WorkflowRun.fromJson({
    'id': 'wf_demo',
    'bundleId': 'bundle_x',
    'status': status,
    'repoId': 'r1',
    'slaveId': 's1',
    'nodes': [
      {
        'id': 'A',
        'title': 'First',
        'status': nodeStatus,
        'dependsOn': <String>[],
      },
      {
        'id': 'B',
        'title': 'Second',
        'status': 'pending',
        'dependsOn': ['A'],
      },
    ],
    'createdAt': '2026-09-28T00:00:00Z',
  });
}

void main() {
  testWidgets('list shows workflow id and status', (tester) async {
    final api = WorkflowApi(
      session: _session,
      get: (uri, {headers}) async {
        if (uri.path == '/v1/workflows') {
          return http.Response(
            jsonEncode({
              'workflows': [
                {
                  'id': 'wf_demo',
                  'bundleId': 'bundle_x',
                  'status': 'running',
                  'nodes': [
                    {'id': 'A', 'status': 'awaiting_review', 'dependsOn': []},
                  ],
                  'createdAt': '2026-09-28T00:00:00Z',
                },
              ],
            }),
            200,
          );
        }
        return http.Response('{}', 404);
      },
    );

    await tester.pumpWidget(
      MaterialApp(
        home: WorkflowListPage(
          session: _session,
          api: api,
          pollInterval: const Duration(days: 1),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('wf_demo'), findsOneWidget);
    expect(find.textContaining('awaiting review'), findsOneWidget);
  });

  testWidgets('detail highlights awaiting_review and can start', (tester) async {
    var started = false;
    final initial = _sample(nodeStatus: 'awaiting_review');
    final api = WorkflowApi(
      session: _session,
      get: (uri, {headers}) async {
        if (uri.path == '/v1/models') {
          return http.Response(
            jsonEncode({
              'default': 'auto',
              'models': [
                {'id': 'auto', 'label': 'Auto'},
              ],
            }),
            200,
          );
        }
        return http.Response(
          jsonEncode({
            'id': initial.id,
            'bundleId': initial.bundleId,
            'status': initial.status,
            'repoId': 'r1',
            'slaveId': 's1',
            'nodes': [
              {
                'id': 'A',
                'title': 'First',
                'status': 'awaiting_review',
                'dependsOn': <String>[],
                'model': 'auto',
                'policy': {'autoApprove': false, 'autoStartNext': false},
              },
              {
                'id': 'B',
                'title': 'Second',
                'status': 'pending',
                'dependsOn': ['A'],
                'model': 'auto',
                'policy': {'autoApprove': false, 'autoStartNext': false},
              },
            ],
            'createdAt': initial.createdAt,
          }),
          200,
        );
      },
      post: (uri, {headers, body}) async {
        started = true;
        expect(uri.path, endsWith('/start'));
        return http.Response(
          jsonEncode({
            'delivered': false,
            'workflow': {
              'id': 'wf_demo',
              'bundleId': 'bundle_x',
              'status': 'running',
              'nodes': [
                {
                  'id': 'A',
                  'title': 'First',
                  'status': 'awaiting_review',
                  'dependsOn': <String>[],
                  'model': 'auto',
                  'policy': {'autoApprove': false, 'autoStartNext': false},
                },
              ],
              'createdAt': '2026-09-28T00:00:00Z',
            },
          }),
          200,
        );
      },
    );

    await tester.pumpWidget(
      MaterialApp(
        home: WorkflowDetailPage(
          session: _session,
          workflowId: 'wf_demo',
          api: api,
          initial: initial,
          pollInterval: const Duration(days: 1),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('待审核'), findsWidgets);
    expect(find.text('Start'), findsOneWidget);
    // Combined auto-continue switch defaults off.
    final autoContinue = tester.widget<Switch>(
      find.byKey(const ValueKey('autoContinue-A')),
    );
    expect(autoContinue.value, isFalse);
    expect(find.text('Auto'), findsWidgets);

    await tester.tap(find.text('Start'));
    await tester.pumpAndSettle();
    expect(started, isTrue);
    expect(find.textContaining('Started'), findsOneWidget);
  });

  testWidgets('ready node shows Start and Continue after approve path', (tester) async {
    var patched = false;
    var nodeStarted = false;
    final initial = WorkflowRun.fromJson({
      'id': 'wf_ready',
      'bundleId': 'b',
      'status': 'running',
      'nodes': [
        {
          'id': 'A',
          'title': 'Done',
          'status': 'approved',
          'dependsOn': <String>[],
          'model': 'auto',
          'policy': {'autoApprove': false, 'autoStartNext': false},
        },
        {
          'id': 'B',
          'title': 'Next',
          'status': 'ready',
          'dependsOn': ['A'],
          'model': 'auto',
          'policy': {'autoApprove': false, 'autoStartNext': false},
        },
      ],
      'createdAt': '2026-09-28T00:00:00Z',
    });

    final api = WorkflowApi(
      session: _session,
      get: (uri, {headers}) async {
        if (uri.path == '/v1/models') {
          return http.Response(
            jsonEncode({
              'default': 'auto',
              'models': [
                {'id': 'auto', 'label': 'Auto'},
                {'id': 'composer-2.5', 'label': 'Composer 2.5'},
              ],
            }),
            200,
          );
        }
        return http.Response(jsonEncode({
          'id': initial.id,
          'bundleId': initial.bundleId,
          'status': initial.status,
          'nodes': initial.nodes
              .map((n) => {
                    'id': n.id,
                    'title': n.title,
                    'status': n.status,
                    'dependsOn': n.dependsOn,
                    'model': n.model,
                    'policy': n.policy.toJson(),
                  })
              .toList(),
          'createdAt': initial.createdAt,
        }), 200);
      },
      post: (uri, {headers, body}) async {
        if (uri.path.endsWith('/continue')) {
          return http.Response(
            jsonEncode({
              'delivered': true,
              'workflow': {
                'id': 'wf_ready',
                'bundleId': 'b',
                'status': 'running',
                'nodes': [
                  {
                    'id': 'B',
                    'title': 'Next',
                    'status': 'running',
                    'dependsOn': ['A'],
                    'model': 'auto',
                    'policy': {'autoApprove': false, 'autoStartNext': false},
                  },
                ],
                'createdAt': '2026-09-28T00:00:00Z',
              },
            }),
            200,
          );
        }
        if (uri.path.endsWith('/nodes/B/start')) {
          nodeStarted = true;
          return http.Response(
            jsonEncode({
              'delivered': true,
              'workflow': {
                'id': 'wf_ready',
                'bundleId': 'b',
                'status': 'running',
                'nodes': [
                  {
                    'id': 'B',
                    'title': 'Next',
                    'status': 'running',
                    'dependsOn': ['A'],
                    'model': 'auto',
                    'policy': {'autoApprove': false, 'autoStartNext': false},
                  },
                ],
                'createdAt': '2026-09-28T00:00:00Z',
              },
            }),
            200,
          );
        }
        return http.Response('{}', 404);
      },
      patch: (uri, {headers, body}) async {
        patched = true;
        final map = jsonDecode(body as String) as Map;
        expect(map['policy']['autoApprove'], isTrue);
        expect(map['policy']['autoStartNext'], isTrue);
        return http.Response(
          jsonEncode({
            'id': 'wf_ready',
            'bundleId': 'b',
            'status': 'running',
            'nodes': [
              {
                'id': 'A',
                'title': 'Done',
                'status': 'approved',
                'dependsOn': <String>[],
                'model': 'auto',
                'policy': {'autoApprove': false, 'autoStartNext': false},
              },
              {
                'id': 'B',
                'title': 'Next',
                'status': 'ready',
                'dependsOn': ['A'],
                'model': 'auto',
                'policy': {'autoApprove': true, 'autoStartNext': true},
              },
            ],
            'createdAt': '2026-09-28T00:00:00Z',
          }),
          200,
        );
      },
    );

    await tester.pumpWidget(
      MaterialApp(
        home: WorkflowDetailPage(
          session: _session,
          workflowId: 'wf_ready',
          api: api,
          initial: initial,
          pollInterval: const Duration(days: 1),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Continue'), findsOneWidget);
    expect(find.byKey(const ValueKey('startNode-B')), findsOneWidget);

    // Toggle autoContinue on B → PATCH both flags
    await tester.tap(find.byKey(const ValueKey('autoContinue-B')));
    await tester.pumpAndSettle();
    expect(patched, isTrue);
    final sw = tester.widget<Switch>(
      find.byKey(const ValueKey('autoContinue-B')),
    );
    expect(sw.value, isTrue);

    await tester.tap(find.byKey(const ValueKey('startNode-B')));
    await tester.pumpAndSettle();
    expect(nodeStarted, isTrue);
  });

  testWidgets('detail shows error snackbar path via failed load', (tester) async {
    final api = WorkflowApi(
      session: _session,
      get: (uri, {headers}) async =>
          http.Response(jsonEncode({'error': 'boom'}), 500),
    );

    await tester.pumpWidget(
      MaterialApp(
        home: WorkflowDetailPage(
          session: _session,
          workflowId: 'wf_x',
          api: api,
          pollInterval: const Duration(days: 1),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.textContaining('boom'), findsWidgets);
    expect(find.text('Retry'), findsOneWidget);
  });
}
