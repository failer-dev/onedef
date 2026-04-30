part of 'index.dart';

@JsonSerializable()
class ModelDef {
  const ModelDef({
    required this.name,
    required this.kind,
    this.fields = const [],
  });

  factory ModelDef.fromJson(Map<String, dynamic> json) =>
      _$ModelDefFromJson(json);

  final String name;
  final String kind;

  final List<FieldDef> fields;

  Map<String, dynamic> toJson() => _$ModelDefToJson(this);
}
