import 'dart:convert';

import 'package:cloud_agent_app/auth/session.dart';
import 'package:cloud_agent_app/chat/chat_api.dart';
import 'package:cloud_agent_app/slaves/models.dart';
import 'package:cloud_agent_app/ui/milestone_phases_page.dart';
import 'package:cloud_agent_app/workflow/workflow_api.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;

const _session = Session(
  gatewayBaseUrl: 'http://127.0.0.1:8080',
  token: 'tok_m10p04',
);

void main() {
  testWidgets('execute plan shows default policy off and posts defaults',
      (tester) async {
    Map<String, dynamic>? createBody;
    var startCalled = false;
    final slave = SlaveInfo.fromJson({
      'id': 'slave_devpc',
      'name': 'Dev',
      'online': true,
      'projects': [
        {
          'id': 'r1',
          'name': 'proj',
          'cwd': 'E:/x',
          'milestones': [
            {
              'id': 'M10',
              'title': 'Node policy',
              'progressDoc': 'ai/progress.md',
              'phases': [
                {
                  'id': 'M10-P04',
                  'title': 'Defaults',
                  'phaseRef': 'doc/p.md',
                  'dependsOn': <String>[],
                },
              ],
            },
          ],
        },
      ],
    });
    final project = slave.effectiveProjects.first;
    final milestone = project.milestones.first;

    final api = WorkflowApi(
      session: _session,
      get: (uri, {headers}) async {
        if (uri.path == '/v1/workflows') {
          return http.Response(jsonEncode({'workflows': <dynamic>[]}), 200);
        }
        return http.Response('{}', 404);
      },
      post: (uri, {headers, body}) async {
        if (uri.path == '/v1/workflows') {
          createBody = jsonDecode(body as String) as Map<String, dynamic>;
          return http.Response(
            jsonEncode({
              'id': 'wf_new',
              'bundleId': 'milestone:M10',
              'status': 'pending',
              'repoId': 'r1',
              'slaveId': 'slave_devpc',
              'nodes': [
                {
                  'id': 'M10-P04',
                  'status': 'ready',
                  'model': 'composer-2.5',
                  'policy': {
                    'autoApprove': false,
                    'autoStartNext': true,
                  },
                  'dependsOn': <String>[],
                },
              ],
              'createdAt': '2026-09-29T00:00:00Z',
            }),
            201,
          );
        }
        if (uri.path.endsWith('/start')) {
          startCalled = true;
          return http.Response(
            jsonEncode({
              'delivered': true,
              'workflow': {
                'id': 'wf_new',
                'bundleId': 'milestone:M10',
                'status': 'running',
                'repoId': 'r1',
                'nodes': [
                  {
                    'id': 'M10-P04',
                    'status': 'ready',
                    'model': 'composer-2.5',
                    'policy': {
                      'autoApprove': false,
                      'autoStartNext': true,
                    },
                    'dependsOn': <String>[],
                  },
                ],
                'createdAt': '2026-09-29T00:00:00Z',
              },
            }),
            200,
          );
        }
        return http.Response('{}', 404);
      },
    );

    final chatApi = ChatApi(
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
        return http.Response('{}', 404);
      },
    );

    await tester.pumpWidget(
      MaterialApp(
        home: MilestonePhasesPage(
          session: _session,
          slave: slave,
          project: project,
          milestone: milestone,
          api: api,
          chatApi: chatApi,
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('自动续跑（总开关）'), findsOneWidget);
    final autoContinue = tester.widget<Switch>(
      find.byKey(const ValueKey('milestone-default-autoContinue')),
    );
    expect(autoContinue.value, isFalse);

    await tester.tap(find.byKey(const ValueKey('milestone-default-autoContinue')));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const ValueKey('milestone-default-model')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Composer 2.5').last);
    await tester.pumpAndSettle();

    await tester.tap(find.text('创建此 Milestone'));
    await tester.pumpAndSettle();

    expect(createBody, isNotNull);
    expect(createBody!['defaultModel'], 'composer-2.5');
    expect(createBody!['defaultPolicy']['autoApprove'], isTrue);
    expect(createBody!['defaultPolicy']['autoStartNext'], isTrue);
    expect(startCalled, isFalse);
  });

  testWidgets('master switch patches all nodes on existing workflow',
      (tester) async {
    final patched = <String>[];
    final slave = SlaveInfo.fromJson({
      'id': 'slave_devpc',
      'name': 'Dev',
      'online': true,
      'projects': [
        {
          'id': 'r1',
          'name': 'proj',
          'cwd': 'E:/x',
          'milestones': [
            {
              'id': 'M10',
              'title': 'Node policy',
              'progressDoc': 'ai/progress.md',
              'phases': [
                {
                  'id': 'P1',
                  'title': 'One',
                  'phaseRef': 'doc/a.md',
                  'dependsOn': <String>[],
                },
                {
                  'id': 'P2',
                  'title': 'Two',
                  'phaseRef': 'doc/b.md',
                  'dependsOn': <String>['P1'],
                },
              ],
            },
          ],
        },
      ],
    });
    final project = slave.effectiveProjects.first;
    final milestone = project.milestones.first;

    Map<String, dynamic> runJson({
      required bool a,
      required bool b,
    }) =>
        {
          'id': 'wf_m',
          'bundleId': 'milestone:M10',
          'status': 'running',
          'repoId': 'r1',
          'slaveId': 'slave_devpc',
          'nodes': [
            {
              'id': 'P1',
              'status': 'approved',
              'model': 'auto',
              'policy': {'autoApprove': a, 'autoStartNext': a},
              'dependsOn': <String>[],
            },
            {
              'id': 'P2',
              'status': 'ready',
              'model': 'auto',
              'policy': {'autoApprove': b, 'autoStartNext': b},
              'dependsOn': <String>['P1'],
            },
          ],
          'createdAt': '2026-09-29T00:00:00Z',
        };

    final api = WorkflowApi(
      session: _session,
      get: (uri, {headers}) async {
        if (uri.path == '/v1/workflows') {
          return http.Response(
            jsonEncode({
              'workflows': [runJson(a: false, b: false)],
            }),
            200,
          );
        }
        return http.Response('{}', 404);
      },
      patch: (uri, {headers, body}) async {
        final id = uri.pathSegments.last;
        patched.add(id);
        final map = jsonDecode(body as String) as Map;
        expect(map['policy']['autoApprove'], isTrue);
        expect(map['policy']['autoStartNext'], isTrue);
        final a = patched.contains('P1') || id == 'P1';
        final b = patched.contains('P2') || id == 'P2';
        return http.Response(
          jsonEncode(runJson(a: a, b: b)),
          200,
        );
      },
    );

    final chatApi = ChatApi(
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
        return http.Response('{}', 404);
      },
    );

    await tester.pumpWidget(
      MaterialApp(
        home: MilestonePhasesPage(
          session: _session,
          slave: slave,
          project: project,
          milestone: milestone,
          api: api,
          chatApi: chatApi,
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('自动续跑（总开关）'), findsOneWidget);
    expect(
      tester
          .widget<Switch>(
            find.byKey(const ValueKey('milestone-default-autoContinue')),
          )
          .value,
      isFalse,
    );

    await tester.tap(find.byKey(const ValueKey('milestone-default-autoContinue')));
    await tester.pumpAndSettle();

    expect(patched.toSet(), {'P1', 'P2'});
    expect(
      tester
          .widget<Switch>(
            find.byKey(const ValueKey('milestone-default-autoContinue')),
          )
          .value,
      isTrue,
    );
  });
}
