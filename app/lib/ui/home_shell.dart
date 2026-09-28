import 'package:flutter/material.dart';

import '../auth/auth_controller.dart';
import '../util/redact.dart';
import 'workflow_list_page.dart';

/// Logged-in shell: session summary + entry to workflows.
class HomeShell extends StatelessWidget {
  const HomeShell({super.key, required this.auth});

  final AuthController auth;

  @override
  Widget build(BuildContext context) {
    final session = auth.session!;
    return Scaffold(
      appBar: AppBar(
        title: const Text('cloud-agent'),
        actions: [
          IconButton(
            tooltip: 'Sign out',
            onPressed: () => auth.signOut(),
            icon: const Icon(Icons.logout),
          ),
        ],
      ),
      body: ListView(
        padding: const EdgeInsets.all(24),
        children: [
          Text(
            'Paired',
            style: Theme.of(context).textTheme.titleLarge,
          ),
          const SizedBox(height: 8),
          Text('Gateway: ${session.gatewayBaseUrl}'),
          Text('Token: ${redactSecret(session.token)}'),
          const SizedBox(height: 24),
          Card(
            child: ListTile(
              leading: const Icon(Icons.account_tree_outlined),
              title: const Text('Workflows'),
              subtitle: const Text('List, detail, start'),
              trailing: const Icon(Icons.chevron_right),
              onTap: () {
                Navigator.of(context).push<void>(
                  MaterialPageRoute(
                    builder: (_) => WorkflowListPage(session: session),
                  ),
                );
              },
            ),
          ),
        ],
      ),
    );
  }
}
