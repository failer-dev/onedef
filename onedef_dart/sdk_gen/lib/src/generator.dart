import 'dart:io';

import 'package:path/path.dart' as path;

import 'spec.dart';

Future<void> writePackage({
  required Spec spec,
  required String packageName,
  required String outputDir,
  required String corePath,
  bool format = false,
}) async {
  final files = renderPackage(
    spec: spec,
    packageName: packageName,
    corePath: corePath,
  );
  final outDir = Directory(outputDir);
  if (outDir.existsSync()) {
    await outDir.delete(recursive: true);
  }
  await outDir.create(recursive: true);

  for (final entry in files.entries) {
    final file = File(path.join(outputDir, entry.key));
    await file.parent.create(recursive: true);
    await file.writeAsString(entry.value);
  }

  if (format) {
    await _formatPackage(outputDir);
  }
}

Future<void> _formatPackage(String outputDir) async {
  final result = await Process.run(Platform.resolvedExecutable, [
    'format',
    outputDir,
  ]);
  if (result.exitCode != 0) {
    throw FormatException(
      'dart format failed: ${result.stderr}',
      result.stdout,
    );
  }
}

Map<String, String> renderPackage({
  required Spec spec,
  required String packageName,
  required String corePath,
}) {
  spec.validate();
  return _withIdentifierInitialisms(spec.naming.initialisms, () {
    if (spec.groups.isNotEmpty) {
      return _renderGroupedPackage(spec, packageName, corePath);
    }

    final groups = _collectGroups(spec.endpoints);
    final modelPlan = _planModels(
      spec,
      groupEndpoints: {
        for (final entry in groups.entries)
          _groupDirectoryNameForName(entry.key): entry.value,
      },
      rootEndpoints: spec.endpoints
          .where((endpoint) => endpoint.group.isEmpty)
          .toList(growable: false),
    );
    return <String, String>{
      'pubspec.yaml': _renderPubspec(packageName, corePath: corePath),
      'lib/$packageName.dart': _renderBarrel(packageName, groups.keys),
      'lib/src/client.dart': _renderClient(spec, groups),
      'lib/src/models.dart': _renderModels(modelPlan.sharedTypes),
      'lib/src/providers.dart': _renderProvidersFile(),
      for (final entry in groups.entries)
        'lib/src/groups/${_groupDirectoryNameForName(entry.key)}/client.dart':
            _renderGroup(
          entry.key,
          entry.value,
        ),
      for (final entry in groups.entries)
        'lib/src/groups/${_groupDirectoryNameForName(entry.key)}/models.dart':
            _renderModels(
          modelPlan.localTypesFor(_groupDirectoryNameForName(entry.key)),
          importSharedModels: true,
        ),
    };
  });
}

Map<String, String> _renderGroupedPackage(
  Spec spec,
  String packageName,
  String corePath,
) {
  final groupEntries = _flattenGroupEntries(spec.groups);
  final modelPlan = _planModels(
    spec,
    groupEndpoints: {
      for (final entry in groupEntries)
        _groupDirectoryName(entry.group): entry.group.endpoints,
    },
    rootEndpoints: spec.endpoints
        .where((endpoint) => endpoint.group.isEmpty)
        .toList(growable: false),
  );
  return <String, String>{
    'pubspec.yaml': _renderPubspec(packageName, corePath: corePath),
    'lib/$packageName.dart': _renderGroupedBarrel(groupEntries),
    'lib/src/client.dart': _renderGroupedClient(spec),
    'lib/src/models.dart': _renderModels(modelPlan.sharedTypes),
    'lib/src/providers.dart': _renderProvidersFile(),
    for (final entry in groupEntries)
      'lib/src/groups/${_groupDirectoryName(entry.group)}/client.dart':
          _renderGroupFile(
        entry.group,
        ancestors: entry.ancestors,
      ),
    for (final entry in groupEntries)
      'lib/src/groups/${_groupDirectoryName(entry.group)}/models.dart':
          _renderModels(
        modelPlan.localTypesFor(_groupDirectoryName(entry.group)),
        importSharedModels: true,
      ),
  };
}

String _renderProvidersFile() {
  return 'typedef HeaderValueProvider<T> = Future<T> Function();\n';
}

Map<String, List<Endpoint>> _collectGroups(List<Endpoint> endpoints) {
  final result = <String, List<Endpoint>>{};
  for (final endpoint in endpoints) {
    if (endpoint.group.isEmpty) {
      continue;
    }
    result.putIfAbsent(endpoint.group, () => <Endpoint>[]).add(endpoint);
  }
  return result;
}

String _renderPubspec(
  String packageName, {
  required String corePath,
  Map<String, String> pathDependencies = const {},
}) {
  final buffer = StringBuffer('''
name: $packageName
publish_to: none
environment:
  sdk: ^3.0.0
dependencies:
  http: ^1.2.0
  onedef_dart_sdk_core:
    path: $corePath
''');
  for (final entry in pathDependencies.entries) {
    buffer
      ..writeln('  ${entry.key}:')
      ..writeln('    path: ${entry.value}');
  }
  return buffer.toString();
}

String _renderBarrel(String packageName, Iterable<String> groups) {
  final buffer = StringBuffer()
    ..writeln("export 'src/client.dart';")
    ..writeln("export 'src/models.dart';")
    ..writeln(
      "export 'package:onedef_dart_sdk_core/onedef_dart_sdk_core.dart';",
    )
    ..writeln("export 'src/providers.dart';");
  for (final group in groups) {
    final directory = _groupDirectoryNameForName(group);
    buffer
      ..writeln("export 'src/groups/$directory/client.dart';")
      ..writeln("export 'src/groups/$directory/models.dart';");
  }
  return buffer.toString();
}

String _renderGroupedBarrel(
  List<({GroupSpec group, List<GroupSpec> ancestors})> groupEntries,
) {
  final buffer = StringBuffer()
    ..writeln("export 'src/client.dart';")
    ..writeln("export 'src/models.dart';")
    ..writeln("export 'src/providers.dart';")
    ..writeln(
      "export 'package:onedef_dart_sdk_core/onedef_dart_sdk_core.dart';",
    );
  for (final entry in groupEntries) {
    final directory = _groupDirectoryName(entry.group);
    buffer
      ..writeln("export 'src/groups/$directory/client.dart';")
      ..writeln("export 'src/groups/$directory/models.dart';");
  }
  return buffer.toString();
}

String _renderGroupedClient(Spec spec) {
  final rootEndpoints = spec.endpoints
      .where((endpoint) => endpoint.group.isEmpty)
      .toList(growable: false);

  final buffer = StringBuffer()
    ..writeln('// ignore_for_file: unused_field, unused_import')
    ..writeln()
    ..writeln("import 'package:http/http.dart' as http;")
    ..writeln(
      "import 'package:onedef_dart_sdk_core/onedef_dart_sdk_core.dart';",
    )
    ..writeln("import 'providers.dart';")
    ..writeln("export 'providers.dart';");
  if (rootEndpoints.isNotEmpty) {
    buffer.writeln("import 'models.dart';");
  }
  for (final group in spec.groups) {
    buffer
        .writeln("import 'groups/${_groupDirectoryName(group)}/client.dart';");
  }

  buffer
    ..writeln()
    ..writeln('class ApiClient {')
    ..writeln('  factory ApiClient({')
    ..writeln('    required String baseUrl,');
  for (final group in spec.groups) {
    for (final header in _headersForGroupTree(group, ancestors: const [])) {
      buffer.writeln(
        '    required HeaderValueProvider<${_dartType(header.type)}> ${_groupHeaderProviderFieldName(group, header)},',
      );
    }
  }
  buffer
    ..writeln('    http.Client? client,')
    ..writeln('  }) {')
    ..writeln(
      '    final transport = Transport(baseUrl: baseUrl, client: client);',
    )
    ..writeln('    return ApiClient._(')
    ..writeln('      transport,');
  for (final group in spec.groups) {
    buffer.writeln(
      '      ${_groupPropertyName(group)}: ${_groupClassName(group)}(',
    );
    buffer.writeln('        transport,');
    for (final header in _headersForGroupTree(group, ancestors: const [])) {
      final providerName = _headerProviderFieldName(header);
      final rootProviderName = _groupHeaderProviderFieldName(group, header);
      buffer.writeln('        $providerName: $rootProviderName,');
    }
    buffer.writeln('      ),');
  }
  buffer
    ..writeln('    );')
    ..writeln('  }')
    ..writeln()
    ..writeln('  ApiClient._(')
    ..writeln('    this._transport, {');
  for (final group in spec.groups) {
    buffer.writeln('    required this.${_groupPropertyName(group)},');
  }
  buffer
    ..writeln('  });')
    ..writeln()
    ..writeln('  final Transport _transport;');
  for (final group in spec.groups) {
    buffer.writeln(
      '  final ${_groupClassName(group)} ${_groupPropertyName(group)};',
    );
  }

  final rootMethods = rootEndpoints
      .map((endpoint) => _renderMethod(endpoint, indent: '  '))
      .join('\n');
  if (rootMethods.isNotEmpty) {
    buffer
      ..writeln()
      ..write(rootMethods);
  }

  buffer.writeln('}');
  return buffer.toString();
}

String _renderGroupFile(GroupSpec group, {required List<GroupSpec> ancestors}) {
  final constructorHeaders = _headersForGroupTree(group, ancestors: ancestors);
  final buffer = StringBuffer()
    ..writeln('// ignore_for_file: unused_import')
    ..writeln()
    ..writeln(
      "import 'package:onedef_dart_sdk_core/onedef_dart_sdk_core.dart';",
    )
    ..writeln("import '../../models.dart';")
    ..writeln("import '../../providers.dart';")
    ..writeln("import 'models.dart';");
  for (final child in group.groups) {
    buffer.writeln("import '../${_groupDirectoryName(child)}/client.dart';");
  }

  final className = _groupClassName(group);
  buffer
    ..writeln()
    ..writeln('class $className {');
  if (constructorHeaders.isEmpty) {
    buffer.writeln('  $className(this._transport);');
  } else {
    buffer
      ..writeln('  $className(')
      ..writeln('    this._transport, {');
    for (final header in constructorHeaders) {
      buffer.writeln(
        '    required HeaderValueProvider<${_dartType(header.type)}> ${_headerProviderFieldName(header)},',
      );
    }
    buffer.write('  }) : ');
    for (var i = 0; i < constructorHeaders.length; i++) {
      final header = constructorHeaders[i];
      final suffix = i == constructorHeaders.length - 1 ? ';\n' : ',\n       ';
      buffer.write(
        '${_headerProviderStorageName(header)} = ${_headerProviderFieldName(header)}$suffix',
      );
    }
  }

  buffer
    ..writeln()
    ..writeln('  final Transport _transport;');
  for (final header in constructorHeaders) {
    buffer.writeln(
      '  final HeaderValueProvider<${_dartType(header.type)}> ${_headerProviderStorageName(header)};',
    );
  }

  for (final child in group.groups) {
    buffer
      ..writeln()
      ..writeln(
        '  ${_groupClassName(child)} get ${_groupPropertyName(child)} => ${_groupClassName(child)}(',
      )
      ..writeln('        _transport,');
    for (final header in _headersForGroupTree(
      child,
      ancestors: <GroupSpec>[...ancestors, group],
    )) {
      buffer.writeln(
        '        ${_headerProviderFieldName(header)}: ${_headerProviderStorageName(header)},',
      );
    }
    buffer.writeln('      );');
  }

  final methods = group.endpoints
      .map(
        (endpoint) =>
            _renderGroupedMethod(endpoint, group, ancestors, indent: '  '),
      )
      .join('\n');
  if (methods.isNotEmpty) {
    buffer
      ..writeln()
      ..write(methods);
  }

  buffer.writeln('}');
  return buffer.toString();
}

List<({GroupSpec group, List<GroupSpec> ancestors})> _flattenGroupEntries(
  List<GroupSpec> groups, [
  List<GroupSpec> ancestors = const [],
]) {
  final result = <({GroupSpec group, List<GroupSpec> ancestors})>[];
  for (final group in groups) {
    result.add((group: group, ancestors: ancestors));
    result.addAll(
      _flattenGroupEntries(group.groups, <GroupSpec>[...ancestors, group]),
    );
  }
  return result;
}

String _renderClient(Spec spec, Map<String, List<Endpoint>> groups) {
  final rootEndpoints = spec.endpoints.where(
    (endpoint) => endpoint.group.isEmpty,
  );
  final hasRootEndpoints = rootEndpoints.isNotEmpty;
  final buffer = StringBuffer()
    ..writeln("import 'package:http/http.dart' as http;")
    ..writeln(
      "import 'package:onedef_dart_sdk_core/onedef_dart_sdk_core.dart';",
    )
    ..writeln("export 'providers.dart';");
  if (hasRootEndpoints) {
    buffer.writeln("import 'models.dart';");
  }
  for (final group in groups.keys) {
    buffer.writeln(
      "import 'groups/${_groupDirectoryNameForName(group)}/client.dart';",
    );
  }
  buffer
    ..writeln()
    ..writeln('class ApiClient {')
    ..writeln(
      '  factory ApiClient({required String baseUrl, http.Client? client}) {',
    )
    ..writeln(
      '    final transport = Transport(baseUrl: baseUrl, client: client);',
    )
    ..writeln('    return ApiClient._(transport);')
    ..writeln('  }')
    ..writeln()
    ..writeln(
      hasRootEndpoints
          ? '  ApiClient._(this._transport)'
          : '  ApiClient._(Transport transport)',
    );

  if (groups.isNotEmpty) {
    final entries = groups.entries.toList();
    for (var i = 0; i < entries.length; i++) {
      final entry = entries[i];
      final prefix = i == 0 ? '      :' : '      ,';
      buffer.writeln(
        '$prefix ${_camelCase(entry.key)} = ${_pascalCase(entry.key)}Group(${hasRootEndpoints ? '_transport' : 'transport'})',
      );
    }
    buffer.writeln('      ;');
  } else {
    buffer.writeln('      ;');
  }

  buffer.writeln();
  if (hasRootEndpoints) {
    buffer.writeln('  final Transport _transport;');
  }
  for (final group in groups.keys) {
    buffer.writeln('  final ${_pascalCase(group)}Group ${_camelCase(group)};');
  }

  final rootMethods = rootEndpoints
      .map((endpoint) => _renderMethod(endpoint, indent: '  '))
      .join('\n');
  if (rootMethods.isNotEmpty) {
    buffer
      ..writeln()
      ..write(rootMethods);
  }

  buffer.writeln('}');
  return buffer.toString();
}

String _renderGroup(String group, List<Endpoint> endpoints) {
  final buffer = StringBuffer()
    ..writeln(
      "import 'package:onedef_dart_sdk_core/onedef_dart_sdk_core.dart';",
    )
    ..writeln("import '../../models.dart';")
    ..writeln("import 'models.dart';")
    ..writeln()
    ..writeln('class ${_pascalCase(group)}Group {')
    ..writeln('  ${_pascalCase(group)}Group(this._transport);')
    ..writeln()
    ..writeln('  final Transport _transport;')
    ..writeln();

  buffer.write(
    endpoints
        .map((endpoint) => _renderMethod(endpoint, indent: '  '))
        .join('\n'),
  );
  buffer.writeln('}');
  return buffer.toString();
}

String _renderMethod(Endpoint endpoint, {required String indent}) {
  return _renderEndpointMethod(
    endpoint,
    indent: indent,
    providerHeaders: const [],
  );
}

String _renderGroupedMethod(
  Endpoint endpoint,
  GroupSpec group,
  List<GroupSpec> ancestors, {
  required String indent,
}) {
  return _renderEndpointMethod(
    endpoint,
    indent: indent,
    providerHeaders: <Parameter>[
      ..._headersFromGroups(ancestors),
      ..._providerHeadersForGroup(group),
    ],
  );
}

String _renderEndpointMethod(
  Endpoint endpoint, {
  required String indent,
  required List<Parameter> providerHeaders,
}) {
  final buffer = StringBuffer();
  final returnType = endpoint.response.body == null
      ? 'void'
      : _dartType(endpoint.response.body!);
  final errorType = _dartType(endpoint.error.body);
  final resultType = 'Result<$returnType, $errorType>';
  final methodName = _endpointMethodName(endpoint);
  final parameters = <String>[];

  for (final parameter in endpoint.request.pathParams) {
    parameters.add(
      'required ${_dartType(parameter.type)} ${_camelCase(parameter.name)}',
    );
  }
  if (endpoint.request.body != null) {
    parameters.add('required ${_dartType(endpoint.request.body!)} body');
  }
  for (final parameter in endpoint.request.headerParams) {
    parameters.add(
      '${parameter.required ? 'required ' : ''}${_dartType(parameter.type, forceOptional: !parameter.required)} ${_camelCase(parameter.name)}',
    );
  }
  for (final parameter in endpoint.request.queryParams) {
    parameters.add(
      '${_dartType(parameter.type, forceOptional: true)} ${_camelCase(parameter.name)}',
    );
  }

  final signature = parameters.isEmpty ? '' : '{${parameters.join(', ')}}';

  buffer.writeln('$indent// ${endpoint.method} ${endpoint.path}');
  buffer.writeln(
    '${indent}Future<$resultType> $methodName($signature) async {',
  );
  final innerIndent = '$indent  ';
  buffer.writeln('${innerIndent}final transport = _transport;');
  buffer.writeln();

  final hasHeaders =
      providerHeaders.isNotEmpty || endpoint.request.headerParams.isNotEmpty;
  if (hasHeaders) {
    buffer.writeln('${innerIndent}final headers = <String, String>{};');
    for (final header in providerHeaders) {
      buffer.writeln(
        "${innerIndent}headers['${header.wireName}'] = (await ${_headerProviderStorageName(header)}()).toString();",
      );
    }
    for (final parameter in endpoint.request.headerParams) {
      final name = _camelCase(parameter.name);
      if (parameter.required) {
        buffer.writeln(
          "${innerIndent}headers['${parameter.wireName}'] = $name.toString();",
        );
      } else {
        buffer.writeln(
          "${innerIndent}if ($name != null) headers['${parameter.wireName}'] = $name.toString();",
        );
      }
    }
    buffer.writeln();
  }

  if (endpoint.request.pathParams.isNotEmpty) {
    buffer.writeln(
      "${innerIndent}final pathParameters = <String, Object?>{${_pathParameterMap(endpoint.request.pathParams)}};",
    );
    buffer.writeln();
  }

  if (endpoint.request.queryParams.isNotEmpty) {
    buffer.writeln('${innerIndent}final queryParameters = <String, String>{};');
    for (final parameter in endpoint.request.queryParams) {
      final name = _camelCase(parameter.name);
      buffer.writeln(
        "${innerIndent}if ($name != null) queryParameters['${parameter.wireName}'] = $name.toString();",
      );
    }
  } else {
    buffer.writeln('${innerIndent}const queryParameters = <String, String>{};');
  }
  buffer.writeln();

  buffer.writeln(
    '${innerIndent}return transport.requestResult<$returnType, $errorType>(',
  );
  buffer
      .writeln('${innerIndent}  method: ${_httpMethodEnum(endpoint.method)},');
  buffer.writeln("${innerIndent}  path: '${_pathTemplate(endpoint.path)}',");
  buffer.writeln('${innerIndent}  expectedStatus: ${endpoint.successStatus},');
  buffer.writeln('${innerIndent}  queryParameters: queryParameters,');
  if (hasHeaders) {
    buffer.writeln('${innerIndent}  headers: headers,');
  }
  if (endpoint.request.body != null) {
    buffer.writeln(
      "${innerIndent}  body: ${_bodyToJson(endpoint.request.body!, 'body')},",
    );
  }
  if (endpoint.response.body != null) {
    buffer.writeln(
      '${innerIndent}  decodeSuccess: (data, status, rawBody) => ${_decodeResponseValue(endpoint.response.body!, "data", "status", "rawBody", transportExpr: "transport")},',
    );
  }
  buffer.writeln(
    '${innerIndent}  decodeError: (data, status, rawBody) => ${_decodeResponseValue(endpoint.error.body, "data", "status", "rawBody", transportExpr: "transport")},',
  );
  buffer.writeln('${innerIndent});');
  buffer.writeln('$indent}');
  return buffer.toString();
}

String _httpMethodEnum(String method) {
  switch (method.toUpperCase()) {
    case 'GET':
      return 'HttpMethod.get';
    case 'POST':
      return 'HttpMethod.post';
    case 'PUT':
      return 'HttpMethod.put';
    case 'PATCH':
      return 'HttpMethod.patch';
    case 'DELETE':
      return 'HttpMethod.delete';
    case 'HEAD':
      return 'HttpMethod.head';
    case 'OPTIONS':
      return 'HttpMethod.options';
    default:
      throw StateError('Unsupported HTTP method $method');
  }
}

String _pathTemplate(String path) {
  final buffer = StringBuffer();
  var index = 0;
  while (index < path.length) {
    final start = path.indexOf('{', index);
    if (start == -1) {
      buffer.write(path.substring(index));
      break;
    }
    final end = path.indexOf('}', start);
    buffer.write(path.substring(index, start));
    final name = path.substring(start + 1, end);
    buffer.write(
      '\${Uri.encodeComponent(pathParameters[\'$name\']!.toString())}',
    );
    index = end + 1;
  }
  return buffer.toString();
}

String _pathParameterMap(List<Parameter> parameters) {
  if (parameters.isEmpty) {
    return '';
  }
  return parameters
      .map(
        (parameter) => "'${parameter.wireName}': ${_camelCase(parameter.name)}",
      )
      .join(', ');
}

class _ModelPlan {
  _ModelPlan({
    required this.typeTable,
    required this.sharedTypeNames,
    required this.groupTypeNames,
  });

  final Map<String, TypeDef> typeTable;
  final Set<String> sharedTypeNames;
  final Map<String, Set<String>> groupTypeNames;

  List<TypeDef> get sharedTypes => _typesFor(sharedTypeNames);

  List<TypeDef> localTypesFor(String group) {
    return _typesFor(groupTypeNames[group] ?? const <String>{});
  }

  List<TypeDef> _typesFor(Set<String> names) {
    return typeTable.values
        .where((type) => names.contains(type.name))
        .toList(growable: false);
  }
}

_ModelPlan _planModels(
  Spec spec, {
  required Map<String, List<Endpoint>> groupEndpoints,
  required List<Endpoint> rootEndpoints,
}) {
  const rootScope = '<root>';
  final typeTable = {for (final type in spec.types) type.name: type};
  final usage = <String, Set<String>>{};
  final groupTypeNames = <String, Set<String>>{
    for (final group in groupEndpoints.keys) group: <String>{},
  };

  void mark(String name, String scope) {
    usage.putIfAbsent(name, () => <String>{}).add(scope);
    if (scope != rootScope) {
      groupTypeNames.putIfAbsent(scope, () => <String>{}).add(name);
    }
  }

  void collectType(TypeRef? type, String scope, Set<String> visiting) {
    if (type == null) {
      return;
    }
    switch (type.kind) {
      case 'named':
        final name = type.name;
        final typeDef = typeTable[name];
        if (typeDef == null) {
          return;
        }
        mark(name, scope);
        if (!visiting.add(name)) {
          return;
        }
        for (final field in typeDef.fields) {
          collectType(field.type, scope, visiting);
        }
        visiting.remove(name);
        break;
      case 'list':
        collectType(type.elem, scope, visiting);
        break;
      case 'map':
        collectType(type.value, scope, visiting);
        break;
    }
  }

  void collectEndpoint(Endpoint endpoint, String scope) {
    final visiting = <String>{};
    collectType(endpoint.request.body, scope, visiting);
    collectType(endpoint.response.body, scope, visiting);
    collectType(endpoint.error.body, scope, visiting);
  }

  for (final endpoint in rootEndpoints) {
    collectEndpoint(endpoint, rootScope);
  }
  for (final entry in groupEndpoints.entries) {
    for (final endpoint in entry.value) {
      collectEndpoint(endpoint, entry.key);
    }
  }

  final sharedTypeNames = <String>{};
  for (final type in spec.types) {
    final scopes = usage[type.name] ?? const <String>{};
    if (scopes.isEmpty || scopes.contains(rootScope) || scopes.length > 1) {
      sharedTypeNames.add(type.name);
    }
  }

  for (final entry in groupTypeNames.entries) {
    entry.value.removeAll(sharedTypeNames);
  }

  return _ModelPlan(
    typeTable: typeTable,
    sharedTypeNames: sharedTypeNames,
    groupTypeNames: groupTypeNames,
  );
}

String _renderModels(
  List<TypeDef> types, {
  bool importSharedModels = false,
}) {
  final buffer = StringBuffer()
    ..writeln('// ignore_for_file: unused_element, unused_import')
    ..writeln()
    ..writeln(
      "import 'package:onedef_dart_sdk_core/onedef_dart_sdk_core.dart';",
    );
  if (importSharedModels) {
    buffer.writeln("import '../../models.dart';");
  }
  buffer.writeln();

  for (final typeDef in types) {
    buffer
      ..writeln('class ${typeDef.name} {')
      ..writeln('  const ${typeDef.name}({');
    for (final field in typeDef.fields) {
      final prefix = field.required ? '    required this.' : '    this.';
      buffer.writeln('$prefix${_camelCase(field.name)},');
    }
    buffer
      ..writeln('  });')
      ..writeln();

    for (final field in typeDef.fields) {
      buffer.writeln(
        '  final ${_dartType(field.type)} ${_camelCase(field.name)};',
      );
    }

    buffer
      ..writeln()
      ..writeln(
        '  factory ${typeDef.name}.fromJson(Map<String, dynamic> json) => ${typeDef.name}(',
      );
    for (final field in typeDef.fields) {
      final fieldName = _camelCase(field.name);
      buffer.writeln(
        "    $fieldName: ${_decodeModelValue(field.type, "json['${field.wireName}']", "${typeDef.name}.${field.wireName}")},",
      );
    }
    buffer
      ..writeln('  );')
      ..writeln()
      ..writeln('  Map<String, dynamic> toJson() => {');
    for (final field in typeDef.fields) {
      final fieldName = _camelCase(field.name);
      if (field.omitEmpty && field.type.nullable) {
        final nonNullable = _nonNullableType(field.type);
        buffer.writeln(
          "    if ($fieldName != null) '${field.wireName}': ${_encodeModelValue(nonNullable, fieldName)},",
        );
      } else {
        buffer.writeln(
          "    '${field.wireName}': ${_encodeModelValue(field.type, fieldName)},",
        );
      }
    }
    buffer
      ..writeln('  };')
      ..writeln('}')
      ..writeln();
  }

  return buffer.toString();
}

TypeRef _nonNullableType(TypeRef type) => TypeRef(
      kind: type.kind,
      name: type.name,
      nullable: false,
      elem: type.elem,
      key: type.key,
      value: type.value,
    );

String _dartType(TypeRef type, {bool forceOptional = false}) {
  final nullable = forceOptional || type.nullable;
  String base;
  switch (type.kind) {
    case 'bool':
      base = 'bool';
    case 'int':
      base = 'int';
    case 'float':
      base = 'double';
    case 'string':
    case 'uuid':
      base = 'String';
    case 'any':
      base = 'Object?';
    case 'named':
      base = type.name;
    case 'list':
      base = 'List<${_dartType(type.elem!)}>';
    case 'map':
      base = 'Map<String, dynamic>';
    default:
      base = 'Object?';
  }

  if (base.endsWith('?') || !nullable) {
    return base;
  }
  return '$base?';
}

String _decodeModelValue(TypeRef type, String expr, String context) {
  if (type.nullable) {
    final nonNullable = TypeRef(
      kind: type.kind,
      name: type.name,
      nullable: false,
      elem: type.elem,
      key: type.key,
      value: type.value,
    );
    return '$expr == null ? null : ${_decodeModelValue(nonNullable, expr, context)}';
  }

  switch (type.kind) {
    case 'bool':
      return '$expr as bool';
    case 'int':
      return '$expr as int';
    case 'float':
      return '($expr as num).toDouble()';
    case 'string':
    case 'uuid':
      return '$expr as String';
    case 'any':
      return expr;
    case 'named':
      return '${type.name}.fromJson(expectJsonObject($expr, \'$context\'))';
    case 'list':
      return '(expectJsonList($expr, \'$context\')).map((element) => ${_decodeCollectionItem(type.elem!, 'element', context)}).toList()';
    case 'map':
      return 'Map<String, dynamic>.from(expectJsonObject($expr, \'$context\'))';
    default:
      return expr;
  }
}

String _decodeCollectionItem(TypeRef type, String expr, String context) {
  if (type.nullable) {
    final nonNullable = TypeRef(
      kind: type.kind,
      name: type.name,
      nullable: false,
      elem: type.elem,
      key: type.key,
      value: type.value,
    );
    return '$expr == null ? null : ${_decodeCollectionItem(nonNullable, expr, context)}';
  }

  switch (type.kind) {
    case 'bool':
      return '$expr as bool';
    case 'int':
      return '$expr as int';
    case 'float':
      return '($expr as num).toDouble()';
    case 'string':
    case 'uuid':
      return '$expr as String';
    case 'any':
      return expr;
    case 'named':
      return '${type.name}.fromJson(expectJsonObject($expr, \'$context\'))';
    case 'list':
      return '(expectJsonList($expr, \'$context\')).map((nested) => ${_decodeCollectionItem(type.elem!, 'nested', context)}).toList()';
    case 'map':
      return 'Map<String, dynamic>.from(expectJsonObject($expr, \'$context\'))';
    default:
      return expr;
  }
}

String _encodeModelValue(TypeRef type, String expr) {
  if (type.nullable) {
    final nonNullable = TypeRef(
      kind: type.kind,
      name: type.name,
      nullable: false,
      elem: type.elem,
      key: type.key,
      value: type.value,
    );
    return '$expr == null ? null : ${_encodeModelValue(nonNullable, expr)}';
  }

  switch (type.kind) {
    case 'named':
      return '$expr.toJson()';
    case 'list':
      return '$expr.map((item) => ${_encodeCollectionItem(type.elem!, 'item')}).toList()';
    default:
      return expr;
  }
}

String _encodeCollectionItem(TypeRef type, String expr) {
  if (type.nullable) {
    final nonNullable = TypeRef(
      kind: type.kind,
      name: type.name,
      nullable: false,
      elem: type.elem,
      key: type.key,
      value: type.value,
    );
    return '$expr == null ? null : ${_encodeCollectionItem(nonNullable, expr)}';
  }

  switch (type.kind) {
    case 'named':
      return '$expr.toJson()';
    case 'list':
      return '$expr.map((nested) => ${_encodeCollectionItem(type.elem!, 'nested')}).toList()';
    default:
      return expr;
  }
}

String _decodeResponseValue(
  TypeRef type,
  String expr,
  String statusExpr,
  String rawBodyExpr, {
  String transportExpr = '_transport',
}) {
  if (type.nullable) {
    final nonNullable = TypeRef(
      kind: type.kind,
      name: type.name,
      nullable: false,
      elem: type.elem,
      key: type.key,
      value: type.value,
    );
    return '$expr == null ? null : ${_decodeResponseValue(nonNullable, expr, statusExpr, rawBodyExpr, transportExpr: transportExpr)}';
  }

  switch (type.kind) {
    case 'bool':
      return "$transportExpr.expectBool($expr, $statusExpr, $rawBodyExpr, 'Expected response data to be a boolean.')";
    case 'int':
      return "$transportExpr.expectInt($expr, $statusExpr, $rawBodyExpr, 'Expected response data to be an integer.')";
    case 'float':
      return "$transportExpr.expectDouble($expr, $statusExpr, $rawBodyExpr, 'Expected response data to be a number.')";
    case 'string':
    case 'uuid':
      return "$transportExpr.expectString($expr, $statusExpr, $rawBodyExpr, 'Expected response data to be a string.')";
    case 'any':
      return expr;
    case 'named':
      return '${type.name}.fromJson($transportExpr.expectJsonObject($expr, $statusExpr, $rawBodyExpr, \'Expected response data to be a JSON object.\'))';
    case 'list':
      return "($transportExpr.expectJsonList($expr, $statusExpr, $rawBodyExpr, 'Expected response data to be a JSON array.')).map((element) => ${_decodeResponseListItem(type.elem!, 'element', statusExpr, rawBodyExpr, transportExpr: transportExpr)}).toList()";
    case 'map':
      return "$transportExpr.expectJsonObject($expr, $statusExpr, $rawBodyExpr, 'Expected response data to be a JSON object.')";
    default:
      return expr;
  }
}

String _decodeResponseListItem(
  TypeRef type,
  String expr,
  String statusExpr,
  String rawBodyExpr, {
  String transportExpr = '_transport',
}) {
  if (type.nullable) {
    final nonNullable = TypeRef(
      kind: type.kind,
      name: type.name,
      nullable: false,
      elem: type.elem,
      key: type.key,
      value: type.value,
    );
    return '$expr == null ? null : ${_decodeResponseListItem(nonNullable, expr, statusExpr, rawBodyExpr, transportExpr: transportExpr)}';
  }

  switch (type.kind) {
    case 'bool':
      return "$transportExpr.expectBool($expr, $statusExpr, $rawBodyExpr, 'Expected response item to be a boolean.')";
    case 'int':
      return "$transportExpr.expectInt($expr, $statusExpr, $rawBodyExpr, 'Expected response item to be an integer.')";
    case 'float':
      return "$transportExpr.expectDouble($expr, $statusExpr, $rawBodyExpr, 'Expected response item to be a number.')";
    case 'string':
    case 'uuid':
      return "$transportExpr.expectString($expr, $statusExpr, $rawBodyExpr, 'Expected response item to be a string.')";
    case 'any':
      return expr;
    case 'named':
      return '${type.name}.fromJson($transportExpr.expectJsonObject($expr, $statusExpr, $rawBodyExpr, \'Expected response item to be a JSON object.\'))';
    case 'list':
      return "($transportExpr.expectJsonList($expr, $statusExpr, $rawBodyExpr, 'Expected response item to be a JSON array.')).map((nested) => ${_decodeResponseListItem(type.elem!, 'nested', statusExpr, rawBodyExpr, transportExpr: transportExpr)}).toList()";
    case 'map':
      return "$transportExpr.expectJsonObject($expr, $statusExpr, $rawBodyExpr, 'Expected response item to be a JSON object.')";
    default:
      return expr;
  }
}

String _bodyToJson(TypeRef type, String expr) {
  if (type.nullable) {
    final nonNullable = TypeRef(
      kind: type.kind,
      name: type.name,
      nullable: false,
      elem: type.elem,
      key: type.key,
      value: type.value,
    );
    return '$expr == null ? null : ${_bodyToJson(nonNullable, expr)}';
  }

  switch (type.kind) {
    case 'named':
      return '$expr.toJson()';
    case 'list':
      return '$expr.map((item) => ${_bodyCollectionItem(type.elem!, 'item')}).toList()';
    default:
      return expr;
  }
}

String _bodyCollectionItem(TypeRef type, String expr) {
  if (type.nullable) {
    final nonNullable = TypeRef(
      kind: type.kind,
      name: type.name,
      nullable: false,
      elem: type.elem,
      key: type.key,
      value: type.value,
    );
    return '$expr == null ? null : ${_bodyCollectionItem(nonNullable, expr)}';
  }

  switch (type.kind) {
    case 'named':
      return '$expr.toJson()';
    case 'list':
      return '$expr.map((nested) => ${_bodyCollectionItem(type.elem!, 'nested')}).toList()';
    default:
      return expr;
  }
}

T _withIdentifierInitialisms<T>(
  Iterable<String> initialisms,
  T Function() render,
) {
  final previous = _identifierNaming;
  _identifierNaming = _IdentifierNaming(initialisms);
  try {
    return render();
  } finally {
    _identifierNaming = previous;
  }
}

var _identifierNaming = _IdentifierNaming();

class _IdentifierNaming {
  _IdentifierNaming([Iterable<String> initialisms = const []])
      : initialisms = _normalizeInitialisms(initialisms);

  final List<String> initialisms;

  String camelCase(String value) {
    if (value.isEmpty) {
      return value;
    }

    final words = identifierWords(value);
    if (words.isEmpty) {
      return value;
    }

    final buffer = StringBuffer(words.first.toLowerCase());
    for (final word in words.skip(1)) {
      if (_isAllUppercase(word) && !isInitialism(word)) {
        buffer.write(word);
        continue;
      }
      final lower = word.toLowerCase();
      buffer
        ..write(lower[0].toUpperCase())
        ..write(lower.substring(1));
    }
    return buffer.toString();
  }

  List<String> identifierWords(String value) {
    final words = <String>[];
    final chunks = value
        .replaceAll('-', '_')
        .split(RegExp(r'[_\s]+'))
        .where((chunk) => chunk.isNotEmpty);

    for (final chunk in chunks) {
      var index = 0;
      while (index < chunk.length) {
        final initialism = _matchInitialism(chunk, index);
        if (initialism != null) {
          words.add(initialism);
          index += initialism.length;
          continue;
        }

        final wordEnd = _normalWordEnd(chunk, index);
        if (wordEnd > index) {
          words.add(chunk.substring(index, wordEnd));
          index = wordEnd;
          continue;
        }

        final upperEnd = _upperRunEnd(chunk, index);
        if (upperEnd > index) {
          words.add(chunk.substring(index, upperEnd));
          index = upperEnd;
          continue;
        }

        final numberEnd = _numberRunEnd(chunk, index);
        if (numberEnd > index) {
          words.add(chunk.substring(index, numberEnd));
          index = numberEnd;
          continue;
        }

        index++;
      }
    }
    return words;
  }

  String? _matchInitialism(String chunk, int start) {
    for (final initialism in initialisms) {
      final end = start + initialism.length;
      if (end > chunk.length || !_hasIdentifierBoundaryAfter(chunk, end)) {
        continue;
      }

      final segment = chunk.substring(start, end);
      if (segment == initialism ||
          (segment == segment.toUpperCase() &&
              segment.toUpperCase() == initialism.toUpperCase())) {
        return segment;
      }
    }
    return null;
  }

  bool isInitialism(String value) => initialisms.any(
        (initialism) => initialism.toUpperCase() == value.toUpperCase(),
      );
}

String _camelCase(String value) => _identifierNaming.camelCase(value);

String _endpointMethodName(Endpoint endpoint) {
  final name = endpoint.sdkName.isEmpty ? endpoint.name : endpoint.sdkName;
  return _camelCase(name);
}

List<String> _normalizeInitialisms(Iterable<String> values) {
  final seen = <String>{};
  final result = <String>[];
  for (final raw in values) {
    final value = raw.trim();
    if (value.isEmpty) {
      continue;
    }
    final key = value.toUpperCase();
    if (seen.add(key)) {
      result.add(value);
    }
  }
  result.sort((a, b) {
    final byLength = b.length.compareTo(a.length);
    if (byLength != 0) {
      return byLength;
    }
    return a.compareTo(b);
  });
  return result;
}

bool _hasIdentifierBoundaryAfter(String chunk, int index) {
  if (index >= chunk.length) {
    return true;
  }
  return !_isLowercase(chunk.codeUnitAt(index));
}

int _normalWordEnd(String chunk, int start) {
  if (start >= chunk.length) {
    return start;
  }

  var index = start;
  if (_isUppercase(chunk.codeUnitAt(index))) {
    if (index + 1 >= chunk.length ||
        !_isLowercase(chunk.codeUnitAt(index + 1))) {
      return start;
    }
    index++;
  } else if (!_isLowercase(chunk.codeUnitAt(index))) {
    return start;
  }

  for (; index < chunk.length; index++) {
    if (!_isLowercase(chunk.codeUnitAt(index))) {
      break;
    }
  }
  return index;
}

int _upperRunEnd(String chunk, int start) {
  var index = start;
  for (; index < chunk.length; index++) {
    if (!_isUppercase(chunk.codeUnitAt(index))) {
      break;
    }
  }
  return index;
}

int _numberRunEnd(String chunk, int start) {
  var index = start;
  for (; index < chunk.length; index++) {
    if (!_isDigit(chunk.codeUnitAt(index))) {
      break;
    }
  }
  return index;
}

String _pascalCase(String value) {
  final parts = value.split('_');
  final buffer = StringBuffer();
  for (final part in parts) {
    if (part.isEmpty) {
      continue;
    }
    buffer.write(part[0].toUpperCase());
    if (part.length > 1) {
      buffer.write(part.substring(1));
    }
  }
  return buffer.toString();
}

bool _isUppercase(int rune) => rune >= 65 && rune <= 90;

bool _isLowercase(int rune) => rune >= 97 && rune <= 122;

bool _isDigit(int rune) => rune >= 48 && rune <= 57;

bool _isAllUppercase(String value) =>
    value.isNotEmpty &&
    value.codeUnits.every((codeUnit) => !_isLowercase(codeUnit));

String _sanitizeIdentifierSegment(String value) {
  return value.replaceAll('-', '_');
}

String _groupUniqueName(GroupSpec group) {
  return _packageNameForPathSegments(group.pathSegments);
}

String _groupDirectoryName(GroupSpec group) => _groupUniqueName(group);

String _groupDirectoryNameForName(String name) =>
    _sanitizeIdentifierSegment(name);

String _groupClassName(GroupSpec group) {
  return '${_pascalCase(_groupUniqueName(group))}Group';
}

String _packageNameForPathSegments(Iterable<String> pathSegments) {
  final segments = pathSegments.map(_sanitizeIdentifierSegment).toList();
  final name = segments.last;
  if (segments.length == 1) {
    return name;
  }

  final parent = segments[segments.length - 2];
  final parentStem = parent.endsWith('_api')
      ? parent.substring(0, parent.length - '_api'.length)
      : parent;
  if (name == parentStem || name.startsWith('${parentStem}_')) {
    return name;
  }
  return '${parentStem}_$name';
}

String _groupPropertyName(GroupSpec group) {
  return _camelCase(_sanitizeIdentifierSegment(group.name));
}

String _headerProviderFieldName(Parameter header) {
  return _camelCase(_sanitizeIdentifierSegment(header.name));
}

String _groupHeaderProviderFieldName(GroupSpec group, Parameter header) {
  return '${_groupPropertyName(group)}${_pascalCase(_headerProviderFieldName(header))}';
}

String _headerProviderStorageName(Parameter header) {
  return '_${_headerProviderFieldName(header)}';
}

List<Parameter> _headersFromGroups(List<GroupSpec> groups) {
  return _uniqueHeaders(
    groups.expand(_providerHeadersForGroup).toList(),
  );
}

List<Parameter> _headersForGroupTree(
  GroupSpec group, {
  required List<GroupSpec> ancestors,
}) {
  final headers = <Parameter>[
    ..._headersFromGroups(ancestors),
    ..._providerHeadersForGroup(group),
  ];
  for (final child in group.groups) {
    headers.addAll(
      _headersForGroupTree(child, ancestors: <GroupSpec>[...ancestors, group]),
    );
  }
  return _uniqueHeaders(headers);
}

List<Parameter> _providerHeadersForGroup(GroupSpec group) {
  if (group.providerHeaders.isNotEmpty) {
    return group.providerHeaders;
  }
  return group.requiredHeaders.map(_stringProviderHeader).toList();
}

Parameter _stringProviderHeader(String wireName) {
  return Parameter(
    name:
        _pascalCase(_sanitizeIdentifierSegment(wireName.replaceAll('-', '_'))),
    wireName: wireName,
    type: const TypeRef(kind: 'string'),
    required: true,
    description: '',
    examples: const [],
  );
}

List<Parameter> _uniqueHeaders(List<Parameter> values) {
  final seen = <String>{};
  final result = <Parameter>[];
  for (final value in values) {
    if (seen.add(value.wireName.trim().toLowerCase())) {
      result.add(value);
    }
  }
  return result;
}
