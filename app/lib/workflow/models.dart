/// Workflow / node models matching Gateway JSON (contracts/schemas).

/// Per-node execution policy (M10). Defaults are all off.
class NodePolicy {
  const NodePolicy({
    this.autoApprove = false,
    this.autoStartNext = false,
  });

  final bool autoApprove;
  final bool autoStartNext;

  static const defaults = NodePolicy();

  factory NodePolicy.fromJson(Map<String, dynamic>? json) {
    if (json == null) return defaults;
    return NodePolicy(
      autoApprove: json['autoApprove'] == true,
      autoStartNext: json['autoStartNext'] == true,
    );
  }

  Map<String, dynamic> toJson() => {
        'autoApprove': autoApprove,
        'autoStartNext': autoStartNext,
      };

  NodePolicy copyWith({bool? autoApprove, bool? autoStartNext}) {
    return NodePolicy(
      autoApprove: autoApprove ?? this.autoApprove,
      autoStartNext: autoStartNext ?? this.autoStartNext,
    );
  }
}

class WorkflowNode {
  const WorkflowNode({
    required this.id,
    required this.status,
    this.phaseRef,
    this.title,
    this.dependsOn = const [],
    this.taskId,
    this.unitId,
    this.model = 'auto',
    this.policy = NodePolicy.defaults,
  });

  final String id;
  final String status;
  final String? phaseRef;
  final String? title;
  final List<String> dependsOn;
  final String? taskId;
  final String? unitId;
  final String model;
  final NodePolicy policy;

  bool get isAwaitingReview => status == 'awaiting_review';

  bool get isReady => status == 'ready';

  bool get canReset =>
      status == 'failed' || status == 'rejected' || status == 'cancelled';

  /// Policy/model edits apply immediately except while actively running.
  bool get policyEditable => status != 'running';

  factory WorkflowNode.fromJson(Map<String, dynamic> json) {
    final deps = json['dependsOn'];
    final rawPolicy = json['policy'];
    Map<String, dynamic>? policyMap;
    if (rawPolicy is Map<String, dynamic>) {
      policyMap = rawPolicy;
    } else if (rawPolicy is Map) {
      policyMap = Map<String, dynamic>.from(rawPolicy);
    }
    final model = (json['model'] as String?)?.trim();
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
      model: (model == null || model.isEmpty) ? 'auto' : model,
      policy: NodePolicy.fromJson(policyMap),
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

  bool get hasReadyNodes => nodes.any((n) => n.isReady);

  /// Show Continue when workflow already started and a ready node waits.
  bool get canContinue => !isTerminal && !canStart && hasReadyNodes;

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
