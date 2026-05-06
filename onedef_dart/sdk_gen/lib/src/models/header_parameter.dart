part of 'index.dart';

@JsonSerializable()
class HeaderParameter {
  const HeaderParameter({
    required this.name,
    required this.key,
    required this.type,
    required this.required,
    this.description = '',
    this.examples = const [],
  });

  factory HeaderParameter.fromJson(Map<String, dynamic> json) =>
      _$HeaderParameterFromJson(json);

  final String name;
  final String key;
  final TypeUsage type;
  final bool required;

  final String description;

  final List<String> examples;

  Map<String, dynamic> toJson() => _$HeaderParameterToJson(this);
}
