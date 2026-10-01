import 'package:cloud_agent_app/ws/task_log_buffer.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('activityFromEvents tracks thinking → tool → writing', () {
    final activity = activityFromEvents([
      const TaskLogEvent(seq: 1, kind: 'status', payload: {'status': 'running'}),
      const TaskLogEvent(seq: 2, kind: 'status', payload: {'status': 'thinking'}),
      const TaskLogEvent(
        seq: 3,
        kind: 'tool.started',
        payload: {'name': 'read', 'summary': 'a.ts', 'callId': 'c1'},
      ),
      const TaskLogEvent(
        seq: 4,
        kind: 'tool.finished',
        payload: {'name': 'read', 'ok': true, 'callId': 'c1'},
      ),
      const TaskLogEvent(
        seq: 5,
        kind: 'assistant.delta',
        payload: {'text': 'hi'},
      ),
    ]);
    expect(activity.phase, 'writing');
    expect(activity.phaseLabel, '生成回复');
    expect(activity.tools, hasLength(1));
    expect(activity.tools.first.done, isTrue);
    expect(activity.tools.first.label, 'read · a.ts');
  });

  test('activityFromEvents dedupes tool by callId', () {
    final activity = activityFromEvents([
      const TaskLogEvent(
        seq: 1,
        kind: 'tool.started',
        payload: {'name': 'read', 'callId': 'c1', 'summary': 'a.ts'},
      ),
      const TaskLogEvent(
        seq: 2,
        kind: 'tool.started',
        payload: {'name': 'read', 'callId': 'c1', 'summary': 'a.ts'},
      ),
      const TaskLogEvent(
        seq: 3,
        kind: 'tool.finished',
        payload: {'name': 'read', 'callId': 'c1', 'ok': true},
      ),
    ]);
    expect(activity.tools, hasLength(1));
    expect(activity.tools.first.done, isTrue);
  });
}
