part of 'index.dart';

@JsonSerializable()
class GroupSpec {
  const GroupSpec({
    required this.name,
    this.headers = const [],
    this.endpoints = const [],
    this.groups = const [],
  });

  factory GroupSpec.fromJson(Map<String, dynamic> json) =>
      _$GroupSpecFromJson(json);

  final String name;

  final List<HeaderSpec> headers;

  final List<Endpoint> endpoints;

  final List<GroupSpec> groups;

  Map<String, dynamic> toJson() => _$GroupSpecToJson(this);
}
