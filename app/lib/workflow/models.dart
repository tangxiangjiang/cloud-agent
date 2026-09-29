/// Workflow / node models matching Gateway JSON (contracts/schemas).
class WorkflowNode {
  const WorkflowNode({
    required this.id,
    required this.status,
    this.phaseRef,
    this.title,
    this.dependsOn = const [],
    this.taskId,
    this.unitId,
  });

  final String id;
  final String status;
  final String? phaseRef;
  final String? title;
  final List<String> dependsOn;
  final String? taskId;
  final String? unitId;

  bool get isAwaitingReview => status == 'awaiting_review';

  factory WorkflowNode.fromJson(Map<String, dynamic> json) {
    final deps = json['dependsOn'];
    return WorkflowNode(
      id: json['id'] as String? ?? '',
      status: json['status'] as String? ?? 'pending',
      phaseRef: json['phaseRef'] as String?,
      title: json['title'] as String?,
      dependsOn: deps is List
          ? deps.map((e) => e.toString()).toList(growable: false)
          : const [],
      taskId: json['taskId'] as String?,
      unitId: json['unitId'] as String?,
    );
  }
}

class WorkflowRun {
  const WorkflowRun({
    required this.id,
    required this.bundleId,
    required this.status,
    required this.nodes,
    required this.createdAt,
    this.bundleRef,
    this.slaveId,
    this.repoId,
    this.progressDoc,
    this.updatedAt,
  });

  final String id;
  final String bundleId;
  final String? bundleRef;
  final String? slaveId;
  final String? repoId;
  final String? progressDoc;
  final String status;
  final List<WorkflowNode> nodes;
  final String createdAt;
  final String? updatedAt;

  bool get canStart => status == 'pending';

  bool get isActive =>
      status == 'pending' || status == 'running';

  bool get isTerminal =>
      status == 'completed' || status == 'failed' || status == 'cancelled';

  factory WorkflowRun.fromJson(Map<String, dynamic> json) {
    final rawNodes = json['nodes'];
    final nodes = <WorkflowNode>[];
    if (rawNodes is List) {
      for (final n in rawNodes) {
        if (n is Map<String, dynamic>) {
          nodes.add(WorkflowNode.fromJson(n));
        } else if (n is Map) {
          nodes.add(WorkflowNode.fromJson(Map<String, dynamic>.from(n)));
        }
      }
    }
    return WorkflowRun(
      id: json['id'] as String? ?? '',
      bundleId: json['bundleId'] as String? ?? '',
      bundleRef: json['bundleRef'] as String?,
      slaveId: json['slaveId'] as String?,
      repoId: json['repoId'] as String?,
      progressDoc: json['progressDoc'] as String?,
      status: json['status'] as String? ?? 'pending',
      nodes: nodes,
      createdAt: json['createdAt'] as String? ?? '',
      updatedAt: json['updatedAt'] as String?,
    );
  }
}

class StartWorkflowResult {
  const StartWorkflowResult({
    required this.workflow,
    required this.delivered,
  });

  final WorkflowRun workflow;
  final bool delivered;
}
