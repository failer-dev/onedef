part of 'index.dart';

@JsonSerializable()
class RequestSpec {
  const RequestSpec({
    this.paths = const [],
    this.queries = const [],
    this.headers = const [],
    this.body,
  });

  factory RequestSpec.fromJson(Map<String, dynamic> json) =>
      _$RequestSpecFromJson(json);

  final List<Parameter> paths;

  final List<Parameter> queries;

  final List<HeaderParameter> headers;

  final TypeUsage? body;

  Map<String, dynamic> toJson() => _$RequestSpecToJson(this);
}
