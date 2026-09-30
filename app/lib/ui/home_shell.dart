import 'package:flutter/material.dart';

import '../auth/auth_controller.dart';
import 'fleet_page.dart';
import 'project_entry_page.dart';

/// Logged-in home: 舰队（配置/启停）| 工程（仅 running∧gatewayOnline）.
class HomeShell extends StatefulWidget {
  const HomeShell({super.key, required this.auth});

  final AuthController auth;

  @override
  State<HomeShell> createState() => _HomeShellState();
}

class _HomeShellState extends State<HomeShell> {
  int _index = 0;

  @override
  Widget build(BuildContext context) {
    final session = widget.auth.session!;
    final pages = [
      FleetPage(session: session, onSignOut: widget.auth.signOut),
      ProjectEntryPage(session: session),
    ];
    return Scaffold(
      body: IndexedStack(index: _index, children: pages),
      bottomNavigationBar: NavigationBar(
        selectedIndex: _index,
        onDestinationSelected: (i) => setState(() => _index = i),
        destinations: const [
          NavigationDestination(
            icon: Icon(Icons.dns_outlined),
            selectedIcon: Icon(Icons.dns),
            label: '舰队',
          ),
          NavigationDestination(
            icon: Icon(Icons.folder_outlined),
            selectedIcon: Icon(Icons.folder),
            label: '工程',
          ),
        ],
      ),
    );
  }
}
