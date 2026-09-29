/// Models for GET /v1/slaves (projects + milestones).
class MilestonePhaseInfo {
  const MilestonePhaseInfo({
    required this.id,
    required this.title,
    required this.phaseRef,
    this.dependsOn = const [],
    this.model,
    this.onFailure,
    this.prompt,
  });

  final String id;
  final String title;
  final String phaseRef;
  final List<String> dependsOn;
  final String? model;
  final String? onFailure;
  final Map<String, dynamic>? prompt;

  factory MilestonePhaseInfo.fromJson(Map<String, dynamic> json) {
    final deps = json['dependsOn'];
    Map<String, dynamic>? prompt;
    final rawPrompt = json['prompt'];
    if (rawPrompt is Map) {
      prompt = Map<String, dynamic>.from(rawPrompt);
    }
    return MilestonePhaseInfo(
      id: json['id'] as String? ?? '',
      title: json['title'] as String? ?? '',
      phaseRef: json['phaseRef'] as String? ?? '',
      dependsOn: deps is List
          ? deps.map((e) => e.toString()).toList(growable: false)
          : const [],
      model: json['model'] as String?,
      onFailure: json['onFailure'] as String?,
      prompt: prompt,
    );
  }

  Map<String, dynamic> toWorkflowNodeJson() {
    return {
      'id': id,
      'title': title,
      'phaseRef': phaseRef,
      'dependsOn': dependsOn,
      if (model != null && model!.isNotEmpty) 'model': model,
      if (onFailure != null && onFailure!.isNotEmpty) 'onFailure': onFailure,
      'prompt': prompt ?? {'mode': 'phase_file'},
    };
  }
}

class MilestoneInfo {
  const MilestoneInfo({
    required this.id,
    required this.title,
    this.progressDoc,
    this.phases = const [],
  });

  final String id;
  final String title;
  final String? progressDoc;
  final List<MilestonePhaseInfo> phases;

  factory MilestoneInfo.fromJson(Map<String, dynamic> json) {
    final raw = json['phases'];
    final phases = <MilestonePhaseInfo>[];
    if (raw is List) {
      for (final p in raw) {
        if (p is Map) {
          phases.add(MilestonePhaseInfo.fromJson(Map<String, dynamic>.from(p)));
        }
      }
    }
    return MilestoneInfo(
      id: json['id'] as String? ?? '',
      title: json['title'] as String? ?? '',
      progressDoc: json['progressDoc'] as String?,
      phases: phases,
    );
  }
}

class ProjectInfo {
  const ProjectInfo({
    required this.id,
    required this.name,
    required this.cwd,
    this.index,
    this.milestones = const [],
  });

  final String id;
  final String name;
  final String cwd;
  final String? index;
  final List<MilestoneInfo> milestones;

  factory ProjectInfo.fromJson(Map<String, dynamic> json) {
    final raw = json['milestones'];
    final milestones = <MilestoneInfo>[];
    if (raw is List) {
      for (final m in raw) {
        if (m is Map) {
          milestones.add(MilestoneInfo.fromJson(Map<String, dynamic>.from(m)));
        }
      }
    }
    return ProjectInfo(
      id: json['id'] as String? ?? '',
      name: json['name'] as String? ?? '',
      cwd: json['cwd'] as String? ?? '',
      index: json['index'] as String?,
      milestones: milestones,
    );
  }
}

class SlaveInfo {
  const SlaveInfo({
    required this.id,
    required this.name,
    required this.online,
    this.repos = const [],
    this.projects = const [],
  });

  final String id;
  final String name;
  final bool online;
  final List<ProjectInfo> repos;
  final List<ProjectInfo> projects;

  /// Prefer projects; fall back to legacy repos.
  List<ProjectInfo> get effectiveProjects =>
      projects.isNotEmpty ? projects : repos;

  factory SlaveInfo.fromJson(Map<String, dynamic> json) {
    ProjectInfo mapRepo(Map m) {
      final map = Map<String, dynamic>.from(m);
      // legacy repos have no milestones
      return ProjectInfo.fromJson(map);
    }

    final projects = <ProjectInfo>[];
    final rawProjects = json['projects'];
    if (rawProjects is List) {
      for (final p in rawProjects) {
        if (p is Map) projects.add(ProjectInfo.fromJson(Map<String, dynamic>.from(p)));
      }
    }

    final repos = <ProjectInfo>[];
    final rawRepos = json['repos'];
    if (rawRepos is List) {
      for (final r in rawRepos) {
        if (r is Map) repos.add(mapRepo(r));
      }
    }

    return SlaveInfo(
      id: json['id'] as String? ?? '',
      name: json['name'] as String? ?? '',
      online: json['online'] == true,
      repos: repos,
      projects: projects,
    );
  }
}
