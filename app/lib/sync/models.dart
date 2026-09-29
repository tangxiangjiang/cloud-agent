/// Models for project sync GET/POST (M08).
class SyncWarning {
  const SyncWarning({
    required this.code,
    required this.message,
    this.phaseId,
    this.workflowId,
    this.suggestion,
  });

  final String code;
  final String message;
  final String? phaseId;
  final String? workflowId;
  final String? suggestion;

  bool get canContinueWorkflow =>
      workflowId != null &&
      workflowId!.isNotEmpty &&
      (suggestion == 'continue_workflow' ||
          suggestion == 'resume_or_create' ||
          code == 'progress_ahead' ||
          code == 'no_active_workflow');

  factory SyncWarning.fromJson(Map<String, dynamic> json) {
    return SyncWarning(
      code: json['code'] as String? ?? '',
      message: json['message'] as String? ?? json['code']?.toString() ?? '',
      phaseId: json['phaseId'] as String?,
      workflowId: json['workflowId'] as String?,
      suggestion: json['suggestion'] as String?,
    );
  }

  /// Accepts structured map or plain string from older responses.
  static SyncWarning parse(dynamic raw) {
    if (raw is String) {
      return SyncWarning(code: 'info', message: raw);
    }
    if (raw is Map) {
      return SyncWarning.fromJson(Map<String, dynamic>.from(raw));
    }
    return SyncWarning(code: 'info', message: raw.toString());
  }
}

class SyncPhaseRow {
  const SyncPhaseRow({
    required this.id,
    required this.progressStatus,
    this.title,
    this.nodeStatus,
    this.workflowId,
  });

  final String id;
  final String progressStatus;
  final String? title;
  final String? nodeStatus;
  final String? workflowId;

  factory SyncPhaseRow.fromJson(Map<String, dynamic> json) {
    return SyncPhaseRow(
      id: json['id'] as String? ?? '',
      progressStatus: json['progressStatus'] as String? ?? 'unknown',
      title: json['title'] as String?,
      nodeStatus: json['nodeStatus'] as String?,
      workflowId: json['workflowId'] as String?,
    );
  }
}

class SyncReport {
  const SyncReport({
    this.workflowId,
    this.bundleId,
    this.workflowStatus,
    this.active = false,
    this.branch,
    this.dirty,
    this.phases = const [],
  });

  final String? workflowId;
  final String? bundleId;
  final String? workflowStatus;
  final bool active;
  final String? branch;
  final bool? dirty;
  final List<SyncPhaseRow> phases;

  factory SyncReport.fromJson(Map<String, dynamic> json) {
    final rawPhases = json['phases'];
    final phases = <SyncPhaseRow>[];
    if (rawPhases is List) {
      for (final p in rawPhases) {
        if (p is Map) {
          phases.add(SyncPhaseRow.fromJson(Map<String, dynamic>.from(p)));
        }
      }
    }
    return SyncReport(
      workflowId: json['workflowId'] as String?,
      bundleId: json['bundleId'] as String?,
      workflowStatus: json['workflowStatus'] as String?,
      active: json['active'] == true,
      branch: json['branch'] as String?,
      dirty: json['dirty'] is bool ? json['dirty'] as bool : null,
      phases: phases,
    );
  }
}

class ProjectSyncSnapshot {
  const ProjectSyncSnapshot({
    required this.slaveId,
    required this.repoId,
    required this.syncedAt,
    this.payload = const {},
    this.warnings = const [],
    this.report,
  });

  final String slaveId;
  final String repoId;
  final String syncedAt;
  final Map<String, dynamic> payload;
  final List<SyncWarning> warnings;
  final SyncReport? report;

  String? get summary {
    final s = payload['summary'];
    return s is String ? s : null;
  }

  String? get branch {
    final b = payload['branch'] ?? report?.branch;
    return b is String ? b : null;
  }

  bool? get dirty {
    final d = payload['dirty'];
    if (d is bool) return d;
    return report?.dirty;
  }

  factory ProjectSyncSnapshot.fromJson(Map<String, dynamic> json) {
    Map<String, dynamic> payload = const {};
    final rawPayload = json['payload'];
    if (rawPayload is Map) {
      payload = Map<String, dynamic>.from(rawPayload);
    }

    final warnings = <SyncWarning>[];
    final rawWarnings = json['warnings'];
    if (rawWarnings is List) {
      for (final w in rawWarnings) {
        warnings.add(SyncWarning.parse(w));
      }
    }

    SyncReport? report;
    final rawReport = json['report'];
    if (rawReport is Map) {
      report = SyncReport.fromJson(Map<String, dynamic>.from(rawReport));
    }

    return ProjectSyncSnapshot(
      slaveId: json['slaveId'] as String? ?? '',
      repoId: json['repoId'] as String? ?? '',
      syncedAt: json['syncedAt'] as String? ?? '',
      payload: payload,
      warnings: warnings,
      report: report,
    );
  }
}

class SyncTriggerResult {
  const SyncTriggerResult({
    required this.requestId,
    required this.status,
    this.slaveId,
    this.repoId,
  });

  final String requestId;
  final String status;
  final String? slaveId;
  final String? repoId;

  factory SyncTriggerResult.fromJson(Map<String, dynamic> json) {
    return SyncTriggerResult(
      requestId: json['requestId'] as String? ?? '',
      status: json['status'] as String? ?? 'accepted',
      slaveId: json['slaveId'] as String?,
      repoId: json['repoId'] as String?,
    );
  }
}
