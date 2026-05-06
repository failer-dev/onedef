// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'index.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

Endpoint _$EndpointFromJson(Map<String, dynamic> json) => Endpoint(
  name: json['name'] as String,
  sdkName: json['sdkName'] as String? ?? '',
  method: json['method'] as String,
  path: json['path'] as String,
  successStatus: (json['successStatus'] as num).toInt(),
  request: RequestSpec.fromJson(json['request'] as Map<String, dynamic>),
  response: ResponseSpec.fromJson(json['response'] as Map<String, dynamic>),
  error: json['error'] == null
      ? const ErrorSpec.defaultError()
      : ErrorSpec.fromJson(json['error'] as Map<String, dynamic>),
);

Map<String, dynamic> _$EndpointToJson(Endpoint instance) => <String, dynamic>{
  'name': instance.name,
  'sdkName': instance.sdkName,
  'method': instance.method,
  'path': instance.path,
  'successStatus': instance.successStatus,
  'request': instance.request.toJson(),
  'response': instance.response.toJson(),
  'error': instance.error.toJson(),
};

ErrorSpec _$ErrorSpecFromJson(Map<String, dynamic> json) =>
    ErrorSpec(body: TypeUsage.fromJson(json['body']));

Map<String, dynamic> _$ErrorSpecToJson(ErrorSpec instance) => <String, dynamic>{
  'body': instance.body.toJson(),
};

FieldDef _$FieldDefFromJson(Map<String, dynamic> json) => FieldDef(
  name: json['name'] as String,
  key: json['key'] as String,
  type: TypeUsage.fromJson(json['type']),
  required: json['required'] as bool,
  omitEmpty: json['omitEmpty'] as bool? ?? false,
);

Map<String, dynamic> _$FieldDefToJson(FieldDef instance) => <String, dynamic>{
  'name': instance.name,
  'key': instance.key,
  'type': instance.type.toJson(),
  'required': instance.required,
  'omitEmpty': instance.omitEmpty,
};

GroupSpec _$GroupSpecFromJson(Map<String, dynamic> json) => GroupSpec(
  name: json['name'] as String,
  headers:
      (json['headers'] as List<dynamic>?)
          ?.map((e) => HeaderSpec.fromJson(e as Map<String, dynamic>))
          .toList() ??
      const [],
  endpoints:
      (json['endpoints'] as List<dynamic>?)
          ?.map((e) => Endpoint.fromJson(e as Map<String, dynamic>))
          .toList() ??
      const [],
  groups:
      (json['groups'] as List<dynamic>?)
          ?.map((e) => GroupSpec.fromJson(e as Map<String, dynamic>))
          .toList() ??
      const [],
);

Map<String, dynamic> _$GroupSpecToJson(GroupSpec instance) => <String, dynamic>{
  'name': instance.name,
  'headers': instance.headers.map((e) => e.toJson()).toList(),
  'endpoints': instance.endpoints.map((e) => e.toJson()).toList(),
  'groups': instance.groups.map((e) => e.toJson()).toList(),
};

HeaderParameter _$HeaderParameterFromJson(Map<String, dynamic> json) =>
    HeaderParameter(
      name: json['name'] as String,
      key: json['key'] as String,
      type: TypeUsage.fromJson(json['type']),
      required: json['required'] as bool,
      description: json['description'] as String? ?? '',
      examples:
          (json['examples'] as List<dynamic>?)
              ?.map((e) => e as String)
              .toList() ??
          const [],
    );

Map<String, dynamic> _$HeaderParameterToJson(HeaderParameter instance) =>
    <String, dynamic>{
      'name': instance.name,
      'key': instance.key,
      'type': instance.type.toJson(),
      'required': instance.required,
      'description': instance.description,
      'examples': instance.examples,
    };

HeaderSpec _$HeaderSpecFromJson(Map<String, dynamic> json) => HeaderSpec(
  key: json['key'] as String,
  type: TypeUsage.fromJson(json['type']),
  description: json['description'] as String? ?? '',
  examples:
      (json['examples'] as List<dynamic>?)?.map((e) => e as String).toList() ??
      const [],
);

Map<String, dynamic> _$HeaderSpecToJson(HeaderSpec instance) =>
    <String, dynamic>{
      'key': instance.key,
      'type': instance.type.toJson(),
      'description': instance.description,
      'examples': instance.examples,
    };

Parameter _$ParameterFromJson(Map<String, dynamic> json) => Parameter(
  name: json['name'] as String,
  key: json['key'] as String,
  type: TypeUsage.fromJson(json['type']),
  description: json['description'] as String? ?? '',
  examples:
      (json['examples'] as List<dynamic>?)?.map((e) => e as String).toList() ??
      const [],
);

Map<String, dynamic> _$ParameterToJson(Parameter instance) => <String, dynamic>{
  'name': instance.name,
  'key': instance.key,
  'type': instance.type.toJson(),
  'description': instance.description,
  'examples': instance.examples,
};

RequestSpec _$RequestSpecFromJson(Map<String, dynamic> json) => RequestSpec(
  paths:
      (json['paths'] as List<dynamic>?)
          ?.map((e) => Parameter.fromJson(e as Map<String, dynamic>))
          .toList() ??
      const [],
  queries:
      (json['queries'] as List<dynamic>?)
          ?.map((e) => Parameter.fromJson(e as Map<String, dynamic>))
          .toList() ??
      const [],
  headers:
      (json['headers'] as List<dynamic>?)
          ?.map((e) => HeaderParameter.fromJson(e as Map<String, dynamic>))
          .toList() ??
      const [],
  body: json['body'] == null ? null : TypeUsage.fromJson(json['body']),
);

Map<String, dynamic> _$RequestSpecToJson(RequestSpec instance) =>
    <String, dynamic>{
      'paths': instance.paths.map((e) => e.toJson()).toList(),
      'queries': instance.queries.map((e) => e.toJson()).toList(),
      'headers': instance.headers.map((e) => e.toJson()).toList(),
      'body': instance.body?.toJson(),
    };

ResponseSpec _$ResponseSpecFromJson(Map<String, dynamic> json) => ResponseSpec(
  envelope: json['envelope'] as bool,
  body: json['body'] == null ? null : TypeUsage.fromJson(json['body']),
);

Map<String, dynamic> _$ResponseSpecToJson(ResponseSpec instance) =>
    <String, dynamic>{
      'envelope': instance.envelope,
      'body': instance.body?.toJson(),
    };

RoutesSpec _$RoutesSpecFromJson(Map<String, dynamic> json) => RoutesSpec(
  headers:
      (json['headers'] as List<dynamic>?)
          ?.map((e) => HeaderSpec.fromJson(e as Map<String, dynamic>))
          .toList() ??
      const [],
  endpoints:
      (json['endpoints'] as List<dynamic>?)
          ?.map((e) => Endpoint.fromJson(e as Map<String, dynamic>))
          .toList() ??
      const [],
  groups:
      (json['groups'] as List<dynamic>?)
          ?.map((e) => GroupSpec.fromJson(e as Map<String, dynamic>))
          .toList() ??
      const [],
);

Map<String, dynamic> _$RoutesSpecToJson(RoutesSpec instance) =>
    <String, dynamic>{
      'headers': instance.headers.map((e) => e.toJson()).toList(),
      'endpoints': instance.endpoints.map((e) => e.toJson()).toList(),
      'groups': instance.groups.map((e) => e.toJson()).toList(),
    };

Spec _$SpecFromJson(Map<String, dynamic> json) => Spec(
  version: json['version'] as String,
  routes: RoutesSpec.fromJson(json['routes'] as Map<String, dynamic>),
  initialisms:
      (json['initialisms'] as List<dynamic>?)
          ?.map((e) => e as String)
          .toList() ??
      const [],
  models:
      (json['models'] as List<dynamic>?)
          ?.map((e) => ModelDef.fromJson(e as Map<String, dynamic>))
          .toList() ??
      const [],
);

Map<String, dynamic> _$SpecToJson(Spec instance) => <String, dynamic>{
  'version': instance.version,
  'initialisms': instance.initialisms,
  'routes': instance.routes.toJson(),
  'models': instance.models.map((e) => e.toJson()).toList(),
};

ModelDef _$ModelDefFromJson(Map<String, dynamic> json) => ModelDef(
  name: json['name'] as String,
  kind: json['kind'] as String,
  fields:
      (json['fields'] as List<dynamic>?)
          ?.map((e) => FieldDef.fromJson(e as Map<String, dynamic>))
          .toList() ??
      const [],
);

Map<String, dynamic> _$ModelDefToJson(ModelDef instance) => <String, dynamic>{
  'name': instance.name,
  'kind': instance.kind,
  'fields': instance.fields.map((e) => e.toJson()).toList(),
};
