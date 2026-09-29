import 'package:flutter/material.dart';

import '../auth/session.dart';
import '../slaves/models.dart';
import 'milestone_list_page.dart';

/// Step 2: 工程列表 → Milestone 列表.
class ProjectListPage extends StatelessWidget {
  const ProjectListPage({
    super.key,
    required this.session,
    required this.slave,
  });

  final Session session;
  final SlaveInfo slave;

  @override
  Widget build(BuildContext context) {
    final projects = slave.effectiveProjects;
    return Scaffold(
      appBar: AppBar(
        title: Text(slave.name.isNotEmpty ? slave.name : slave.id),
      ),
      body: projects.isEmpty
          ? const Center(
              child: Padding(
                padding: EdgeInsets.all(24),
                child: Text('No projects (check Slave projects[] / repos[]).'),
              ),
            )
          : ListView.builder(
              itemCount: projects.length,
              itemBuilder: (context, i) {
                final p = projects[i];
                return ListTile(
                  leading: const Icon(Icons.folder_outlined),
                  title: Text(p.name.isNotEmpty ? p.name : p.id),
                  subtitle: Text(
                    '${p.id}'
                    '${p.index != null ? ' · ${p.index}' : ''}'
                    ' · ${p.milestones.length} milestone'
                    '${p.milestones.length == 1 ? '' : 's'}',
                  ),
                  trailing: const Icon(Icons.chevron_right),
                  onTap: () {
                    Navigator.of(context).push<void>(
                      MaterialPageRoute(
                        builder: (_) => MilestoneListPage(
                          session: session,
                          slave: slave,
                          project: p,
                        ),
                      ),
                    );
                  },
                );
              },
            ),
    );
  }
}
