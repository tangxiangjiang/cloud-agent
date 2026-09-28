/// Authenticated session after successful Gateway pairing.
class Session {
  const Session({
    required this.gatewayBaseUrl,
    required this.token,
    this.expiresAt,
  });

  /// Normalized HTTP(S) origin, no trailing slash (e.g. `http://10.0.2.2:8080`).
  final String gatewayBaseUrl;

  /// Bearer token from `POST /v1/auth/pair`. Never log in full.
  final String token;

  final DateTime? expiresAt;

  bool get isAuthenticated => token.isNotEmpty && gatewayBaseUrl.isNotEmpty;
}
