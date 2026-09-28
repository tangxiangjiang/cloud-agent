import 'package:flutter/material.dart';

import '../auth/auth_controller.dart';
import 'home_shell.dart';
import 'pair_page.dart';

/// Routes to pair or home. Unauthenticated users never see [HomeShell].
class AuthGate extends StatelessWidget {
  const AuthGate({super.key, required this.auth});

  final AuthController auth;

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: auth,
      builder: (context, _) {
        switch (auth.status) {
          case AuthStatus.unknown:
            return const Scaffold(
              body: Center(child: CircularProgressIndicator()),
            );
          case AuthStatus.signedOut:
            return PairPage(auth: auth);
          case AuthStatus.signedIn:
            if (!auth.isSignedIn) {
              return PairPage(auth: auth);
            }
            return HomeShell(auth: auth);
        }
      },
    );
  }
}
