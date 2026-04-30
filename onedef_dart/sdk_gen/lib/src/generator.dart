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
  return _withIdentifierInitialisms(spec.initialisms, () {
    final plan = _PackagePlan.fromSpec(
      spec: spec,
      packageName: packageName,
      corePath: corePath,
    );
    return _renderPackagePlan(plan);
  });
}

Map<String, String> _renderPackagePlan(_PackagePlan plan) {
  return <String, String>{
    'pubspec.yaml': _renderPubspec(plan.packageName, corePath: plan.corePath),
    'lib/${plan.packageName}.dart': _renderBarrel(plan.groupPlans),
    'lib/src/client.dart': _renderApiClient(plan),
    'lib/src/models.dart': _renderModels(plan.modelPlan.sharedModels),
    'lib/src/providers.dart': _renderProvidersFile(),
    for (final groupPlan in plan.groupPlans)
      'lib/src/groups/${_groupDirectoryName(groupPlan)}/client.dart':
          _renderGroupFile(groupPlan),
    for (final groupPlan in plan.groupPlans)
      'lib/src/groups/${_groupDirectoryName(groupPlan)}/models.dart':
          _renderModels(
            plan.modelPlan.localModelsFor(_groupDirectoryName(groupPlan)),
            importSharedModels: true,
          ),
  };
}

class _PackagePlan {
  _PackagePlan({
    required this.packageName,
    required this.corePath,
    required this.rootHeaders,
    required this.rootEndpoints,
    required this.rootGroupPlans,
    required this.groupPlans,
    required this.modelPlan,
  });

  factory _PackagePlan.fromSpec({
    required Spec spec,
    required String packageName,
    required String corePath,
  }) {
    final groupPlans = _flattenGroupEntries(spec.routes.groups)
        .map(
          (entry) => _GroupPlan(
            group: entry.group,
            ancestors: entry.ancestors,
            rootHeaders: spec.routes.headers,
          ),
        )
        .toList(growable: false);
    final rootEndpoints = spec.routes.endpoints;
    final modelPlan = _planModels(
      spec,
      groupEndpoints: {
        for (final groupPlan in groupPlans)
          _groupDirectoryName(groupPlan): groupPlan.group.endpoints,
      },
      rootEndpoints: rootEndpoints,
    );
    return _PackagePlan(
      packageName: packageName,
      corePath: corePath,
      rootHeaders: spec.routes.headers,
      rootEndpoints: rootEndpoints,
      rootGroupPlans: groupPlans
          .where((groupPlan) => groupPlan.ancestors.isEmpty)
          .toList(growable: false),
      groupPlans: groupPlans,
      modelPlan: modelPlan,
    );
  }

  final String packageName;
  final String corePath;
  final List<HeaderSpec> rootHeaders;
  final List<Endpoint> rootEndpoints;
  final List<_GroupPlan> rootGroupPlans;
  final List<_GroupPlan> groupPlans;
  final _ModelPlan modelPlan;
}

class _GroupPlan {
  _GroupPlan({
    required this.group,
    required this.ancestors,
    required List<HeaderSpec> rootHeaders,
  }) : ancestorHeaders = _uniqueHeaders([
         ...rootHeaders,
         ..._headersFromGroups(ancestors),
       ]),
       ownHeaders = _groupHeadersForGroup(group),
       pathSegments = [
         for (final ancestor in ancestors) ancestor.name,
         group.name,
       ];

  final GroupSpec group;
  final List<GroupSpec> ancestors;
  final List<HeaderSpec> ancestorHeaders;
  final List<HeaderSpec> ownHeaders;
  final List<String> pathSegments;

  List<HeaderSpec> get boundHeaders =>
      _uniqueHeaders(<HeaderSpec>[...ancestorHeaders, ...ownHeaders]);

  List<_EndpointMethodPlan> get methodPlans => group.endpoints
      .map(
        (endpoint) => _EndpointMethodPlan(
          endpoint: endpoint,
          groupHeaders: boundHeaders,
          indent: '  ',
        ),
      )
      .toList(growable: false);
}

class _EndpointMethodPlan {
  const _EndpointMethodPlan({
    required this.endpoint,
    required this.groupHeaders,
    required this.indent,
  });

  final Endpoint endpoint;
  final List<HeaderSpec> groupHeaders;
  final String indent;
}

class _ModelClassPlan {
  const _ModelClassPlan(this.model);

  final ModelDef model;
}

String _renderProvidersFile() {
  return 'typedef HeaderValueProvider<T> = Future<T> Function();\n';
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

String _renderBarrel(List<_GroupPlan> groupPlans) {
  final buffer = StringBuffer()
    ..writeln("export 'src/client.dart';")
    ..writeln("export 'src/models.dart';")
    ..writeln("export 'src/providers.dart';")
    ..writeln(
      "export 'package:onedef_dart_sdk_core/onedef_dart_sdk_core.dart';",
    );
  for (final groupPlan in groupPlans) {
    final directory = _groupDirectoryName(groupPlan);
    buffer
      ..writeln("export 'src/groups/$directory/client.dart';")
      ..writeln("export 'src/groups/$directory/models.dart';");
  }
  return buffer.toString();
}

String _renderApiClient(_PackagePlan plan) {
  final buffer = StringBuffer()
    ..writeln('// ignore_for_file: unused_field, unused_import')
    ..writeln()
    ..writeln("import 'package:http/http.dart' as http;")
    ..writeln(
      "import 'package:onedef_dart_sdk_core/onedef_dart_sdk_core.dart';",
    )
    ..writeln("import 'providers.dart';")
    ..writeln("export 'providers.dart';");
  if (plan.rootEndpoints.isNotEmpty) {
    buffer.writeln("import 'models.dart';");
  }
  for (final groupPlan in plan.rootGroupPlans) {
    buffer.writeln(
      "import 'groups/${_groupDirectoryName(groupPlan)}/client.dart';",
    );
  }

  buffer
    ..writeln()
    ..writeln('class ApiClient {')
    ..writeln('  factory ApiClient({')
    ..writeln('    required String baseUrl,');
  for (final header in plan.rootHeaders) {
    buffer.writeln(
      '    required HeaderValueProvider<${_dartType(header.type)}> ${_headerProviderFieldName(header)},',
    );
  }
  for (final groupPlan in plan.rootGroupPlans) {
    buffer.writeln(_renderGroupConfigParameter(groupPlan, indent: '    '));
  }
  buffer
    ..writeln('    http.Client? client,')
    ..writeln('  }) {')
    ..writeln(
      '    final transport = Transport(baseUrl: baseUrl, client: client);',
    )
    ..writeln('    return ApiClient._(')
    ..writeln('      transport,');
  for (final header in plan.rootHeaders) {
    buffer.writeln(
      '      ${_headerProviderFieldName(header)}: ${_headerProviderFieldName(header)},',
    );
  }
  for (final groupPlan in plan.rootGroupPlans) {
    final propertyName = _groupPropertyName(groupPlan.group);
    buffer
      ..writeln(
        '      $propertyName: ${_groupClientClassName(groupPlan)}._bind(',
      )
      ..writeln('        transport,')
      ..writeln('        $propertyName,');
    for (final header in plan.rootHeaders) {
      buffer.writeln(
        '        ${_headerProviderFieldName(header)}: ${_headerProviderFieldName(header)},',
      );
    }
    buffer.writeln('      ),');
  }
  buffer
    ..writeln('    );')
    ..writeln('  }')
    ..writeln()
    ..writeln('  ApiClient._(')
    ..write('    this._transport');
  final hasPrivateNamedArgs =
      plan.rootHeaders.isNotEmpty || plan.rootGroupPlans.isNotEmpty;
  if (!hasPrivateNamedArgs) {
    buffer.writeln(',');
    buffer.writeln('  });');
  } else {
    buffer.writeln(', {');
    for (final header in plan.rootHeaders) {
      buffer.writeln(
        '    required HeaderValueProvider<${_dartType(header.type)}> ${_headerProviderFieldName(header)},',
      );
    }
    for (final groupPlan in plan.rootGroupPlans) {
      buffer.writeln(
        '    required this.${_groupPropertyName(groupPlan.group)},',
      );
    }
    if (plan.rootHeaders.isEmpty) {
      buffer.writeln('  });');
    } else {
      buffer.write('  }) : ');
      for (var i = 0; i < plan.rootHeaders.length; i++) {
        final header = plan.rootHeaders[i];
        final suffix = i == plan.rootHeaders.length - 1 ? ';\n' : ',\n       ';
        buffer.write(
          '${_headerProviderStorageName(header)} = ${_headerProviderFieldName(header)}$suffix',
        );
      }
    }
  }
  buffer
    ..writeln()
    ..writeln('  final Transport _transport;');
  for (final header in plan.rootHeaders) {
    buffer.writeln(
      '  final HeaderValueProvider<${_dartType(header.type)}> ${_headerProviderStorageName(header)};',
    );
  }
  for (final groupPlan in plan.rootGroupPlans) {
    buffer.writeln(
      '  final ${_groupClientClassName(groupPlan)} ${_groupPropertyName(groupPlan.group)};',
    );
  }

  final rootMethods = plan.rootEndpoints
      .map(
        (endpoint) => _renderEndpointMethod(
          _EndpointMethodPlan(
            endpoint: endpoint,
            groupHeaders: plan.rootHeaders,
            indent: '  ',
          ),
        ),
      )
      .join('\n');
  if (rootMethods.isNotEmpty) {
    buffer
      ..writeln()
      ..write(rootMethods);
  }

  buffer.writeln('}');
  return buffer.toString();
}

String _renderGroupFile(_GroupPlan plan) {
  final group = plan.group;
  final ancestorHeaders = plan.ancestorHeaders;
  final ownHeaders = plan.ownHeaders;
  final constructorHeaders = plan.boundHeaders;
  final buffer = StringBuffer()
    ..writeln('// ignore_for_file: unused_element, unused_field, unused_import')
    ..writeln()
    ..writeln(
      "import 'package:onedef_dart_sdk_core/onedef_dart_sdk_core.dart';",
    )
    ..writeln("import '../../models.dart';")
    ..writeln("import '../../providers.dart';")
    ..writeln("import 'models.dart';");
  for (final child in group.groups) {
    buffer.writeln(
      "import '../${_groupDirectoryNameForSegments(_childPathSegments(plan, child))}/client.dart';",
    );
  }

  final configClassName = _groupConfigClassName(plan);
  buffer
    ..writeln()
    ..writeln('class $configClassName {');
  if (ownHeaders.isEmpty && group.groups.isEmpty) {
    buffer.writeln('  const $configClassName();');
  } else {
    buffer..writeln('  const $configClassName({');
    for (final header in ownHeaders) {
      buffer.writeln('    required this.${_headerProviderFieldName(header)},');
    }
    for (final child in group.groups) {
      final childPropertyName = _groupPropertyName(child);
      if (_groupConfigHasRequiredValues(child)) {
        buffer.writeln('    required this.$childPropertyName,');
      } else {
        buffer.writeln(
          '    this.$childPropertyName = const ${_groupConfigClassNameForSegments(_childPathSegments(plan, child))}(),',
        );
      }
    }
    buffer.writeln('  });');
  }

  for (final header in ownHeaders) {
    buffer.writeln(
      '  final HeaderValueProvider<${_dartType(header.type)}> ${_headerProviderFieldName(header)};',
    );
  }
  for (final child in group.groups) {
    buffer.writeln(
      '  final ${_groupConfigClassNameForSegments(_childPathSegments(plan, child))} ${_groupPropertyName(child)};',
    );
  }
  buffer.writeln('}');

  final className = _groupClientClassName(plan);
  buffer
    ..writeln()
    ..writeln('class $className {');
  if (constructorHeaders.isEmpty && group.groups.isEmpty) {
    buffer.writeln('  $className._(this._transport);');
  } else {
    buffer
      ..writeln('  $className._(')
      ..writeln('    this._transport, {');
    for (final header in constructorHeaders) {
      buffer.writeln(
        '    required HeaderValueProvider<${_dartType(header.type)}> ${_headerProviderFieldName(header)},',
      );
    }
    for (final child in group.groups) {
      buffer.writeln('    required this.${_groupPropertyName(child)},');
    }
    buffer.write('  }) : ');
    if (constructorHeaders.isEmpty) {
      buffer.write('super();\n');
    }
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
    ..writeln('  static $className _bind(')
    ..writeln('    Transport transport,')
    ..writeln('    $configClassName config,');
  if (ancestorHeaders.isEmpty) {
    buffer.writeln('  ) {');
  } else {
    buffer.writeln('    {');
    for (final header in ancestorHeaders) {
      buffer.writeln(
        '    required HeaderValueProvider<${_dartType(header.type)}> ${_headerProviderFieldName(header)},',
      );
    }
    buffer.writeln('  }) {');
  }
  buffer
    ..writeln('    return $className._(')
    ..writeln('      transport,');
  for (final header in ancestorHeaders) {
    buffer.writeln(
      '      ${_headerProviderFieldName(header)}: ${_headerProviderFieldName(header)},',
    );
  }
  for (final header in ownHeaders) {
    buffer.writeln(
      '      ${_headerProviderFieldName(header)}: config.${_headerProviderFieldName(header)},',
    );
  }
  for (final child in group.groups) {
    buffer.writeln(
      '      ${_groupPropertyName(child)}: ${_groupClientClassNameForSegments(_childPathSegments(plan, child))}._bind(',
    );
    buffer
      ..writeln('        transport,')
      ..writeln('        config.${_groupPropertyName(child)},');
    for (final header in _uniqueHeaders([...ancestorHeaders, ...ownHeaders])) {
      final value =
          ownHeaders.any(
            (ownHeader) =>
                ownHeader.key.trim().toLowerCase() ==
                header.key.trim().toLowerCase(),
          )
          ? 'config.${_headerProviderFieldName(header)}'
          : _headerProviderFieldName(header);
      buffer.writeln('        ${_headerProviderFieldName(header)}: $value,');
    }
    buffer.writeln('      ),');
  }
  buffer
    ..writeln('    );')
    ..writeln('  }');

  buffer
    ..writeln()
    ..writeln('  final Transport _transport;');
  for (final header in constructorHeaders) {
    buffer.writeln(
      '  final HeaderValueProvider<${_dartType(header.type)}> ${_headerProviderStorageName(header)};',
    );
  }

  for (final child in group.groups) {
    buffer..writeln(
      '  final ${_groupClientClassNameForSegments(_childPathSegments(plan, child))} ${_groupPropertyName(child)};',
    );
  }

  final methods = plan.methodPlans.map(_renderEndpointMethod).join('\n');
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

String _renderEndpointMethod(_EndpointMethodPlan plan) {
  final endpoint = plan.endpoint;
  final indent = plan.indent;
  final groupHeaders = plan.groupHeaders;
  final buffer = StringBuffer();
  final returnType = endpoint.response.body == null
      ? 'void'
      : _dartType(endpoint.response.body!);
  final errorType = _dartType(endpoint.error.body);
  final resultType = 'Result<$returnType, $errorType>';
  final methodName = _endpointMethodName(endpoint);
  final parameters = <String>[];

  for (final parameter in endpoint.request.paths) {
    parameters.add(
      'required ${_dartType(parameter.type)} ${_camelCase(parameter.name)}',
    );
  }
  if (endpoint.request.body != null) {
    parameters.add('required ${_dartType(endpoint.request.body!)} body');
  }
  for (final parameter in endpoint.request.headers) {
    parameters.add(
      '${parameter.required ? 'required ' : ''}${_dartType(parameter.type, forceOptional: !parameter.required)} ${_camelCase(parameter.name)}',
    );
  }
  for (final parameter in endpoint.request.queries) {
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
      groupHeaders.isNotEmpty || endpoint.request.headers.isNotEmpty;
  if (hasHeaders) {
    buffer.writeln('${innerIndent}final headers = <String, String>{};');
    for (final header in groupHeaders) {
      buffer.writeln(
        "${innerIndent}headers['${header.key}'] = (await ${_headerProviderStorageName(header)}()).toString();",
      );
    }
    for (final parameter in endpoint.request.headers) {
      final name = _camelCase(parameter.name);
      if (parameter.required) {
        buffer.writeln(
          "${innerIndent}headers['${parameter.key}'] = $name.toString();",
        );
      } else {
        buffer.writeln(
          "${innerIndent}if ($name != null) headers['${parameter.key}'] = $name.toString();",
        );
      }
    }
    buffer.writeln();
  }

  if (endpoint.request.paths.isNotEmpty) {
    buffer.writeln(
      "${innerIndent}final pathParameters = <String, Object?>{${_pathParameterMap(endpoint.request.paths)}};",
    );
    buffer.writeln();
  }

  if (endpoint.request.queries.isNotEmpty) {
    buffer.writeln('${innerIndent}final queryParameters = <String, String>{};');
    for (final parameter in endpoint.request.queries) {
      final name = _camelCase(parameter.name);
      buffer.writeln(
        "${innerIndent}if ($name != null) queryParameters['${parameter.key}'] = $name.toString();",
      );
    }
  } else {
    buffer.writeln('${innerIndent}const queryParameters = <String, String>{};');
  }
  buffer.writeln();

  buffer.writeln(
    '${innerIndent}return transport.requestResult<$returnType, $errorType>(',
  );
  buffer.writeln(
    '${innerIndent}  method: ${_httpMethodEnum(endpoint.method)},',
  );
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
      .map((parameter) => "'${parameter.key}': ${_camelCase(parameter.name)}")
      .join(', ');
}

class _ModelPlan {
  _ModelPlan({
    required this.modelTable,
    required this.sharedModelNames,
    required this.groupModelNames,
  });

  final Map<String, ModelDef> modelTable;
  final Set<String> sharedModelNames;
  final Map<String, Set<String>> groupModelNames;

  List<_ModelClassPlan> get sharedModels => _modelsFor(sharedModelNames);

  List<_ModelClassPlan> localModelsFor(String group) {
    return _modelsFor(groupModelNames[group] ?? const <String>{});
  }

  List<_ModelClassPlan> _modelsFor(Set<String> names) {
    return modelTable.values
        .where((model) => names.contains(model.name))
        .map(_ModelClassPlan.new)
        .toList(growable: false);
  }
}

_ModelPlan _planModels(
  Spec spec, {
  required Map<String, List<Endpoint>> groupEndpoints,
  required List<Endpoint> rootEndpoints,
}) {
  const rootScope = '<root>';
  final modelTable = {for (final model in spec.models) model.name: model};
  final usage = <String, Set<String>>{};
  final groupModelNames = <String, Set<String>>{
    for (final group in groupEndpoints.keys) group: <String>{},
  };

  void mark(String name, String scope) {
    usage.putIfAbsent(name, () => <String>{}).add(scope);
    if (scope != rootScope) {
      groupModelNames.putIfAbsent(scope, () => <String>{}).add(name);
    }
  }

  void collectType(TypeUsage? type, String scope, Set<String> visiting) {
    if (type == null) {
      return;
    }
    switch (type.kind) {
      case TypeUsageKind.named:
        final name = type.name;
        final modelDef = modelTable[name];
        if (modelDef == null) {
          return;
        }
        mark(name, scope);
        if (!visiting.add(name)) {
          return;
        }
        for (final field in modelDef.fields) {
          collectType(field.type, scope, visiting);
        }
        visiting.remove(name);
        break;
      case TypeUsageKind.list:
        collectType(type.elem, scope, visiting);
        break;
      case TypeUsageKind.map:
        collectType(type.value, scope, visiting);
        break;
      default:
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

  final sharedModelNames = <String>{};
  for (final model in spec.models) {
    final scopes = usage[model.name] ?? const <String>{};
    if (scopes.isEmpty || scopes.contains(rootScope) || scopes.length > 1) {
      sharedModelNames.add(model.name);
    }
  }

  for (final entry in groupModelNames.entries) {
    entry.value.removeAll(sharedModelNames);
  }

  return _ModelPlan(
    modelTable: modelTable,
    sharedModelNames: sharedModelNames,
    groupModelNames: groupModelNames,
  );
}

String _renderModels(
  List<_ModelClassPlan> modelPlans, {
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

  for (final modelPlan in modelPlans) {
    final modelDef = modelPlan.model;
    buffer
      ..writeln('class ${modelDef.name} {')
      ..writeln('  const ${modelDef.name}({');
    for (final field in modelDef.fields) {
      final prefix = field.required ? '    required this.' : '    this.';
      buffer.writeln('$prefix${_camelCase(field.name)},');
    }
    buffer
      ..writeln('  });')
      ..writeln();

    for (final field in modelDef.fields) {
      buffer.writeln(
        '  final ${_dartType(field.type)} ${_camelCase(field.name)};',
      );
    }

    buffer
      ..writeln()
      ..writeln(
        '  factory ${modelDef.name}.fromJson(Map<String, dynamic> json) => ${modelDef.name}(',
      );
    for (final field in modelDef.fields) {
      final fieldName = _camelCase(field.name);
      buffer.writeln(
        "    $fieldName: ${_decodeModelValue(field.type, "json['${field.key}']", "${modelDef.name}.${field.key}")},",
      );
    }
    buffer
      ..writeln('  );')
      ..writeln()
      ..writeln('  Map<String, dynamic> toJson() => {');
    for (final field in modelDef.fields) {
      final fieldName = _camelCase(field.name);
      if (field.omitEmpty && field.type.nullable) {
        final nonNullable = _nonNullableType(field.type);
        buffer.writeln(
          "    if ($fieldName != null) '${field.key}': ${_encodeModelValue(nonNullable, fieldName)},",
        );
      } else {
        buffer.writeln(
          "    '${field.key}': ${_encodeModelValue(field.type, fieldName)},",
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

TypeUsage _nonNullableType(TypeUsage type) => TypeUsage(
  kind: type.kind,
  name: type.name,
  nullable: false,
  elem: type.elem,
  key: type.key,
  value: type.value,
);

String _dartType(TypeUsage type, {bool forceOptional = false}) {
  final nullable = forceOptional || type.nullable;
  String base;
  switch (type.kind) {
    case TypeUsageKind.bool:
      base = 'bool';
    case TypeUsageKind.int:
      base = 'int';
    case TypeUsageKind.float:
      base = 'double';
    case TypeUsageKind.string:
    case TypeUsageKind.uuid:
      base = 'String';
    case TypeUsageKind.any:
      base = 'Object?';
    case TypeUsageKind.named:
      base = type.name;
    case TypeUsageKind.list:
      base = 'List<${_dartType(type.elem!)}>';
    case TypeUsageKind.map:
      base = 'Map<String, ${_dartType(type.value!)}>';
  }

  if (base.endsWith('?') || !nullable) {
    return base;
  }
  return '$base?';
}

String _decodeModelValue(TypeUsage type, String expr, String context) {
  if (type.nullable) {
    final nonNullable = TypeUsage(
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
    case TypeUsageKind.bool:
      return '$expr as bool';
    case TypeUsageKind.int:
      return '$expr as int';
    case TypeUsageKind.float:
      return '($expr as num).toDouble()';
    case TypeUsageKind.string:
    case TypeUsageKind.uuid:
      return '$expr as String';
    case TypeUsageKind.any:
      return expr;
    case TypeUsageKind.named:
      return '${type.name}.fromJson(expectJsonObject($expr, \'$context\'))';
    case TypeUsageKind.list:
      return '(expectJsonList($expr, \'$context\')).map((element) => ${_decodeCollectionItem(type.elem!, 'element', context)}).toList()';
    case TypeUsageKind.map:
      return "expectJsonObject($expr, '$context').map((key, value) => MapEntry(key, ${_decodeCollectionItem(type.value!, 'value', context)}))";
  }
}

String _decodeCollectionItem(TypeUsage type, String expr, String context) {
  if (type.nullable) {
    final nonNullable = TypeUsage(
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
    case TypeUsageKind.bool:
      return '$expr as bool';
    case TypeUsageKind.int:
      return '$expr as int';
    case TypeUsageKind.float:
      return '($expr as num).toDouble()';
    case TypeUsageKind.string:
    case TypeUsageKind.uuid:
      return '$expr as String';
    case TypeUsageKind.any:
      return expr;
    case TypeUsageKind.named:
      return '${type.name}.fromJson(expectJsonObject($expr, \'$context\'))';
    case TypeUsageKind.list:
      return '(expectJsonList($expr, \'$context\')).map((nested) => ${_decodeCollectionItem(type.elem!, 'nested', context)}).toList()';
    case TypeUsageKind.map:
      return "expectJsonObject($expr, '$context').map((key, value) => MapEntry(key, ${_decodeCollectionItem(type.value!, 'value', context)}))";
  }
}

String _encodeModelValue(TypeUsage type, String expr) {
  if (type.nullable) {
    final nonNullable = TypeUsage(
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
    case TypeUsageKind.named:
      return '$expr.toJson()';
    case TypeUsageKind.list:
      return '$expr.map((item) => ${_encodeCollectionItem(type.elem!, 'item')}).toList()';
    case TypeUsageKind.map:
      return '$expr.map((key, value) => MapEntry(key, ${_encodeCollectionItem(type.value!, 'value')}))';
    default:
      return expr;
  }
}

String _encodeCollectionItem(TypeUsage type, String expr) {
  if (type.nullable) {
    final nonNullable = TypeUsage(
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
    case TypeUsageKind.named:
      return '$expr.toJson()';
    case TypeUsageKind.list:
      return '$expr.map((nested) => ${_encodeCollectionItem(type.elem!, 'nested')}).toList()';
    case TypeUsageKind.map:
      return '$expr.map((key, value) => MapEntry(key, ${_encodeCollectionItem(type.value!, 'value')}))';
    default:
      return expr;
  }
}

String _decodeResponseValue(
  TypeUsage type,
  String expr,
  String statusExpr,
  String rawBodyExpr, {
  String transportExpr = '_transport',
}) {
  if (type.nullable) {
    final nonNullable = TypeUsage(
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
    case TypeUsageKind.bool:
      return "$transportExpr.expectBool($expr, $statusExpr, $rawBodyExpr, 'Expected response data to be a boolean.')";
    case TypeUsageKind.int:
      return "$transportExpr.expectInt($expr, $statusExpr, $rawBodyExpr, 'Expected response data to be an integer.')";
    case TypeUsageKind.float:
      return "$transportExpr.expectDouble($expr, $statusExpr, $rawBodyExpr, 'Expected response data to be a number.')";
    case TypeUsageKind.string:
    case TypeUsageKind.uuid:
      return "$transportExpr.expectString($expr, $statusExpr, $rawBodyExpr, 'Expected response data to be a string.')";
    case TypeUsageKind.any:
      return expr;
    case TypeUsageKind.named:
      return '${type.name}.fromJson($transportExpr.expectJsonObject($expr, $statusExpr, $rawBodyExpr, \'Expected response data to be a JSON object.\'))';
    case TypeUsageKind.list:
      return "($transportExpr.expectJsonList($expr, $statusExpr, $rawBodyExpr, 'Expected response data to be a JSON array.')).map((element) => ${_decodeResponseListItem(type.elem!, 'element', statusExpr, rawBodyExpr, transportExpr: transportExpr)}).toList()";
    case TypeUsageKind.map:
      return "$transportExpr.expectJsonObject($expr, $statusExpr, $rawBodyExpr, 'Expected response data to be a JSON object.').map((key, value) => MapEntry(key, ${_decodeResponseListItem(type.value!, 'value', statusExpr, rawBodyExpr, transportExpr: transportExpr)}))";
  }
}

String _decodeResponseListItem(
  TypeUsage type,
  String expr,
  String statusExpr,
  String rawBodyExpr, {
  String transportExpr = '_transport',
}) {
  if (type.nullable) {
    final nonNullable = TypeUsage(
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
    case TypeUsageKind.bool:
      return "$transportExpr.expectBool($expr, $statusExpr, $rawBodyExpr, 'Expected response item to be a boolean.')";
    case TypeUsageKind.int:
      return "$transportExpr.expectInt($expr, $statusExpr, $rawBodyExpr, 'Expected response item to be an integer.')";
    case TypeUsageKind.float:
      return "$transportExpr.expectDouble($expr, $statusExpr, $rawBodyExpr, 'Expected response item to be a number.')";
    case TypeUsageKind.string:
    case TypeUsageKind.uuid:
      return "$transportExpr.expectString($expr, $statusExpr, $rawBodyExpr, 'Expected response item to be a string.')";
    case TypeUsageKind.any:
      return expr;
    case TypeUsageKind.named:
      return '${type.name}.fromJson($transportExpr.expectJsonObject($expr, $statusExpr, $rawBodyExpr, \'Expected response item to be a JSON object.\'))';
    case TypeUsageKind.list:
      return "($transportExpr.expectJsonList($expr, $statusExpr, $rawBodyExpr, 'Expected response item to be a JSON array.')).map((nested) => ${_decodeResponseListItem(type.elem!, 'nested', statusExpr, rawBodyExpr, transportExpr: transportExpr)}).toList()";
    case TypeUsageKind.map:
      return "$transportExpr.expectJsonObject($expr, $statusExpr, $rawBodyExpr, 'Expected response item to be a JSON object.').map((key, value) => MapEntry(key, ${_decodeResponseListItem(type.value!, 'value', statusExpr, rawBodyExpr, transportExpr: transportExpr)}))";
  }
}

String _bodyToJson(TypeUsage type, String expr) {
  if (type.nullable) {
    final nonNullable = TypeUsage(
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
    case TypeUsageKind.named:
      return '$expr.toJson()';
    case TypeUsageKind.list:
      return '$expr.map((item) => ${_bodyCollectionItem(type.elem!, 'item')}).toList()';
    case TypeUsageKind.map:
      return '$expr.map((key, value) => MapEntry(key, ${_bodyCollectionItem(type.value!, 'value')}))';
    default:
      return expr;
  }
}

String _bodyCollectionItem(TypeUsage type, String expr) {
  if (type.nullable) {
    final nonNullable = TypeUsage(
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
    case TypeUsageKind.named:
      return '$expr.toJson()';
    case TypeUsageKind.list:
      return '$expr.map((nested) => ${_bodyCollectionItem(type.elem!, 'nested')}).toList()';
    case TypeUsageKind.map:
      return '$expr.map((key, value) => MapEntry(key, ${_bodyCollectionItem(type.value!, 'value')}))';
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

String _groupUniqueName(_GroupPlan plan) {
  return _groupDirectoryNameForSegments(plan.pathSegments);
}

String _groupDirectoryName(_GroupPlan plan) => _groupUniqueName(plan);

String _groupConfigClassName(_GroupPlan plan) =>
    '${_pascalCase(_groupUniqueName(plan))}Group';

String _groupClientClassName(_GroupPlan plan) =>
    '${_pascalCase(_groupUniqueName(plan))}GroupClient';

String _groupConfigClassNameForSegments(Iterable<String> pathSegments) =>
    '${_pascalCase(_groupDirectoryNameForSegments(pathSegments))}Group';

String _groupClientClassNameForSegments(Iterable<String> pathSegments) =>
    '${_pascalCase(_groupDirectoryNameForSegments(pathSegments))}GroupClient';

List<String> _childPathSegments(_GroupPlan plan, GroupSpec child) => [
  ...plan.pathSegments,
  child.name,
];

String _groupDirectoryNameForSegments(Iterable<String> pathSegments) {
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

String _headerProviderFieldName(HeaderSpec header) {
  return _camelCase(_identifierNameFromKey(header.key));
}

String _headerProviderStorageName(HeaderSpec header) {
  return '_${_headerProviderFieldName(header)}';
}

List<HeaderSpec> _headersFromGroups(List<GroupSpec> groups) {
  return _uniqueHeaders(groups.expand(_groupHeadersForGroup).toList());
}

List<HeaderSpec> _groupHeadersForGroup(GroupSpec group) => group.headers;

List<HeaderSpec> _uniqueHeaders(List<HeaderSpec> values) {
  final seen = <String>{};
  final result = <HeaderSpec>[];
  for (final value in values) {
    if (seen.add(value.key.trim().toLowerCase())) {
      result.add(value);
    }
  }
  return result;
}

bool _groupConfigHasRequiredValues(GroupSpec group) {
  return group.headers.isNotEmpty ||
      group.groups.any(_groupConfigHasRequiredValues);
}

String _renderGroupConfigParameter(_GroupPlan plan, {required String indent}) {
  final type = _groupConfigClassName(plan);
  final name = _groupPropertyName(plan.group);
  if (_groupConfigHasRequiredValues(plan.group)) {
    return '${indent}required $type $name,';
  }
  return '${indent}$type $name = const $type(),';
}

String _identifierNameFromKey(String key) {
  final normalized = key.replaceAll(RegExp(r'[^A-Za-z0-9]+'), '_');
  return normalized.replaceAll(RegExp(r'^_+|_+$'), '');
}
