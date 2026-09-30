import 'dart:convert';

import 'package:cloud_agent_app/auth/session.dart';
import 'package:cloud_agent_app/slaves/models.dart';
import 'package:cloud_agent_app/slaves/slaves_api.dart';
import 'package:cloud_agent_app/ui/milestone_list_page.dart';
import 'package:cloud_agent_app/ui/milestone_phases_page.dart';
import 'package:cloud_agent_app/ui/slave_list_page.dart';
import 'package:cloud_agent_app/workflow/models.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;

const _session = Session(
  gatewayBaseUrl: 'http://127.0.0.1:8080',
  token: 'tok_slaves_test_xxxx',
);

Map<String, dynamic> _slaveJson({bool online = true}) => {
      'id': 'slave_devpc',
      'name': 'Dev PC',
      'online': online,
      'repos': [
        {
          'id': 'r_cloud_agent',
          'name': 'cloud-agent',
          'cwd': 'E:/workspace/cloud-agent',
        },
      ],
      'projects': [
        {
          'id': 'r_cloud_agent',
          'name': 'cloud-agent',
          'cwd': 'E:/workspace/cloud-agent',
          'index': 'ai/milestones.json',
          'milestones': [
            {
              'id': 'M07',
              'title': 'Harden and wrap-up',
              'progressDoc': 'ai/progress.md',
              'phases': [
                {
                  'id': 'M07-P01',
                  'title': 'Audit rate limit',
                  'phaseRef': 'doc/p.md',
                  'dependsOn': <String>[],
                  'model': 'composer-2.5',
                },
              ],
            },
          ],
        },
      ],
    };

void main() {
  test('SlaveInfo parses projects and milestones', () {
    final s = SlaveInfo.fromJson(_slaveJson());
    expect(s.id, 'slave_devpc');
    expect(s.online, isTrue);
    expect(s.effectiveProjects.length, 1);
    expect(s.effectiveProjects.first.milestones.length, 1);
    expect(s.effectiveProjects.first.milestones.first.id, 'M07');
  });

  test('SlavesApi.listSlaves', () async {
    final api = SlavesApi(
      session: _session,
      get: (uri, {headers}) async {
        expect(uri.path, '/v1/slaves');
        expect(headers?['Authorization'], 'Bearer tok_slaves_test_xxxx');
        return http.Response(
          jsonEncode({
            'slaves': [_slaveJson()],
          }),
          200,
        );
      },
    );
    final list = await api.listSlaves();
    expect(list.length, 1);
    expect(list.first.projects.first.index, 'ai/milestones.json');
  });

  testWidgets('slave → project → milestone navigation', (tester) async {
    final api = SlavesApi(
      session: _session,
      get: (uri, {headers}) async {
        return http.Response(
          jsonEncode({
            'slaves': [_slaveJson()],
          }),
          200,
        );
      },
    );

    await tester.pumpWidget(
      MaterialApp(
        home: SlaveListPage(
          session: _session,
          api: api,
          pollInterval: const Duration(days: 1),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Dev PC'), findsOneWidget);
    await tester.tap(find.text('Dev PC'));
    await tester.pumpAndSettle();

    // M11 1:1 — skip project list, land on milestones.
    expect(find.byType(MilestoneListPage), findsOneWidget);
    expect(find.textContaining('M07'), findsOneWidget);
  });

  test('findMilestoneRun prefers active over older completed', () {
    final runs = [
      WorkflowRun.fromJson({
        'id': 'wf_old',
        'bundleId': 'milestone:M01',
        'repoId': 'r1',
        'slaveId': 's1',
        'status': 'completed',
        'nodes': [
          {'id': 'A', 'status': 'approved', 'dependsOn': []},
        ],
        'createdAt': '2026-09-28T00:00:00Z',
      }),
      WorkflowRun.fromJson({
        'id': 'wf_active',
        'bundleId': 'milestone:M01',
        'repoId': 'r1',
        'slaveId': 's1',
        'status': 'running',
        'nodes': [
          {'id': 'A', 'status': 'approved', 'dependsOn': []},
          {'id': 'B', 'status': 'ready', 'dependsOn': ['A']},
        ],
        'createdAt': '2026-09-29T00:00:00Z',
      }),
    ];
    final hit = findMilestoneRun(
      runs,
      bundleId: 'milestone:M01',
      repoId: 'r1',
      slaveId: 's1',
    );
    expect(hit?.id, 'wf_active');
    expect(hit?.nodes.first.status, 'approved');
  });

  test('phase toWorkflowNodeJson includes prompt', () {
    final p = MilestonePhaseInfo.fromJson({
      'id': 'M07-P01',
      'title': '审计',
      'phaseRef': 'doc/p.md',
      'dependsOn': <String>[],
      'model': 'composer-2.5',
    });
    final n = p.toWorkflowNodeJson();
    expect(n['id'], 'M07-P01');
    expect(n['prompt'], {'mode': 'phase_file'});
    expect(n['model'], 'composer-2.5');
  });
}
