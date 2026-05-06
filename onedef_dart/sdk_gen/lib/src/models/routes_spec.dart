part of 'index.dart';

@JsonSerializable()
class RoutesSpec {
  const RoutesSpec({
    this.headers = const [],
    this.endpoints = const [],
    this.groups = const [],
  });

  factory RoutesSpec.fromJson(Map<String, dynamic> json) =>
      _$RoutesSpecFromJson(json);

  final List<HeaderSpec> headers;

  final List<Endpoint> endpoints;

  final List<GroupSpec> groups;

  Map<String, dynamic> toJson() => _$RoutesSpecToJson(this);
}
