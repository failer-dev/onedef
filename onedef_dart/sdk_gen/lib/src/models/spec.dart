part of 'index.dart';

@JsonSerializable()
class Spec {
  const Spec({
    required this.version,
    required this.routes,
    this.initialisms = const [],
    this.models = const [],
  });

  factory Spec.fromJson(Map<String, dynamic> json) {
    final spec = _$SpecFromJson(json);
    return Spec(
      version: spec.version,
      routes: spec.routes,
      initialisms: _normalizeInitialisms(spec.initialisms),
      models: spec.models,
    );
  }

  final String version;

  final List<String> initialisms;

  final RoutesSpec routes;

  final List<ModelDef> models;

  Map<String, dynamic> toJson() => _$SpecToJson(this);
}

List<String> _normalizeInitialisms(List<String> values) {
  final seen = <String>{};
  final result = <String>[];
  for (final raw in values) {
    final value = raw.trim();
    if (value.isEmpty) {
      continue;
    }
    final key = value.toUpperCase();
    if (seen.add(key)) {
      result.add(value);
    }
  }
  result.sort((a, b) {
    final byLength = b.length.compareTo(a.length);
    if (byLength != 0) {
      return byLength;
    }
    return a.compareTo(b);
  });
  return result;
}
