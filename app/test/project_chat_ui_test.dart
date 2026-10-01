import 'dart:convert';

import 'package:cloud_agent_app/auth/session.dart';
import 'package:cloud_agent_app/chat/chat_api.dart';
import 'package:cloud_agent_app/chat/models.dart';
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

SlaveInfo _slave({bool online = true, bool withPlans = false}) =>
    SlaveInfo.fromJson({
      'id': 'slave_devpc',
      'name': 'Dev PC',
      'online': online,
      'projects': [
        {
          'id': 'r_cloud_agent',
          'name': 'cloud-agent',
          'cwd': 'E:/workspace/cloud-agent',
          'milestones': withPlans
              ? [
                  {
                    'id': 'M11',
                    'title': 'Slave Master',
                    'phases': [
                      {
                        'id': 'M11-P06',
                        'title': '文档硬化',
                        'phaseRef':
                            'doc/roadmaps/cloud-agent/phases/M11-P06-docs-harden.md',
                        'dependsOn': <String>[],
                      },
                    ],
                  },
                ]
              : <Map<String, dynamic>>[],
        },
      ],
    });

void main() {
  test('ChatApi create, listModels, rename and sendMessage', () async {
    var posts = 0;
    var gets = 0;
    var patches = 0;
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
      patch: (uri, {headers, body}) async {
        patches++;
        expect(uri.path, '/v1/chats/chat_1');
        final map = jsonDecode(body as String) as Map;
        expect(map['title'], 'My title');
        return http.Response(
          jsonEncode({
            'id': 'chat_1',
            'slaveId': 'slave_devpc',
            'repoId': 'r_cloud_agent',
            'mode': 'agent',
            'model': 'auto',
            'title': 'My title',
            'status': 'idle',
            'messages': <dynamic>[],
          }),
          200,
        );
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

    final renamed = await api.renameChat(sess.id, 'My title');
    expect(renamed.title, 'My title');
    expect(renamed.displayTitle, 'My title');

    final send = await api.sendMessage(
      chatId: sess.id,
      text: 'hello',
      mode: 'ask',
      model: 'composer-2.5',
    );
    expect(send.taskId, 'tsk_1');
    expect(posts, 2);
    expect(gets, 1);
    expect(patches, 1);
  });

  test('ChatApi sendMessage includes refs', () async {
    Map? sentBody;
    final api = ChatApi(
      session: _session,
      post: (uri, {headers, body}) async {
        expect(uri.path.endsWith('/messages'), isTrue);
        sentBody = jsonDecode(body as String) as Map;
        return http.Response(
          jsonEncode({
            'taskId': 'tsk_ref',
            'chatId': 'chat_1',
            'mode': 'agent',
            'model': 'auto',
          }),
          202,
        );
      },
    );
    await api.sendMessage(
      chatId: 'chat_1',
      text: 'fix DoD',
      mode: 'agent',
      refs: const [
        ChatRef(
          kind: 'phase',
          id: 'M11-P06',
          title: '文档硬化',
          milestoneId: 'M11',
          phaseRef: 'doc/roadmaps/cloud-agent/phases/M11-P06-docs-harden.md',
        ),
      ],
    );
    expect(sentBody?['text'], 'fix DoD');
    final refs = sentBody?['refs'] as List?;
    expect(refs, isNotNull);
    expect(refs!.length, 1);
    expect((refs.first as Map)['id'], 'M11-P06');
    expect((refs.first as Map)['phaseRef'], contains('M11-P06'));
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

  testWidgets('chat page lazy-creates on first send', (tester) async {
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
        if (uri.path == '/v1/chats') {
          return http.Response(jsonEncode({'chats': <dynamic>[]}), 200);
        }
        if (uri.path.endsWith('/chat_ui')) {
          return http.Response(
            jsonEncode({
              'id': 'chat_ui',
              'slaveId': 'slave_devpc',
              'repoId': 'r_cloud_agent',
              'mode': 'ask',
              'model': 'composer-2.5',
              'title': 'add loading',
              'status': 'idle',
              'messages': <dynamic>[],
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
    expect(created, isFalse);
    expect(find.textContaining('New chat'), findsWidgets);
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

    expect(created, isTrue);
    expect(find.text('add loading'), findsWidgets);
    expect(find.text('You'), findsOneWidget);
  });

  testWidgets('opens latest chat and history shows title', (tester) async {
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
              ],
            }),
            200,
          );
        }
        if (uri.path == '/v1/chats') {
          return http.Response(
            jsonEncode({
              'chats': [
                {
                  'id': 'chat_old',
                  'slaveId': 'slave_devpc',
                  'repoId': 'r_cloud_agent',
                  'mode': 'ask',
                  'model': 'auto',
                  'title': 'Earlier topic',
                  'preview': 'earlier question',
                  'messageCount': 2,
                  'messages': <dynamic>[],
                  'updatedAt': '2026-09-29T01:00:00Z',
                },
              ],
            }),
            200,
          );
        }
        if (uri.path.endsWith('/chat_old')) {
          return http.Response(
            jsonEncode({
              'id': 'chat_old',
              'slaveId': 'slave_devpc',
              'repoId': 'r_cloud_agent',
              'mode': 'ask',
              'model': 'auto',
              'title': 'Earlier topic',
              'status': 'idle',
              'messages': [
                {
                  'id': 'm1',
                  'role': 'user',
                  'content': 'earlier question',
                  'at': '2026-09-29T01:00:00Z',
                },
                {
                  'id': 'm2',
                  'role': 'assistant',
                  'content': 'earlier answer',
                  'at': '2026-09-29T01:00:01Z',
                },
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
              'id': 'chat_new',
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
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(created, isFalse);
    expect(find.text('earlier answer'), findsOneWidget);
    expect(find.textContaining('Earlier topic'), findsOneWidget);

    await tester.tap(find.byTooltip('History'));
    await tester.pumpAndSettle();
    expect(find.text('Earlier topic'), findsWidgets);
  });

  testWidgets('reopen running chat reattaches stream', (tester) async {
    final slave = _slave();
    final project = slave.effectiveProjects.first;
    var wsAttached = false;
    final api = ChatApi(
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
        if (uri.path == '/v1/chats') {
          return http.Response(
            jsonEncode({
              'chats': [
                {
                  'id': 'chat_run',
                  'slaveId': 'slave_devpc',
                  'repoId': 'r_cloud_agent',
                  'mode': 'agent',
                  'model': 'auto',
                  'title': 'In flight',
                  'status': 'running',
                  'preview': 'keep going',
                  'messageCount': 1,
                  'updatedAt': '2026-09-29T02:00:00Z',
                },
              ],
            }),
            200,
          );
        }
        if (uri.path.endsWith('/chat_run')) {
          return http.Response(
            jsonEncode({
              'id': 'chat_run',
              'slaveId': 'slave_devpc',
              'repoId': 'r_cloud_agent',
              'mode': 'agent',
              'model': 'auto',
              'title': 'In flight',
              'status': 'running',
              'messages': [
                {
                  'id': 'm1',
                  'role': 'user',
                  'content': 'keep going',
                  'taskId': 'tsk_inflight',
                  'at': '2026-09-29T02:00:00Z',
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
        home: ProjectChatPage(
          session: _session,
          slave: slave,
          project: project,
          api: api,
          wsFactory: (buf, taskId) {
            wsAttached = true;
            expect(taskId, 'tsk_inflight');
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
    // Running reattach shows phase label + spinner (infinite animation → no pumpAndSettle).
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 50));

    expect(find.text('keep going'), findsOneWidget);
    expect(find.text('处理中'), findsOneWidget);
    expect(find.byType(CircularProgressIndicator), findsWidgets);
    expect(wsAttached, isTrue);
  });

  testWidgets('plus panel cites plan phase and sends refs', (tester) async {
    final slave = _slave(withPlans: true);
    final project = slave.effectiveProjects.first;
    Map? sentBody;
    final api = ChatApi(
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
        if (uri.path == '/v1/chats') {
          return http.Response(jsonEncode({'chats': <dynamic>[]}), 200);
        }
        return http.Response('{}', 404);
      },
      post: (uri, {headers, body}) async {
        if (uri.path == '/v1/chats') {
          return http.Response(
            jsonEncode({
              'id': 'chat_cite',
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
          sentBody = jsonDecode(body as String) as Map;
          return http.Response(
            jsonEncode({
              'taskId': 'tsk_cite',
              'chatId': 'chat_cite',
              'mode': 'agent',
              'model': 'auto',
            }),
            202,
          );
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

    await tester.tap(find.byTooltip('Add'));
    await tester.pumpAndSettle();
    expect(find.text('Cite a milestone phase'), findsOneWidget);

    await tester.tap(find.text('Cite a milestone phase'));
    await tester.pumpAndSettle();
    expect(find.textContaining('M11-P06'), findsWidgets);

    await tester.tap(find.text('M11-P06 · 文档硬化'));
    await tester.pumpAndSettle();
    expect(find.text('M11-P06 文档硬化'), findsOneWidget);

    await tester.enterText(find.byType(TextField), 'fix this plan DoD');
    await tester.tap(find.byTooltip('Send'));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 50));

    expect(sentBody, isNotNull);
    final refs = sentBody!['refs'] as List?;
    expect(refs, isNotNull);
    expect((refs!.first as Map)['id'], 'M11-P06');
    expect(find.text('fix this plan DoD'), findsWidgets);
    expect(find.text('M11-P06 文档硬化'), findsWidgets);

    // Flush delayed title-refresh timers from first send.
    await tester.pump(const Duration(seconds: 9));
  });
}
