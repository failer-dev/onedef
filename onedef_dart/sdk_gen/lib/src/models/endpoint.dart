part of 'index.dart';

@JsonSerializable()
class Endpoint {
  const Endpoint({
    required this.name,
    this.sdkName = '',
    required this.method,
    required this.path,
    required this.successStatus,
    required this.request,
    required this.response,
    this.error = const ErrorSpec.defaultError(),
  });

  factory Endpoint.fromJson(Map<String, dynamic> json) =>
      _$EndpointFromJson(json);

  final String name;

  final String sdkName;

  final String method;
  final String path;
  final int successStatus;

  final RequestSpec request;
  final ResponseSpec response;

  final ErrorSpec error;

  Map<String, dynamic> toJson() => _$EndpointToJson(this);
}
