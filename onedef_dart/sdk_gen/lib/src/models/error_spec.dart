part of 'index.dart';

@JsonSerializable()
class ErrorSpec {
  const ErrorSpec({required this.body});

  const ErrorSpec.defaultError()
    : body = const TypeUsage(kind: TypeUsageKind.named, name: 'DefaultError');

  factory ErrorSpec.fromJson(Map<String, dynamic> json) =>
      _$ErrorSpecFromJson(json);

  final TypeUsage body;

  Map<String, dynamic> toJson() => _$ErrorSpecToJson(this);
}
