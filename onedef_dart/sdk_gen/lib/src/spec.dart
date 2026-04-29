class Spec {
  Spec({
    required this.version,
    required this.naming,
    required this.endpoints,
    required this.groups,
    required this.types,
  });

  factory Spec.fromJson(Map<String, dynamic> json) {
    final spec = Spec(
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
    spec.validate();
    return spec;
  }

  final String version;
  final NamingSpec naming;
  final List<Endpoint> endpoints;
  final List<GroupSpec> groups;
  final List<TypeDef> types;

  void validate() {
    if (version != 'v1') {
      throw const SpecValidationException(
        code: 'unsupported_version',
        path: r'$.version',
        message: 'version must be v1',
      );
    }

    final typeTable = <String, TypeDef>{};
    for (var i = 0; i < types.length; i++) {
      final type = types[i];
      final path = '\$.types[$i]';
      if (type.name.isEmpty) {
        throw SpecValidationException(
          code: 'invalid_type_ref',
          path: '$path.name',
          message: 'type name must not be empty',
        );
      }
      if (typeTable.containsKey(type.name)) {
        throw SpecValidationException(
          code: 'duplicate_type',
          path: '$path.name',
          message: 'type name ${type.name} is duplicated',
        );
      }
      if (type.kind != 'object') {
        throw SpecValidationException(
          code: 'invalid_type_ref',
          path: '$path.kind',
          message: 'type kind must be object',
        );
      }
      typeTable[type.name] = type;
    }

    for (var i = 0; i < types.length; i++) {
      final type = types[i];
      for (var j = 0; j < type.fields.length; j++) {
        _validateTypeRef(
          type.fields[j].type,
          '\$.types[$i].fields[$j].type',
          typeTable,
        );
      }
    }

    for (var i = 0; i < endpoints.length; i++) {
      _validateEndpoint(
        endpoints[i],
        '\$.endpoints[$i]',
        typeTable,
      );
    }
    for (var i = 0; i < groups.length; i++) {
      _validateGroup(
        groups[i],
        '\$.groups[$i]',
        typeTable,
      );
    }
  }
}

class NamingSpec {
  NamingSpec({required this.initialisms});

  factory NamingSpec.fromJson(Map<String, dynamic> json) {
    return NamingSpec(
      initialisms: _normalizeInitialisms(
        (json['initialisms'] as List<dynamic>? ?? const [])
            .map((entry) => entry as String)
            .toList(),
      ),
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
    this.providerHeaders = const [],
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
      providerHeaders: (json['providerHeaders'] as List<dynamic>? ?? const [])
          .map((entry) => Parameter.fromJson(entry as Map<String, dynamic>))
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
  final List<Parameter> providerHeaders;
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
  ResponseSpec({required this.envelope, required this.body});

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
  const ErrorSpec({required this.body});

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
    this.description = '',
    this.examples = const [],
  });

  factory Parameter.fromJson(Map<String, dynamic> json) {
    return Parameter(
      name: json['name'] as String,
      wireName: json['wireName'] as String,
      type: TypeRef.fromJson(json['type'] as Map<String, dynamic>),
      required: json['required'] as bool,
      description: (json['description'] as String?) ?? '',
      examples: (json['examples'] as List<dynamic>? ?? const [])
          .map((entry) => entry as String)
          .toList(),
    );
  }

  final String name;
  final String wireName;
  final TypeRef type;
  final bool required;
  final String description;
  final List<String> examples;
}

class TypeDef {
  TypeDef({required this.name, required this.kind, required this.fields});

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

class SpecValidationException implements Exception {
  const SpecValidationException({
    required this.code,
    required this.path,
    required this.message,
  });

  final String code;
  final String path;
  final String message;

  @override
  String toString() => 'SpecValidationException($code at $path: $message)';
}

List<String> _normalizeInitialisms(List<String> values) {
  final seen = <String>{};
  final result = <String>[];
  for (final raw in values) {
    final value = raw.trim();
    if (value.isEmpty) {
      continue;
    }
    final key = value.toUpperCase();
    if (seen.contains(key)) {
      continue;
    }
    seen.add(key);
    result.add(value);
  }
  result.sort((a, b) {
    if (a.length != b.length) {
      return b.length.compareTo(a.length);
    }
    return a.compareTo(b);
  });
  return result;
}

void _validateGroup(
  GroupSpec group,
  String path,
  Map<String, TypeDef> typeTable,
) {
  if (group.name.isEmpty) {
    throw SpecValidationException(
      code: 'invalid_type_ref',
      path: '$path.name',
      message: 'group name must not be empty',
    );
  }
  _validateHeaderNames(group.requiredHeaders, '$path.requiredHeaders');
  for (var i = 0; i < group.providerHeaders.length; i++) {
    _validateParameter(
      group.providerHeaders[i],
      '$path.providerHeaders[$i]',
      typeTable,
    );
  }
  _validateHeaderParams(group.providerHeaders, '$path.providerHeaders');
  for (var i = 0; i < group.endpoints.length; i++) {
    _validateEndpoint(group.endpoints[i], '$path.endpoints[$i]', typeTable);
  }
  for (var i = 0; i < group.groups.length; i++) {
    _validateGroup(group.groups[i], '$path.groups[$i]', typeTable);
  }
}

void _validateEndpoint(
  Endpoint endpoint,
  String path,
  Map<String, TypeDef> typeTable,
) {
  if (endpoint.name.isEmpty) {
    throw SpecValidationException(
      code: 'invalid_type_ref',
      path: '$path.name',
      message: 'endpoint name must not be empty',
    );
  }
  if (!_validMethods.contains(endpoint.method)) {
    throw SpecValidationException(
      code: 'invalid_type_ref',
      path: '$path.method',
      message: 'unsupported HTTP method ${endpoint.method}',
    );
  }
  if (!endpoint.path.startsWith('/')) {
    throw SpecValidationException(
      code: 'path_param_mismatch',
      path: '$path.path',
      message: 'endpoint path must start with /',
    );
  }
  if (endpoint.successStatus < 200 || endpoint.successStatus > 299) {
    throw SpecValidationException(
      code: 'invalid_success_response',
      path: '$path.successStatus',
      message: 'success status must be 2xx',
    );
  }
  _validateHeaderNames(endpoint.requiredHeaders, '$path.requiredHeaders');
  _validatePathParams(endpoint, path);
  _validateRequest(endpoint.request, '$path.request', typeTable);
  _validateResponse(endpoint, '$path.response', typeTable);
  _validateTypeRef(endpoint.error.body, '$path.error.body', typeTable);
}

void _validateRequest(
  RequestSpec request,
  String path,
  Map<String, TypeDef> typeTable,
) {
  for (var i = 0; i < request.pathParams.length; i++) {
    final param = request.pathParams[i];
    _validateParameter(param, '$path.pathParams[$i]', typeTable);
    if (!param.required) {
      throw SpecValidationException(
        code: 'path_param_mismatch',
        path: '$path.pathParams[$i].required',
        message: 'path parameters must be required',
      );
    }
  }
  for (var i = 0; i < request.queryParams.length; i++) {
    _validateParameter(
      request.queryParams[i],
      '$path.queryParams[$i]',
      typeTable,
    );
  }
  for (var i = 0; i < request.headerParams.length; i++) {
    _validateParameter(
      request.headerParams[i],
      '$path.headerParams[$i]',
      typeTable,
    );
  }
  _validateHeaderParams(request.headerParams, '$path.headerParams');
  final body = request.body;
  if (body != null) {
    _validateTypeRef(body, '$path.body', typeTable);
  }
}

void _validateParameter(
  Parameter parameter,
  String path,
  Map<String, TypeDef> typeTable,
) {
  if (parameter.name.isEmpty) {
    throw SpecValidationException(
      code: 'invalid_type_ref',
      path: '$path.name',
      message: 'parameter name must not be empty',
    );
  }
  if (parameter.wireName.isEmpty) {
    throw SpecValidationException(
      code: 'invalid_type_ref',
      path: '$path.wireName',
      message: 'parameter wireName must not be empty',
    );
  }
  _validateTypeRef(parameter.type, '$path.type', typeTable);
}

void _validateHeaderNames(List<String> headers, String path) {
  final seen = <String, String>{};
  for (var i = 0; i < headers.length; i++) {
    final header = headers[i];
    final normalized = _normalizeHeaderName(header);
    if (normalized.isEmpty) {
      throw SpecValidationException(
        code: 'invalid_type_ref',
        path: '$path[$i]',
        message: 'header name must not be empty',
      );
    }
    final existing = seen[normalized];
    if (existing != null) {
      throw SpecValidationException(
        code: 'duplicate_header',
        path: '$path[$i]',
        message: 'header $header duplicates $existing',
      );
    }
    seen[normalized] = header;
  }
}

void _validateHeaderParams(List<Parameter> params, String path) {
  final seen = <String, String>{};
  for (var i = 0; i < params.length; i++) {
    final header = params[i].wireName;
    final normalized = _normalizeHeaderName(header);
    final existing = seen[normalized];
    if (existing != null) {
      throw SpecValidationException(
        code: 'duplicate_header',
        path: '$path[$i].wireName',
        message: 'header parameter $header duplicates $existing',
      );
    }
    seen[normalized] = header;
  }
}

String _normalizeHeaderName(String value) => value.trim().toLowerCase();

void _validateResponse(
  Endpoint endpoint,
  String path,
  Map<String, TypeDef> typeTable,
) {
  if (endpoint.successStatus == 204) {
    if (endpoint.response.envelope || endpoint.response.body != null) {
      throw SpecValidationException(
        code: 'invalid_success_response',
        path: path,
        message: '204 response must not use an envelope or body',
      );
    }
    return;
  }
  if (!endpoint.response.envelope) {
    throw SpecValidationException(
      code: 'invalid_success_response',
      path: '$path.envelope',
      message: 'non-204 response must use the success envelope',
    );
  }
  final body = endpoint.response.body;
  if (body == null) {
    throw SpecValidationException(
      code: 'invalid_success_response',
      path: '$path.body',
      message: 'enveloped response must declare a body',
    );
  }
  _validateTypeRef(body, '$path.body', typeTable);
}

void _validateTypeRef(
  TypeRef type,
  String path,
  Map<String, TypeDef> typeTable,
) {
  switch (type.kind) {
    case 'any':
    case 'bool':
    case 'float':
    case 'int':
    case 'string':
    case 'uuid':
      return;
    case 'named':
      if (type.name.isEmpty) {
        throw SpecValidationException(
          code: 'invalid_type_ref',
          path: '$path.name',
          message: 'named type ref must declare name',
        );
      }
      if (type.name == 'DefaultError') {
        return;
      }
      if (!typeTable.containsKey(type.name)) {
        throw SpecValidationException(
          code: 'unknown_type_ref',
          path: '$path.name',
          message: 'unknown named type ${type.name}',
        );
      }
      return;
    case 'list':
      final elem = type.elem;
      if (elem == null) {
        throw SpecValidationException(
          code: 'invalid_type_ref',
          path: '$path.elem',
          message: 'list type ref must declare elem',
        );
      }
      _validateTypeRef(elem, '$path.elem', typeTable);
      return;
    case 'map':
      final key = type.key;
      if (key != null && key.kind != 'string') {
        throw SpecValidationException(
          code: 'invalid_type_ref',
          path: '$path.key',
          message: 'map key must be string',
        );
      }
      final value = type.value;
      if (value == null) {
        throw SpecValidationException(
          code: 'invalid_type_ref',
          path: '$path.value',
          message: 'map type ref must declare value',
        );
      }
      _validateTypeRef(value, '$path.value', typeTable);
      return;
    default:
      throw SpecValidationException(
        code: 'invalid_type_ref',
        path: '$path.kind',
        message: 'unsupported type kind ${type.kind}',
      );
  }
}

void _validatePathParams(Endpoint endpoint, String path) {
  final pathVars = <String>{};
  for (final match in _pathParamPattern.allMatches(endpoint.path)) {
    pathVars.add(match.group(1)!);
  }
  final paramVars =
      endpoint.request.pathParams.map((param) => param.wireName).toSet();

  for (final pathVar in pathVars) {
    if (!paramVars.contains(pathVar)) {
      throw SpecValidationException(
        code: 'path_param_mismatch',
        path: '$path.request.pathParams',
        message: 'path variable $pathVar is missing from request pathParams',
      );
    }
  }
  for (final paramVar in paramVars) {
    if (!pathVars.contains(paramVar)) {
      throw SpecValidationException(
        code: 'path_param_mismatch',
        path: '$path.request.pathParams',
        message: 'path parameter $paramVar does not exist in endpoint path',
      );
    }
  }
}

const _validMethods = {
  'GET',
  'POST',
  'PUT',
  'PATCH',
  'DELETE',
  'HEAD',
  'OPTIONS',
};

final _pathParamPattern = RegExp(r'\{(\w+)\}');
