import 'package:cloud_agent_app/app.dart';
import 'package:cloud_agent_app/auth/auth_controller.dart';
import 'package:cloud_agent_app/auth/gateway_api.dart';
import 'package:cloud_agent_app/auth/session.dart';
import 'package:cloud_agent_app/auth/session_store.dart';
import 'package:cloud_agent_app/ui/home_shell.dart';
import 'package:cloud_agent_app/ui/pair_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;

void main() {
  testWidgets('signed out shows pair page, not home', (tester) async {
    final auth = AuthController(store: MemorySessionStore());
    await auth.bootstrap();
    await tester.pumpWidget(CloudAgentApp(auth: auth));
    await tester.pumpAndSettle();

    expect(find.byType(PairPage), findsOneWidget);
    expect(find.byType(HomeShell), findsNothing);
    expect(find.text('Pair & continue'), findsOneWidget);
  });

  testWidgets('restored session opens home shell', (tester) async {
    final store = MemorySessionStore();
    await store.save(
      const Session(
        gatewayBaseUrl: 'http://127.0.0.1:8080',
        token: 'tok_restored_abcdefgh',
      ),
    );
    final auth = AuthController(store: store);
    await auth.bootstrap();
    await tester.pumpWidget(CloudAgentApp(auth: auth));
    await tester.pumpAndSettle();

    expect(find.byType(HomeShell), findsOneWidget);
    expect(find.byType(PairPage), findsNothing);
    expect(find.textContaining('tok_restored'), findsNothing);
    expect(find.text('Slaves'), findsOneWidget);
  });

  testWidgets('successful pair navigates to home', (tester) async {
    final auth = AuthController(
      store: MemorySessionStore(),
      api: GatewayApi(
        post: (uri, {headers, body}) async => http.Response(
          '{"token":"tok_new_pair_zzzzzzzz","expiresAt":"2026-10-01T00:00:00Z"}',
          200,
        ),
      ),
    );
    await auth.bootstrap();
    await tester.pumpWidget(CloudAgentApp(auth: auth));
    await tester.pumpAndSettle();

    final fields = find.byType(TextField);
    await tester.enterText(fields.at(0), 'http://127.0.0.1:8080');
    await tester.enterText(fields.at(1), 'ABCD-EFGH');
    await tester.tap(find.text('Pair & continue'));
    await tester.pumpAndSettle();

    expect(find.byType(HomeShell), findsOneWidget);
    expect(find.byType(PairPage), findsNothing);
  });

  testWidgets('sign out returns to pair page', (tester) async {
    final store = MemorySessionStore();
    await store.save(
      const Session(
        gatewayBaseUrl: 'http://127.0.0.1:8080',
        token: 'tok_out_abcdefghij',
      ),
    );
    final auth = AuthController(store: store);
    await auth.bootstrap();
    await tester.pumpWidget(CloudAgentApp(auth: auth));
    await tester.pumpAndSettle();

    await tester.tap(find.byTooltip('Sign out'));
    await tester.pumpAndSettle();

    expect(find.byType(PairPage), findsOneWidget);
    expect(find.byType(HomeShell), findsNothing);
  });
}
