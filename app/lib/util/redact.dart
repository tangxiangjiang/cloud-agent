/// Redact secrets for logs / UI. Never print a full bearer token.
String redactSecret(String? value, {int keepTail = 4}) {
  if (value == null || value.isEmpty) return '(empty)';
  if (value.length <= keepTail + 2) return '***';
  return '***${value.substring(value.length - keepTail)}';
}
