part of 'index.dart';

@JsonSerializable()
class ResponseSpec {
  const ResponseSpec({required this.envelope, this.body});

  factory ResponseSpec.fromJson(Map<String, dynamic> json) =>
      _$ResponseSpecFromJson(json);

  final bool envelope;
  final TypeUsage? body;

  Map<String, dynamic> toJson() => _$ResponseSpecToJson(this);
}
