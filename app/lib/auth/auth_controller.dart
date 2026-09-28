import 'package:flutter/foundation.dart';

import '../util/redact.dart';
import 'gateway_api.dart';
import 'session.dart';
import 'session_store.dart';

enum AuthStatus { unknown, signedOut, signedIn }

/// Owns session lifecycle: load secure store → pair → clear.
class AuthController extends ChangeNotifier {
  AuthController({
    required SessionStore store,
    GatewayApi? api,
  })  : _store = store,
        _api = api ?? GatewayApi();

  final SessionStore _store;
  final GatewayApi _api;

  AuthStatus status = AuthStatus.unknown;
  Session? session;
  String? lastError;

  bool get isSignedIn =>
      status == AuthStatus.signedIn && session?.isAuthenticated == true;

  Future<void> bootstrap() async {
    lastError = null;
    try {
      final loaded = await _store.load();
      if (loaded != null && loaded.isAuthenticated) {
        session = loaded;
        status = AuthStatus.signedIn;
        debugPrint(
          'session restored token=${redactSecret(loaded.token)} '
          'base=${loaded.gatewayBaseUrl}',
        );
      } else {
        session = null;
        status = AuthStatus.signedOut;
      }
    } catch (e) {
      session = null;
      status = AuthStatus.signedOut;
      lastError = 'Failed to load session';
      debugPrint('session load failed: $e');
    }
    notifyListeners();
  }

  Future<bool> pair({
    required String gatewayBaseUrl,
    required String pairCode,
  }) async {
    lastError = null;
    notifyListeners();
    try {
      final next = await _api.pair(
        gatewayBaseUrl: gatewayBaseUrl,
        pairCode: pairCode,
      );
      await _store.save(next);
      session = next;
      status = AuthStatus.signedIn;
      debugPrint(
        'paired token=${redactSecret(next.token)} base=${next.gatewayBaseUrl}',
      );
      notifyListeners();
      return true;
    } on PairException catch (e) {
      lastError = e.message;
      notifyListeners();
      return false;
    } catch (e) {
      lastError = 'Pair failed';
      debugPrint('pair unexpected: $e');
      notifyListeners();
      return false;
    }
  }

  Future<void> signOut() async {
    await _store.clear();
    session = null;
    status = AuthStatus.signedOut;
    lastError = null;
    notifyListeners();
  }
}
