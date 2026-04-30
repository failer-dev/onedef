part of 'index.dart';

@JsonSerializable()
class FieldDef {
  const FieldDef({
    required this.name,
    required this.key,
    required this.type,
    required this.required,
    this.omitEmpty = false,
  });

  factory FieldDef.fromJson(Map<String, dynamic> json) =>
      _$FieldDefFromJson(json);

  final String name;
  final String key;
  final TypeUsage type;
  final bool required;

  final bool omitEmpty;

  Map<String, dynamic> toJson() => _$FieldDefToJson(this);
}
