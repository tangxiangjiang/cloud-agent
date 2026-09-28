import 'package:flutter/material.dart';

/// Compact status chip; [awaiting_review] is visually emphasized.
class NodeStatusChip extends StatelessWidget {
  const NodeStatusChip({super.key, required this.status});

  final String status;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final (label, bg, fg) = _style(scheme);
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
      decoration: BoxDecoration(
        color: bg,
        borderRadius: BorderRadius.circular(8),
        border: status == 'awaiting_review'
            ? Border.all(color: scheme.tertiary, width: 1.5)
            : null,
      ),
      child: Text(
        label,
        style: Theme.of(context).textTheme.labelMedium?.copyWith(
              color: fg,
              fontWeight: status == 'awaiting_review'
                  ? FontWeight.w700
                  : FontWeight.w500,
            ),
      ),
    );
  }

  (String, Color, Color) _style(ColorScheme scheme) {
    switch (status) {
      case 'awaiting_review':
        return ('待审核', scheme.tertiaryContainer, scheme.onTertiaryContainer);
      case 'running':
        return ('进行中', scheme.primaryContainer, scheme.onPrimaryContainer);
      case 'ready':
        return ('就绪', scheme.secondaryContainer, scheme.onSecondaryContainer);
      case 'approved':
        return ('已通过', const Color(0xFFD8F3DC), const Color(0xFF1B4332));
      case 'rejected':
        return ('已驳回', scheme.errorContainer, scheme.onErrorContainer);
      case 'failed':
        return ('失败', scheme.errorContainer, scheme.onErrorContainer);
      case 'cancelled':
        return ('已取消', scheme.surfaceContainerHighest, scheme.onSurfaceVariant);
      case 'skipped':
        return ('跳过', scheme.surfaceContainerHighest, scheme.onSurfaceVariant);
      case 'pending':
      default:
        return (
          status.isEmpty ? 'pending' : status,
          scheme.surfaceContainerHighest,
          scheme.onSurfaceVariant,
        );
    }
  }
}

class WorkflowStatusChip extends StatelessWidget {
  const WorkflowStatusChip({super.key, required this.status});

  final String status;

  @override
  Widget build(BuildContext context) {
    return NodeStatusChip(status: status);
  }
}
