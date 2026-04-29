class Spec {
  Spec({
    required this.version,
    required this.naming,
    required this.endpoints,
    required this.groups,
    required this.types,
  });

  factory Spec.fromJson(Map<String, dynamic> json) {
    return Spec(
      version: json['version'] as String,
      naming: NamingSpec.fromJson(
        (json['naming'] as Map<String, dynamic>?) ?? const {},
      ),
      endpoints: (json['endpoints'] as List<dynamic>? ?? const [])
          .map((entry) => Endpoint.fromJson(entry as Map<String, dynamic>))
          .toList(),
      groups: (json['groups'] as List<dynamic>? ?? const [])
          .map((entry) => GroupSpec.fromJson(entry as Map<String, dynamic>))
          .toList(),
      types: (json['types'] as List<dynamic>? ?? const [])
          .map((entry) => TypeDef.fromJson(entry as Map<String, dynamic>))
          .toList(),
    );
  }

  final String version;
  final NamingSpec naming;
  final List<Endpoint> endpoints;
  final List<GroupSpec> groups;
  final List<TypeDef> types;
}

class NamingSpec {
  NamingSpec({
    required this.initialisms,
  });

  factory NamingSpec.fromJson(Map<String, dynamic> json) {
    return NamingSpec(
      initialisms: (json['initialisms'] as List<dynamic>? ?? const [])
          .map((entry) => entry as String)
          .toList(),
    );
  }

  final List<String> initialisms;
}

class GroupSpec {
  GroupSpec({
    required this.id,
    required this.name,
    required this.pathSegments,
    required this.requiredHeaders,
    required this.endpoints,
    required this.groups,
  });

  factory GroupSpec.fromJson(Map<String, dynamic> json) {
    final name = json['name'] as String;
    final pathSegments = (json['pathSegments'] as List<dynamic>? ?? const [])
        .map((entry) => entry as String)
        .toList();
    final normalizedPathSegments =
        pathSegments.isEmpty ? <String>[name] : pathSegments;
    return GroupSpec(
      id: (json['id'] as String?) ?? normalizedPathSegments.join('.'),
      name: name,
      pathSegments: normalizedPathSegments,
      requiredHeaders: (json['requiredHeaders'] as List<dynamic>? ?? const [])
          .map((entry) => entry as String)
          .toList(),
      endpoints: (json['endpoints'] as List<dynamic>? ?? const [])
          .map((entry) => Endpoint.fromJson(entry as Map<String, dynamic>))
          .toList(),
      groups: (json['groups'] as List<dynamic>? ?? const [])
          .map((entry) => GroupSpec.fromJson(entry as Map<String, dynamic>))
          .toList(),
    );
  }

  final String id;
  final String name;
  final List<String> pathSegments;
  final List<String> requiredHeaders;
  final List<Endpoint> endpoints;
  final List<GroupSpec> groups;
}

class Endpoint {
  Endpoint({
    required this.name,
    this.sdkName = '',
    required this.method,
    required this.path,
    required this.successStatus,
    required this.group,
    required this.requiredHeaders,
    required this.request,
    required this.response,
    this.error = const ErrorSpec.defaultError(),
  });

  factory Endpoint.fromJson(Map<String, dynamic> json) {
    return Endpoint(
      name: json['name'] as String,
      sdkName: (json['sdkName'] as String?) ?? '',
      method: json['method'] as String,
      path: json['path'] as String,
      successStatus: json['successStatus'] as int,
      group: (json['group'] as String?) ?? '',
      requiredHeaders: (json['requiredHeaders'] as List<dynamic>? ?? const [])
          .map((entry) => entry as String)
          .toList(),
      request: RequestSpec.fromJson(json['request'] as Map<String, dynamic>),
      response: ResponseSpec.fromJson(json['response'] as Map<String, dynamic>),
      error: ErrorSpec.fromJson(
        (json['error'] as Map<String, dynamic>?) ??
            const {
              'body': {'kind': 'named', 'name': 'DefaultError'},
            },
      ),
    );
  }

  final String name;
  final String sdkName;
  final String method;
  final String path;
  final int successStatus;
  final String group;
  final List<String> requiredHeaders;
  final RequestSpec request;
  final ResponseSpec response;
  final ErrorSpec error;
}

class RequestSpec {
  RequestSpec({
    required this.pathParams,
    required this.queryParams,
    required this.headerParams,
    required this.body,
  });

  factory RequestSpec.fromJson(Map<String, dynamic> json) {
    return RequestSpec(
      pathParams: (json['pathParams'] as List<dynamic>? ?? const [])
          .map((entry) => Parameter.fromJson(entry as Map<String, dynamic>))
          .toList(),
      queryParams: (json['queryParams'] as List<dynamic>? ?? const [])
          .map((entry) => Parameter.fromJson(entry as Map<String, dynamic>))
          .toList(),
      headerParams: (json['headerParams'] as List<dynamic>? ?? const [])
          .map((entry) => Parameter.fromJson(entry as Map<String, dynamic>))
          .toList(),
      body: json['body'] == null
          ? null
          : TypeRef.fromJson(json['body'] as Map<String, dynamic>),
    );
  }

  final List<Parameter> pathParams;
  final List<Parameter> queryParams;
  final List<Parameter> headerParams;
  final TypeRef? body;
}

class ResponseSpec {
  ResponseSpec({
    required this.envelope,
    required this.body,
  });

  factory ResponseSpec.fromJson(Map<String, dynamic> json) {
    return ResponseSpec(
      envelope: json['envelope'] as bool,
      body: json['body'] == null
          ? null
          : TypeRef.fromJson(json['body'] as Map<String, dynamic>),
    );
  }

  final bool envelope;
  final TypeRef? body;
}

class ErrorSpec {
  const ErrorSpec({
    required this.body,
  });

  const ErrorSpec.defaultError()
      : body = const TypeRef(kind: 'named', name: 'DefaultError');

  factory ErrorSpec.fromJson(Map<String, dynamic> json) {
    return ErrorSpec(
      body: TypeRef.fromJson(json['body'] as Map<String, dynamic>),
    );
  }

  final TypeRef body;
}

class Parameter {
  Parameter({
    required this.name,
    required this.wireName,
    required this.type,
    required this.required,
  });

  factory Parameter.fromJson(Map<String, dynamic> json) {
    return Parameter(
      name: json['name'] as String,
      wireName: json['wireName'] as String,
      type: TypeRef.fromJson(json['type'] as Map<String, dynamic>),
      required: json['required'] as bool,
    );
  }

  final String name;
  final String wireName;
  final TypeRef type;
  final bool required;
}

class TypeDef {
  TypeDef({
    required this.name,
    required this.kind,
    required this.fields,
  });

  factory TypeDef.fromJson(Map<String, dynamic> json) {
    return TypeDef(
      name: json['name'] as String,
      kind: json['kind'] as String,
      fields: (json['fields'] as List<dynamic>? ?? const [])
          .map((entry) => FieldDef.fromJson(entry as Map<String, dynamic>))
          .toList(),
    );
  }

  final String name;
  final String kind;
  final List<FieldDef> fields;
}

class FieldDef {
  FieldDef({
    required this.name,
    required this.wireName,
    required this.type,
    required this.required,
    required this.nullable,
    required this.omitEmpty,
  });

  factory FieldDef.fromJson(Map<String, dynamic> json) {
    return FieldDef(
      name: json['name'] as String,
      wireName: json['wireName'] as String,
      type: TypeRef.fromJson(json['type'] as Map<String, dynamic>),
      required: json['required'] as bool,
      nullable: (json['nullable'] as bool?) ?? false,
      omitEmpty: (json['omitEmpty'] as bool?) ?? false,
    );
  }

  final String name;
  final String wireName;
  final TypeRef type;
  final bool required;
  final bool nullable;
  final bool omitEmpty;
}

class TypeRef {
  const TypeRef({
    required this.kind,
    this.name = '',
    this.nullable = false,
    this.elem,
    this.key,
    this.value,
  });

  factory TypeRef.fromJson(Map<String, dynamic> json) {
    return TypeRef(
      kind: json['kind'] as String,
      name: (json['name'] as String?) ?? '',
      nullable: (json['nullable'] as bool?) ?? false,
      elem: json['elem'] == null
          ? null
          : TypeRef.fromJson(json['elem'] as Map<String, dynamic>),
      key: json['key'] == null
          ? null
          : TypeRef.fromJson(json['key'] as Map<String, dynamic>),
      value: json['value'] == null
          ? null
          : TypeRef.fromJson(json['value'] as Map<String, dynamic>),
    );
  }

  final String kind;
  final String name;
  final bool nullable;
  final TypeRef? elem;
  final TypeRef? key;
  final TypeRef? value;
}
