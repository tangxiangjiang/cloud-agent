import 'dart:convert';

import 'package:cloud_agent_app/auth/session.dart';
import 'package:cloud_agent_app/slaves/models.dart';
import 'package:cloud_agent_app/sync/models.dart';
import 'package:cloud_agent_app/sync/project_sync_api.dart';
import 'package:cloud_agent_app/ui/milestone_list_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;

const _session = Session(
  gatewayBaseUrl: 'http://127.0.0.1:8080',
  token: 'tok_sync_test_xxxx',
);

Map<String, dynamic> _snapJson({
  String syncedAt = '2026-09-29T10:00:00Z',
  List<Map<String, dynamic>>? warnings,
}) =>
    {
      'slaveId': 'slave_devpc',
      'repoId': 'r_cloud_agent',
      'syncedAt': syncedAt,
      'payload': {
        'schemaVersion': 1,
        'branch': 'main',
        'dirty': false,
        'summary': 'branch main; working tree clean.',
        'phases': [
          {'id': 'M07-P01', 'progressStatus': 'approved'},
        ],
      },
      'warnings': warnings ??
          [
            {
              'code': 'progress_ahead',
              'message': 'Local progress ahead of Gateway for M07-P01',
              'phaseId': 'M07-P01',
              'workflowId': 'wf_continue',
              'suggestion': 'continue_workflow',
            },
          ],
      'report': {
        'workflowId': 'wf_continue',
        'bundleId': 'milestone:M07',
        'workflowStatus': 'running',
        'active': true,
        'branch': 'main',
        'dirty': false,
        'phases': [
          {
            'id': 'M07-P01',
            'progressStatus': 'approved',
            'nodeStatus': 'ready',
            'workflowId': 'wf_continue',
          },
        ],
      },
    };

SlaveInfo _slave({bool online = true}) => SlaveInfo.fromJson({
      'id': 'slave_devpc',
      'name': 'Dev PC',
      'online': online,
      'projects': [
        {
          'id': 'r_cloud_agent',
          'name': 'cloud-agent',
          'cwd': 'E:/workspace/cloud-agent',
          'index': 'ai/milestones.json',
          'milestones': [
            {
              'id': 'M07',
              'title': 'Harden',
              'phases': [
                {
                  'id': 'M07-P01',
                  'title': 'Audit',
                  'phaseRef': 'doc/p.md',
                  'dependsOn': <String>[],
                },
              ],
            },
          ],
        },
      ],
    });

void main() {
  test('ProjectSyncSnapshot parses warnings and report', () {
    final s = ProjectSyncSnapshot.fromJson(_snapJson());
    expect(s.branch, 'main');
    expect(s.dirty, isFalse);
    expect(s.summary, contains('clean'));
    expect(s.warnings.length, 1);
    expect(s.warnings.first.canContinueWorkflow, isTrue);
    expect(s.report?.phases.first.nodeStatus, 'ready');
  });

  test('ProjectSyncApi trigger + getSync', () async {
    final api = ProjectSyncApi(
      session: _session,
      get: (uri, {headers}) async {
        expect(uri.path, '/v1/slaves/slave_devpc/projects/r_cloud_agent/sync');
        expect(headers?['Authorization'], 'Bearer tok_sync_test_xxxx');
        return http.Response(jsonEncode(_snapJson()), 200);
      },
      post: (uri, {headers, body}) async {
        expect(uri.path, endsWith('/sync'));
        return http.Response(
          jsonEncode({
            'requestId': 'req_1',
            'status': 'accepted',
            'slaveId': 'slave_devpc',
            'repoId': 'r_cloud_agent',
          }),
          202,
        );
      },
    );

    final trig = await api.triggerSync(
      slaveId: 'slave_devpc',
      repoId: 'r_cloud_agent',
    );
    expect(trig.requestId, 'req_1');

    final snap = await api.getSync(
      slaveId: 'slave_devpc',
      repoId: 'r_cloud_agent',
    );
    expect(snap.warnings.first.workflowId, 'wf_continue');
  });

  test('ProjectSyncApi offline is 409', () async {
    final api = ProjectSyncApi(
      session: _session,
      post: (uri, {headers, body}) async => http.Response(
        jsonEncode({'error': 'slave offline'}),
        409,
      ),
      get: (uri, {headers}) async => http.Response('{}', 404),
    );
    expect(
      () => api.triggerSync(slaveId: 's', repoId: 'r'),
      throwsA(
        isA<ProjectSyncApiException>().having((e) => e.statusCode, 'code', 409),
      ),
    );
  });

  test('syncAndWait returns when syncedAt changes', () async {
    var gets = 0;
    final api = ProjectSyncApi(
      session: _session,
      post: (uri, {headers, body}) async => http.Response(
        jsonEncode({'requestId': 'req_x', 'status': 'accepted'}),
        202,
      ),
      get: (uri, {headers}) async {
        gets++;
        if (gets == 1) {
          return http.Response(
            jsonEncode(_snapJson(syncedAt: '2026-09-29T10:00:00Z', warnings: [])),
            200,
          );
        }
        return http.Response(
          jsonEncode(_snapJson(syncedAt: '2026-09-29T10:00:05Z', warnings: [])),
          200,
        );
      },
    );
    final snap = await api.syncAndWait(
      slaveId: 'slave_devpc',
      repoId: 'r_cloud_agent',
      interval: const Duration(milliseconds: 1),
      timeout: const Duration(seconds: 2),
    );
    expect(snap.syncedAt, '2026-09-29T10:00:05Z');
  });

  testWidgets('project page sync disabled when offline', (tester) async {
    final api = ProjectSyncApi(
      session: _session,
      get: (uri, {headers}) async => http.Response('{"error":"not found"}', 404),
      post: (uri, {headers, body}) async =>
          http.Response('{"error":"slave offline"}', 409),
    );
    final slave = _slave(online: false);
    final project = slave.effectiveProjects.first;

    await tester.pumpWidget(
      MaterialApp(
        home: MilestoneListPage(
          session: _session,
          slave: slave,
          project: project,
          syncApi: api,
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.textContaining('Slave offline'), findsWidgets);
    final syncIcon = find.byIcon(Icons.sync);
    expect(syncIcon, findsOneWidget);
    final btn = tester.widget<IconButton>(
      find.ancestor(of: syncIcon, matching: find.byType(IconButton)),
    );
    expect(btn.onPressed, isNull);
  });

  testWidgets('project page sync shows snapshot and Continue', (tester) async {
    var postCount = 0;
    var getCount = 0;
    final api = ProjectSyncApi(
      session: _session,
      get: (uri, {headers}) async {
        getCount++;
        // Initial load + syncAndWait baseline: no snapshot yet.
        if (getCount <= 2) {
          return http.Response('{"error":"not found"}', 404);
        }
        return http.Response(jsonEncode(_snapJson()), 200);
      },
      post: (uri, {headers, body}) async {
        postCount++;
        return http.Response(
          jsonEncode({'requestId': 'req_ui', 'status': 'accepted'}),
          202,
        );
      },
    );
    final slave = _slave(online: true);
    final project = slave.effectiveProjects.first;

    await tester.pumpWidget(
      MaterialApp(
        home: MilestoneListPage(
          session: _session,
          slave: slave,
          project: project,
          syncApi: api,
          pollAfterSync: const Duration(milliseconds: 1),
          syncTimeout: const Duration(seconds: 3),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.textContaining('Not synced yet'), findsOneWidget);

    await tester.tap(find.byIcon(Icons.sync));
    await tester.pump();
    // Allow syncAndWait poll loops.
    for (var i = 0; i < 20; i++) {
      await tester.pump(const Duration(milliseconds: 20));
    }
    await tester.pumpAndSettle();

    expect(postCount, 1);
    expect(find.textContaining('branch: main'), findsOneWidget);
    expect(find.textContaining('Local progress ahead'), findsOneWidget);
    expect(find.text('Continue'), findsOneWidget);
    expect(find.textContaining('M07'), findsWidgets);
  });
}
