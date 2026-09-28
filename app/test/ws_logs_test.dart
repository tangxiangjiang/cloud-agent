import 'package:cloud_agent_app/auth/session.dart';
import 'package:cloud_agent_app/ws/app_ws_client.dart';
import 'package:cloud_agent_app/ws/task_log_buffer.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('TaskLogEvent parses envelope and dedups by seq', () {
    final buf = TaskLogBuffer();
    buf.ingest(
      TaskLogEvent.fromJson({
        'type': 'task.event',
        'taskId': 'tsk_1',
        'seq': 1,
        'at': '2026-09-28T00:00:00Z',
        'event': {
          'kind': 'assistant.delta',
          'payload': {'text': 'Hello'},
        },
      }),
    );
    buf.ingest(
      TaskLogEvent.fromJson({
        'type': 'task.event',
        'taskId': 'tsk_1',
        'seq': 1,
        'event': {
          'kind': 'assistant.delta',
          'payload': {'text': 'dup'},
        },
      }),
    );
    buf.ingest(
      TaskLogEvent.fromJson({
        'type': 'task.event',
        'seq': 2,
        'event': {
          'kind': 'assistant.delta',
          'payload': {'text': ' world'},
        },
      }),
    );
    expect(buf.assistant.toString(), 'Hello world');
    expect(buf.lastSeq, 2);
    expect(buf.events.length, 2);
  });

  test('AppWsClient.wsUri maps http(s) to ws(s)', () {
    expect(
      AppWsClient.wsUri('http://127.0.0.1:8080').toString(),
      'ws://127.0.0.1:8080/v1/ws',
    );
    expect(
      AppWsClient.wsUri('https://gw.example.com').toString(),
      'wss://gw.example.com/v1/ws',
    );
  });

  test('dispose marks client disposed (unsubscribe path)', () async {
    final buf = TaskLogBuffer();
    final client = AppWsClient(
      session: const Session(
        gatewayBaseUrl: 'http://127.0.0.1:9',
        token: 'tok',
      ),
      buffer: buf,
      connect: (_) => throw StateError('no real socket in unit test'),
    );
    // start will fail open and schedule reconnect; dispose must cancel.
    await client.start(taskId: 'tsk_x');
    await client.dispose();
    expect(client.isDisposed, isTrue);
  });
}
