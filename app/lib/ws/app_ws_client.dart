import 'dart:async';
import 'dart:convert';

import 'package:flutter/foundation.dart';
import 'package:web_socket_channel/web_socket_channel.dart';

import '../auth/session.dart';
import 'task_log_buffer.dart';

typedef WsConnect = WebSocketChannel Function(Uri uri);

/// App ↔ Gateway `/v1/ws` client: auth, subscribe, ping, reconnect.
class AppWsClient {
  AppWsClient({
    required this.session,
    required this.buffer,
    this.taskId,
    WsConnect? connect,
    this.reconnectDelay = const Duration(seconds: 2),
    this.pingInterval = const Duration(seconds: 25),
  }) : _connect = connect ?? WebSocketChannel.connect;

  final Session session;
  final TaskLogBuffer buffer;
  String? taskId;
  final WsConnect _connect;
  final Duration reconnectDelay;
  final Duration pingInterval;

  WebSocketChannel? _channel;
  StreamSubscription<dynamic>? _sub;
  Timer? _ping;
  Timer? _reconnect;
  bool _disposed = false;
  bool _wantOpen = false;
  String? _subscribedTaskId;

  bool get isDisposed => _disposed;

  /// Build `ws(s)://host/v1/ws` from HTTP base.
  static Uri wsUri(String gatewayBaseUrl) {
    final http = Uri.parse(gatewayBaseUrl);
    final scheme = http.scheme == 'https' ? 'wss' : 'ws';
    return Uri(
      scheme: scheme,
      host: http.host,
      port: http.hasPort ? http.port : null,
      path: '/v1/ws',
    );
  }

  Future<void> start({String? taskId}) async {
    if (_disposed) return;
    if (taskId != null) this.taskId = taskId;
    _wantOpen = true;
    await _open();
  }

  Future<void> _open() async {
    if (_disposed || !_wantOpen) return;
    await _tearDownSocket(sendUnsubscribe: false);
    try {
      final ch = _connect(wsUri(session.gatewayBaseUrl));
      _channel = ch;
      _sub = ch.stream.listen(
        _onMessage,
        onError: (_) => _scheduleReconnect(),
        onDone: _scheduleReconnect,
        cancelOnError: true,
      );
      _send({
        'type': 'auth',
        'token': session.token,
      });
      _ping?.cancel();
      _ping = Timer.periodic(pingInterval, (_) {
        if (_disposed) return;
        _send({'type': 'ping'});
      });
    } catch (e) {
      debugPrint('ws open failed: $e');
      _scheduleReconnect();
    }
  }

  void _onMessage(dynamic raw) {
    if (_disposed) return;
    Map<String, dynamic>? msg;
    try {
      final decoded = jsonDecode(raw is String ? raw : raw.toString());
      if (decoded is Map) msg = Map<String, dynamic>.from(decoded);
    } catch (_) {
      return;
    }
    if (msg == null) return;
    final type = msg['type'] as String? ?? '';
    switch (type) {
      case 'auth.ok':
        final tid = taskId;
        if (tid != null && tid.isNotEmpty) {
          _subscribe(tid);
        }
        break;
      case 'subscribed':
        _subscribedTaskId = msg['taskId'] as String?;
        break;
      case 'unsubscribed':
        if (msg['taskId'] == _subscribedTaskId) {
          _subscribedTaskId = null;
        }
        break;
      case 'task.event':
        buffer.ingest(TaskLogEvent.fromJson(msg));
        break;
      case 'pong':
      case 'error':
        break;
      default:
        break;
    }
  }

  void _subscribe(String tid) {
    _subscribedTaskId = tid;
    _send({
      'type': 'subscribe',
      'taskId': tid,
      'lastSeq': buffer.lastSeq,
    });
  }

  void _send(Map<String, dynamic> body) {
    final ch = _channel;
    if (ch == null) return;
    try {
      ch.sink.add(jsonEncode(body));
    } catch (_) {}
  }

  void _scheduleReconnect() {
    if (_disposed || !_wantOpen) return;
    _reconnect?.cancel();
    _reconnect = Timer(reconnectDelay, () {
      if (!_disposed && _wantOpen) {
        unawaited(_open());
      }
    });
  }

  Future<void> _tearDownSocket({required bool sendUnsubscribe}) async {
    _ping?.cancel();
    _ping = null;
    if (sendUnsubscribe &&
        _subscribedTaskId != null &&
        _subscribedTaskId!.isNotEmpty) {
      _send({'type': 'unsubscribe', 'taskId': _subscribedTaskId});
    }
    _subscribedTaskId = null;
    await _sub?.cancel();
    _sub = null;
    try {
      await _channel?.sink.close();
    } catch (_) {}
    _channel = null;
  }

  /// Leave page: unsubscribe and stop reconnect. Prevents leaks.
  Future<void> dispose() async {
    if (_disposed) return;
    _disposed = true;
    _wantOpen = false;
    _reconnect?.cancel();
    _reconnect = null;
    await _tearDownSocket(sendUnsubscribe: true);
  }
}
