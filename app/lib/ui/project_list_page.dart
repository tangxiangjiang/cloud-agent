import 'package:flutter/material.dart';

import '../auth/session.dart';
import '../slaves/models.dart';
import 'milestone_list_page.dart';

/// Legacy multi-project picker.
///
/// M11 is slave↔project 1:1 — callers should open [MilestoneListPage] directly.
/// If only one project exists, this page immediately redirects.
class ProjectListPage extends StatefulWidget {
  const ProjectListPage({
    super.key,
    required this.session,
    required this.slave,
  });

  final Session session;
  final SlaveInfo slave;

  @override
  State<ProjectListPage> createState() => _ProjectListPageState();
}

class _ProjectListPageState extends State<ProjectListPage> {
  @override
  void initState() {
    super.initState();
    final projects = widget.slave.effectiveProjects;
    if (projects.length == 1) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (!mounted) return;
        Navigator.of(context).pushReplacement(
          MaterialPageRoute<void>(
            builder: (_) => MilestoneListPage(
              session: widget.session,
              slave: widget.slave,
              project: projects.first,
            ),
          ),
        );
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final projects = widget.slave.effectiveProjects;
    if (projects.length <= 1) {
      return const Scaffold(
        body: Center(child: CircularProgressIndicator()),
      );
    }
    return Scaffold(
      appBar: AppBar(
        title: Text(
          widget.slave.name.isNotEmpty ? widget.slave.name : widget.slave.id,
        ),
      ),
      body: Column(
        children: [
          if (!widget.slave.online)
            Material(
              color: Theme.of(context).colorScheme.errorContainer,
              child: const ListTile(
                dense: true,
                leading: Icon(Icons.cloud_off),
                title: Text('Slave offline — sync disabled on project page'),
              ),
            ),
          Material(
            color: Theme.of(context).colorScheme.surfaceContainerHighest,
            child: const ListTile(
              dense: true,
              leading: Icon(Icons.info_outline),
              title: Text('已废弃：请改用一 Slave 一工程（M11）'),
            ),
          ),
          Expanded(
            child: ListView.builder(
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
                          session: widget.session,
                          slave: widget.slave,
                          project: p,
                        ),
                      ),
                    );
                  },
                );
              },
            ),
          ),
        ],
      ),
    );
  }
}
