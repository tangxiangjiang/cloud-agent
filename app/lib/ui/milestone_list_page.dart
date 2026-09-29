import 'package:flutter/material.dart';

import '../auth/session.dart';
import '../slaves/models.dart';
import 'milestone_phases_page.dart';

/// Step 3: Milestone 列表 → 执行 plan（phases）.
class MilestoneListPage extends StatelessWidget {
  const MilestoneListPage({
    super.key,
    required this.session,
    required this.slave,
    required this.project,
  });

  final Session session;
  final SlaveInfo slave;
  final ProjectInfo project;

  @override
  Widget build(BuildContext context) {
    final milestones = project.milestones;
    return Scaffold(
      appBar: AppBar(
        title: Text(project.name.isNotEmpty ? project.name : project.id),
      ),
      body: milestones.isEmpty
          ? Center(
              child: Padding(
                padding: const EdgeInsets.all(24),
                child: Text(
                  project.index == null
                      ? 'No milestone index (set projects[].index).'
                      : 'No milestones in ${project.index}.',
                ),
              ),
            )
          : ListView.builder(
              itemCount: milestones.length,
              itemBuilder: (context, i) {
                final m = milestones[i];
                return ListTile(
                  leading: const Icon(Icons.flag_outlined),
                  title: Text('${m.id} · ${m.title}'),
                  subtitle: Text(
                    '${m.phases.length} phase${m.phases.length == 1 ? '' : 's'}'
                    '${m.progressDoc != null ? ' · ${m.progressDoc}' : ''}',
                  ),
                  trailing: const Icon(Icons.chevron_right),
                  onTap: () {
                    Navigator.of(context).push<void>(
                      MaterialPageRoute(
                        builder: (_) => MilestonePhasesPage(
                          session: session,
                          slave: slave,
                          project: project,
                          milestone: m,
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
