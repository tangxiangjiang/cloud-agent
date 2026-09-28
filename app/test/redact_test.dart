import 'package:cloud_agent_app/util/redact.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('redactSecret never returns full token', () {
    const token = 'abcdef0123456789deadbeef';
    final r = redactSecret(token);
    expect(r.contains('deadbeef'), isFalse); // keepTail=4 → beef
    expect(r, '***beef');
    expect(r.length < token.length, isTrue);
  });
}
