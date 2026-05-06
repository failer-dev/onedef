Map<String, dynamic> expectJsonObject(Object? value, String context) {
  if (value is Map<String, dynamic>) {
    return value;
  }
  if (value is Map) {
    return value.map((key, value) => MapEntry(key.toString(), value));
  }
  throw FormatException('Expected JSON object for $context.');
}

List<dynamic> expectJsonList(Object? value, String context) {
  if (value is List<dynamic>) {
    return value;
  }
  if (value is List) {
    return List<dynamic>.from(value);
  }
  throw FormatException('Expected JSON array for $context.');
}
