import 'package:flutter/material.dart';

import '../auth/auth_controller.dart';

/// Gateway URL + pair code login. No workflow business UI (M06-P02+).
class PairPage extends StatefulWidget {
  const PairPage({super.key, required this.auth});

  final AuthController auth;

  @override
  State<PairPage> createState() => _PairPageState();
}

class _PairPageState extends State<PairPage> {
  final _urlCtrl = TextEditingController(text: 'http://10.0.2.2:8080');
  final _codeCtrl = TextEditingController();
  bool _busy = false;

  @override
  void dispose() {
    _urlCtrl.dispose();
    _codeCtrl.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (_busy) return;
    setState(() => _busy = true);
    final ok = await widget.auth.pair(
      gatewayBaseUrl: _urlCtrl.text,
      pairCode: _codeCtrl.text,
    );
    if (!mounted) return;
    setState(() => _busy = false);
    if (!ok && widget.auth.lastError != null) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(widget.auth.lastError!)),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Scaffold(
      body: SafeArea(
        child: Center(
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 420),
            child: ListView(
              padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 32),
              children: [
                Text(
                  'cloud-agent',
                  style: theme.textTheme.headlineMedium?.copyWith(
                    fontWeight: FontWeight.w700,
                  ),
                ),
                const SizedBox(height: 8),
                Text(
                  'Pair with your Gateway to continue. '
                  'This app never holds CURSOR_API_KEY.',
                  style: theme.textTheme.bodyMedium?.copyWith(
                    color: theme.colorScheme.onSurfaceVariant,
                  ),
                ),
                const SizedBox(height: 32),
                TextField(
                  controller: _urlCtrl,
                  enabled: !_busy,
                  keyboardType: TextInputType.url,
                  autocorrect: false,
                  decoration: const InputDecoration(
                    labelText: 'Gateway base URL',
                    hintText: 'http://10.0.2.2:8080',
                    border: OutlineInputBorder(),
                    helperText:
                        'Android emulator → host: 10.0.2.2; iOS sim → localhost',
                  ),
                ),
                const SizedBox(height: 16),
                TextField(
                  controller: _codeCtrl,
                  enabled: !_busy,
                  textCapitalization: TextCapitalization.characters,
                  autocorrect: false,
                  decoration: const InputDecoration(
                    labelText: 'Pair code',
                    hintText: 'XXXX-XXXX',
                    border: OutlineInputBorder(),
                  ),
                  onSubmitted: (_) => _submit(),
                ),
                const SizedBox(height: 24),
                FilledButton(
                  onPressed: _busy ? null : _submit,
                  child: _busy
                      ? const SizedBox(
                          height: 20,
                          width: 20,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        )
                      : const Text('Pair & continue'),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
