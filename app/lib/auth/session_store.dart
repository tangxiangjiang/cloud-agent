import 'package:flutter_secure_storage/flutter_secure_storage.dart';

import 'session.dart';

/// Persist gateway URL + bearer token. Token must use secure storage only.
abstract class SessionStore {
  Future<Session?> load();
  Future<void> save(Session session);
  Future<void> clear();
}

/// Production store backed by [FlutterSecureStorage].
class SecureSessionStore implements SessionStore {
  SecureSessionStore({FlutterSecureStorage? storage})
      : _storage = storage ?? const FlutterSecureStorage();

  static const _kUrl = 'gateway_base_url';
  static const _kToken = 'gateway_bearer_token';
  static const _kExpires = 'gateway_token_expires_at';

  final FlutterSecureStorage _storage;

  @override
  Future<Session?> load() async {
    final url = await _storage.read(key: _kUrl);
    final token = await _storage.read(key: _kToken);
    if (url == null ||
        url.trim().isEmpty ||
        token == null ||
        token.trim().isEmpty) {
      return null;
    }
    final expRaw = await _storage.read(key: _kExpires);
    DateTime? expiresAt;
    if (expRaw != null && expRaw.isNotEmpty) {
      expiresAt = DateTime.tryParse(expRaw);
    }
    return Session(
      gatewayBaseUrl: url.trim(),
      token: token.trim(),
      expiresAt: expiresAt,
    );
  }

  @override
  Future<void> save(Session session) async {
    await _storage.write(key: _kUrl, value: session.gatewayBaseUrl);
    await _storage.write(key: _kToken, value: session.token);
    if (session.expiresAt != null) {
      await _storage.write(
        key: _kExpires,
        value: session.expiresAt!.toUtc().toIso8601String(),
      );
    } else {
      await _storage.delete(key: _kExpires);
    }
  }

  @override
  Future<void> clear() async {
    await _storage.delete(key: _kUrl);
    await _storage.delete(key: _kToken);
    await _storage.delete(key: _kExpires);
  }
}

/// In-memory store for widget / unit tests (no platform channels).
class MemorySessionStore implements SessionStore {
  Session? _session;

  @override
  Future<Session?> load() async => _session;

  @override
  Future<void> save(Session session) async {
    _session = session;
  }

  @override
  Future<void> clear() async {
    _session = null;
  }
}
