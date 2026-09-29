import 'dart:convert';

import 'package:cloud_agent_app/auth/session.dart';
import 'package:cloud_agent_app/chat/chat_api.dart';
import 'package:cloud_agent_app/slaves/models.dart';
import 'package:cloud_agent_app/sync/project_sync_api.dart';
import 'package:cloud_agent_app/ui/milestone_list_page.dart';
import 'package:cloud_agent_app/ui/project_chat_page.dart';
import 'package:cloud_agent_app/ws/app_ws_client.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;

const _session = Session(
  gatewayBaseUrl: 'http://127.0.0.1:8080',
  token: 'tok_chat_test_xxxx',
);

SlaveInfo _slave({bool online = true}) => SlaveInfo.fromJson({
      'id': 'slave_devpc',
      'name': 'Dev PC',
      'online': online,
      'projects': [
        {
          'id': 'r_cloud_agent',
          'name': 'cloud-agent',
          'cwd': 'E:/workspace/cloud-agent',
          'milestones': <Map<String, dynamic>>[],
        },
      ],
    });

void main() {
  test('ChatApi create, listModels and sendMessage', () async {
    var posts = 0;
    var gets = 0;
    final api = ChatApi(
      session: _session,
      get: (uri, {headers}) async {
        gets++;
        expect(headers?['Authorization'], 'Bearer tok_chat_test_xxxx');
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
      post: (uri, {headers, body}) async {
        posts++;
        expect(headers?['Authorization'], 'Bearer tok_chat_test_xxxx');
        if (uri.path == '/v1/chats') {
          return http.Response(
            jsonEncode({
              'id': 'chat_1',
              'slaveId': 'slave_devpc',
              'repoId': 'r_cloud_agent',
              'mode': 'agent',
              'model': 'auto',
              'status': 'idle',
              'messages': <dynamic>[],
              'createdAt': '2026-09-29T00:00:00Z',
              'updatedAt': '2026-09-29T00:00:00Z',
            }),
            201,
          );
        }
        if (uri.path.endsWith('/messages')) {
          final map = jsonDecode(body as String) as Map;
          expect(map['text'], 'hello');
          expect(map['mode'], 'ask');
          expect(map['model'], 'composer-2.5');
          return http.Response(
            jsonEncode({
              'taskId': 'tsk_1',
              'chatId': 'chat_1',
              'mode': 'ask',
              'model': 'composer-2.5',
            }),
            202,
          );
        }
        return http.Response('{}', 404);
      },
    );

    final catalog = await api.listModels();
    expect(catalog.models.length, 2);
    expect(catalog.defaultId, 'auto');

    final sess = await api.createChat(
      slaveId: 'slave_devpc',
      repoId: 'r_cloud_agent',
    );
    expect(sess.id, 'chat_1');
    expect(sess.model, 'auto');

    final send = await api.sendMessage(
      chatId: sess.id,
      text: 'hello',
      mode: 'ask',
      model: 'composer-2.5',
    );
    expect(send.taskId, 'tsk_1');
    expect(posts, 2);
    expect(gets, 1);
  });

  testWidgets('project page has Chat entry', (tester) async {
    final slave = _slave();
    final project = slave.effectiveProjects.first;
    final syncApi = ProjectSyncApi(
      session: _session,
      get: (uri, {headers}) async =>
          http.Response(jsonEncode({'error': 'not found'}), 404),
      post: (uri, {headers, body}) async =>
          http.Response(jsonEncode({'error': 'offline'}), 409),
    );

    await tester.pumpWidget(
      MaterialApp(
        home: MilestoneListPage(
          session: _session,
          slave: slave,
          project: project,
          syncApi: syncApi,
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Chat'), findsOneWidget);
    expect(find.byTooltip('Project chat'), findsOneWidget);
  });

  testWidgets('chat page send shows user bubble', (tester) async {
    final slave = _slave();
    final project = slave.effectiveProjects.first;
    var created = false;
    final api = ChatApi(
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
      post: (uri, {headers, body}) async {
        if (uri.path == '/v1/chats') {
          created = true;
          return http.Response(
            jsonEncode({
              'id': 'chat_ui',
              'slaveId': 'slave_devpc',
              'repoId': 'r_cloud_agent',
              'mode': 'agent',
              'model': 'auto',
              'status': 'idle',
              'messages': <dynamic>[],
            }),
            201,
          );
        }
        if (uri.path.endsWith('/messages')) {
          final map = jsonDecode(body as String) as Map;
          expect(map['mode'], 'ask');
          expect(map['model'], 'composer-2.5');
          return http.Response(
            jsonEncode({
              'taskId': 'tsk_ui',
              'chatId': 'chat_ui',
              'mode': 'ask',
              'model': 'composer-2.5',
            }),
            202,
          );
        }
        if (uri.path.endsWith('/stop') || uri.path.contains('/cancel')) {
          return http.Response(jsonEncode({'status': 'cancelled'}), 200);
        }
        return http.Response('{}', 404);
      },
    );

    await tester.pumpWidget(
      MaterialApp(
        home: ProjectChatPage(
          session: _session,
          slave: slave,
          project: project,
          api: api,
          wsFactory: (buf, taskId) {
            // No-op WS: return client that never connects.
            return AppWsClient(
              session: _session,
              buffer: buf,
              taskId: taskId,
              connect: (_) => throw StateError('no ws in test'),
              reconnectDelay: const Duration(days: 1),
            );
          },
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(created, isTrue);
    expect(find.text('Agent'), findsWidgets);
    expect(find.text('Ask'), findsOneWidget);
    expect(find.text('Plan'), findsOneWidget);
    expect(find.text('Auto'), findsWidgets);

    await tester.tap(find.text('Ask'));
    await tester.pumpAndSettle();

    await tester.tap(find.byType(DropdownButton<String>));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Composer 2.5').last);
    await tester.pumpAndSettle();

    await tester.enterText(find.byType(TextField), 'add loading');
    await tester.tap(find.byTooltip('Send'));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 50));
    await tester.pumpAndSettle();

    expect(find.text('add loading'), findsOneWidget);
    expect(find.text('You'), findsOneWidget);
  });
}
