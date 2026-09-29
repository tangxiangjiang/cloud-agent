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

    expect(find.text('本 Milestone 默认策略'), findsOneWidget);
    final autoApprove = tester.widget<Switch>(
      find.byKey(const ValueKey('milestone-default-autoApprove')),
    );
    final autoNext = tester.widget<Switch>(
      find.byKey(const ValueKey('milestone-default-autoStartNext')),
    );
    expect(autoApprove.value, isFalse);
    expect(autoNext.value, isFalse);

    await tester.tap(find.byKey(const ValueKey('milestone-default-autoStartNext')));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const ValueKey('milestone-default-model')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Composer 2.5').last);
    await tester.pumpAndSettle();

    await tester.tap(find.text('开始此 Milestone'));
    await tester.pumpAndSettle();

    expect(createBody, isNotNull);
    expect(createBody!['defaultModel'], 'composer-2.5');
    expect(createBody!['defaultPolicy']['autoApprove'], isFalse);
    expect(createBody!['defaultPolicy']['autoStartNext'], isTrue);
  });
}
