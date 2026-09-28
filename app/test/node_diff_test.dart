import 'dart:convert';

import 'package:cloud_agent_app/auth/session.dart';
import 'package:cloud_agent_app/ui/node_diff_page.dart';
import 'package:cloud_agent_app/workflow/workflow_api.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;

void main() {
  const session = Session(
    gatewayBaseUrl: 'http://127.0.0.1:8080',
    token: 'tok_diff',
  );

  testWidgets('diff page lists files and shows read-only patch', (tester) async {
    final api = WorkflowApi(
      session: session,
      get: (uri, {headers}) async {
        expect(uri.path, contains('/diff'));
        return http.Response(
          jsonEncode({
            'workflowId': 'wf_1',
            'nodeId': 'N1',
            'baseline': 'git:abc',
            'files': [
              {
                'path': 'a.txt',
                'status': 'modified',
                'additions': 1,
                'deletions': 1,
                'unifiedDiff': '--- a/a.txt\n+++ b/a.txt\n@@ -1 +1 @@\n-old\n+new\n',
              },
              {
                'path': 'b.txt',
                'status': 'added',
                'unifiedDiff': '+++ b/b.txt\n+hi\n',
              },
            ],
          }),
          200,
        );
      },
    );

    await tester.pumpWidget(
      MaterialApp(
        home: NodeDiffPage(
          session: session,
          workflowId: 'wf_1',
          nodeId: 'N1',
          api: api,
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('a.txt'), findsOneWidget);
    expect(find.text('b.txt'), findsOneWidget);
    expect(find.textContaining('+new'), findsOneWidget);
    // No editable TextField for file body / save.
    expect(find.byType(TextField), findsNothing);
    expect(find.text('Save'), findsNothing);
  });

  testWidgets('diff load error shows retry', (tester) async {
    final api = WorkflowApi(
      session: session,
      get: (uri, {headers}) async =>
          http.Response(jsonEncode({'error': 'diff not found'}), 404),
    );
    await tester.pumpWidget(
      MaterialApp(
        home: NodeDiffPage(
          session: session,
          workflowId: 'wf_1',
          nodeId: 'N1',
          api: api,
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.textContaining('diff not found'), findsOneWidget);
    expect(find.text('Retry'), findsOneWidget);
  });
}
