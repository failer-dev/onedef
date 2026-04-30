part of 'index.dart';

@JsonSerializable()
class HeaderSpec {
  const HeaderSpec({
    required this.key,
    required this.type,
    this.description = '',
    this.examples = const [],
  });

  factory HeaderSpec.fromJson(Map<String, dynamic> json) =>
      _$HeaderSpecFromJson(json);

  final String key;
  final TypeUsage type;

  final String description;

  final List<String> examples;

  Map<String, dynamic> toJson() => _$HeaderSpecToJson(this);
}
