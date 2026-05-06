part of 'index.dart';

@JsonSerializable()
class Parameter {
  const Parameter({
    required this.name,
    required this.key,
    required this.type,
    this.description = '',
    this.examples = const [],
  });

  factory Parameter.fromJson(Map<String, dynamic> json) =>
      _$ParameterFromJson(json);

  final String name;
  final String key;
  final TypeUsage type;

  final String description;

  final List<String> examples;

  Map<String, dynamic> toJson() => _$ParameterToJson(this);
}
