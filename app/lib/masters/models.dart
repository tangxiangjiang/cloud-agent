import '../slaves/models.dart';

/// One Master-managed Slave mirrored from GET /v1/masters*.
class FleetSlave {
  const FleetSlave({
    required this.id,
    required this.name,
    required this.enabled,
    required this.process,
    required this.gatewayOnline,
    required this.project,
    this.lastError,
    this.pid,
  });

  final String id;
  final String name;
  final bool enabled;
  final String process;
  final bool gatewayOnline;
  final ProjectInfo project;
  final String? lastError;
  final int? pid;

  bool get canEnterProject => process == 'running' && gatewayOnline;

  factory FleetSlave.fromJson(Map<String, dynamic> json) {
    final rawProject = json['project'];
    final project = rawProject is Map
        ? ProjectInfo.fromJson(Map<String, dynamic>.from(rawProject))
        : const ProjectInfo(id: '', name: '', cwd: '');
    return FleetSlave(
      id: json['id'] as String? ?? '',
      name: json['name'] as String? ?? '',
      enabled: json['enabled'] != false,
      process: json['process'] as String? ?? 'stopped',
      gatewayOnline: json['gatewayOnline'] == true,
      project: project,
      lastError: json['lastError'] as String?,
      pid: json['pid'] is int ? json['pid'] as int : null,
    );
  }

  /// Build data-plane SlaveInfo for existing project/milestone pages.
  SlaveInfo toSlaveInfo({SlaveInfo? enrichFrom}) {
    ProjectInfo project = this.project;
    if (enrichFrom != null) {
      for (final p in enrichFrom.effectiveProjects) {
        if (p.id == this.project.id) {
          project = p;
          break;
        }
      }
    }
    return SlaveInfo(
      id: id,
      name: name.isNotEmpty ? name : id,
      online: gatewayOnline,
      projects: [project],
      repos: [project],
    );
  }
}

class MasterInfo {
  const MasterInfo({
    required this.masterId,
    required this.name,
    required this.online,
    this.slaves = const [],
    this.updatedAt,
  });

  final String masterId;
  final String name;
  final bool online;
  final List<FleetSlave> slaves;
  final String? updatedAt;

  factory MasterInfo.fromJson(Map<String, dynamic> json) {
    final raw = json['slaves'];
    final slaves = <FleetSlave>[];
    if (raw is List) {
      for (final s in raw) {
        if (s is Map) {
          slaves.add(FleetSlave.fromJson(Map<String, dynamic>.from(s)));
        }
      }
    }
    return MasterInfo(
      masterId: json['masterId'] as String? ?? '',
      name: json['name'] as String? ?? '',
      online: json['online'] == true,
      slaves: slaves,
      updatedAt: json['updatedAt'] as String?,
    );
  }
}
