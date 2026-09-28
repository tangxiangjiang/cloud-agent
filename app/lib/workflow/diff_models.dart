class NodeDiffFile {
  const NodeDiffFile({
    required this.path,
    this.status,
    this.additions,
    this.deletions,
    this.unifiedDiff,
  });

  final String path;
  final String? status;
  final int? additions;
  final int? deletions;
  final String? unifiedDiff;

  factory NodeDiffFile.fromJson(Map<String, dynamic> json) {
    return NodeDiffFile(
      path: json['path'] as String? ?? '',
      status: json['status'] as String?,
      additions: (json['additions'] as num?)?.toInt(),
      deletions: (json['deletions'] as num?)?.toInt(),
      unifiedDiff: json['unifiedDiff'] as String?,
    );
  }
}

class NodeDiff {
  const NodeDiff({
    required this.workflowId,
    required this.nodeId,
    required this.files,
    this.baseline,
  });

  final String workflowId;
  final String nodeId;
  final String? baseline;
  final List<NodeDiffFile> files;

  factory NodeDiff.fromJson(Map<String, dynamic> json) {
    final raw = json['files'];
    final files = <NodeDiffFile>[];
    if (raw is List) {
      for (final f in raw) {
        if (f is Map) {
          files.add(NodeDiffFile.fromJson(Map<String, dynamic>.from(f)));
        }
      }
    }
    return NodeDiff(
      workflowId: json['workflowId'] as String? ?? '',
      nodeId: json['nodeId'] as String? ?? '',
      baseline: json['baseline'] as String?,
      files: files,
    );
  }
}
