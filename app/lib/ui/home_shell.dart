import 'package:flutter/material.dart';

import '../auth/auth_controller.dart';
import 'slave_list_page.dart';

/// Logged-in home: Slave → 工程 → Milestone → 执行 plan.
class HomeShell extends StatelessWidget {
  const HomeShell({super.key, required this.auth});

  final AuthController auth;

  @override
  Widget build(BuildContext context) {
    final session = auth.session!;
    return SlaveListPage(
      session: session,
      onSignOut: auth.signOut,
    );
  }
}
