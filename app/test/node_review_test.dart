import 'dart:convert';

import 'package:cloud_agent_app/auth/session.dart';
import 'package:cloud_agent_app/ui/node_review_page.dart';
import 'package:cloud_agent_app/workflow/models.dart';
import 'package:cloud_agent_app/workflow/workflow_api.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;

void main() {
  const session = Session(
    gatewayBaseUrl: 'http://127.0.0.1:8080',
    token: 'tok_rev',
  );

  final node = WorkflowNode.fromJson({
    'id': 'N1',
    'title': 'Phase',
    'status': 'awaiting_review',
    'dependsOn': <String>[],
    'taskId': 'tsk_1',
  });

  testWidgets('approve enabled when awaiting_review', (tester) async {
    var reviewed = false;
    final api = WorkflowApi(
      session: session,
      post: (uri, {headers, body}) async {
        reviewed = true;
        expect(uri.path, contains('/review'));
        final map = jsonDecode(body as String) as Map<String, dynamic>;
        expect(map['decision'], 'approve');
        return http.Response(
          jsonEncode({
            'decision': 'approve',
            'workflow': {
              'id': 'wf_1',
              'bundleId': 'b',
              'status': 'running',
              'nodes': [
                {'id': 'N1', 'status': 'approved', 'dependsOn': <String>[]},
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
        home: NodeReviewPage(
          session: session,
          workflowId: 'wf_1',
          node: node,
          api: api,
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('通过 (approve)'));
    await tester.pumpAndSettle();
    expect(reviewed, isTrue);
  });

  testWidgets('revise requires non-empty instruction then posts', (tester) async {
    var revised = false;
    final api = WorkflowApi(
      session: session,
      post: (uri, {headers, body}) async {
        revised = true;
        expect(uri.path, contains('/revise'));
        return http.Response(
          jsonEncode({
            'workflow': {
              'id': 'wf_1',
              'bundleId': 'b',
              'status': 'running',
              'nodes': [
                {
                  'id': 'N1',
                  'status': 'running',
                  'taskId': 'tsk_2',
                  'dependsOn': <String>[],
                },
              ],
              'createdAt': '2026-09-28T00:00:00Z',
            },
          }),
          200,
        );
      },
      get: (uri, {headers}) async => http.Response(
            jsonEncode({'events': [], 'latestSeq': 0}),
            200,
          ),
    );

    await tester.pumpWidget(
      MaterialApp(
        home: NodeReviewPage(
          session: session,
          workflowId: 'wf_1',
          node: node,
          api: api,
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('提交修改意见'));
    await tester.pumpAndSettle();
    expect(revised, isFalse);

    await tester.enterText(find.byType(TextField), 'fix the codes');
    await tester.pump();
    await tester.tap(find.text('提交修改意见'));
    await tester.pumpAndSettle();
    expect(revised, isTrue);
  });

  testWidgets('approve disabled when not awaiting_review', (tester) async {
    final running = WorkflowNode.fromJson({
      'id': 'N1',
      'status': 'running',
      'dependsOn': <String>[],
    });
    await tester.pumpWidget(
      MaterialApp(
        home: NodeReviewPage(
          session: session,
          workflowId: 'wf_1',
          node: running,
          api: WorkflowApi(session: session),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.textContaining('仅 awaiting_review 可审核'), findsOneWidget);
    // Tap approve should no-op (button disabled).
    await tester.tap(find.text('通过 (approve)'));
    await tester.pumpAndSettle();
  });
}
