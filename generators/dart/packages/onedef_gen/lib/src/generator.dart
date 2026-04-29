import 'dart:io';

import 'package:path/path.dart' as path;

import 'spec.dart';

Future<void> writePackage({
  required Spec spec,
  required String packageName,
  required String outputDir,
  required String corePath,
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
}

Map<String, String> renderPackage({
  required Spec spec,
  required String packageName,
  required String corePath,
}) {
  return _withIdentifierInitialisms(spec.naming.initialisms, () {
    if (spec.groups.isNotEmpty) {
      return _renderGroupedPackage(spec, packageName, corePath);
    }

    final groups = _collectGroups(spec.endpoints);
    return <String, String>{
      'pubspec.yaml': _renderPubspec(
        packageName,
        corePath: corePath,
      ),
      'lib/$packageName.dart': _renderBarrel(packageName, groups.keys),
      'lib/src/client.dart': _renderClient(spec, groups),
      'lib/src/models.dart': _renderModels(spec),
      'lib/src/providers.dart': _renderProvidersFile(),
      for (final entry in groups.entries)
        'lib/src/groups/${entry.key}.dart':
            _renderGroup(entry.key, entry.value),
    };
  });
}

Map<String, String> _renderGroupedPackage(
  Spec spec,
  String packageName,
  String corePath,
) {
  final groupEntries = _flattenGroupEntries(spec.groups);
  return <String, String>{
    'pubspec.yaml': _renderPubspec(
      packageName,
      corePath: corePath,
    ),
    'lib/$packageName.dart': _renderGroupedBarrel(groupEntries),
    'lib/src/client.dart': _renderGroupedClient(spec),
    'lib/src/models.dart': _renderModels(spec),
    'lib/src/providers.dart': _renderProvidersFile(),
    for (final entry in groupEntries)
      'lib/src/groups/${_groupFileName(entry.group)}': _renderGroupFile(
        entry.group,
        ancestors: entry.ancestors,
      ),
  };
}

String _renderProvidersFile() {
  return 'typedef HeaderValueProvider = Future<String> Function();\n';
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
environment:
  sdk: ^3.0.0
dependencies:
  http: ^1.2.0
  onedef_core:
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
    ..writeln("export 'package:onedef_core/onedef_core.dart';")
    ..writeln("export 'src/providers.dart';");
  for (final group in groups) {
    buffer.writeln("export 'src/groups/$group.dart';");
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
    ..writeln("export 'package:onedef_core/onedef_core.dart';");
  for (final entry in groupEntries) {
    buffer.writeln("export 'src/groups/${_groupFileName(entry.group)}';");
  }
  return buffer.toString();
}

String _renderGroupedClient(Spec spec) {
  final rootEndpoints = spec.endpoints
      .where((endpoint) => endpoint.group.isEmpty)
      .toList(growable: false);
  final allHeaders = _uniqueStrings(
    spec.groups
        .expand((group) => _headersForGroupTree(group, ancestors: const []))
        .toList(),
  );

  final buffer = StringBuffer()
    ..writeln('// ignore_for_file: unused_field, unused_import')
    ..writeln()
    ..writeln("import 'package:http/http.dart' as http;")
    ..writeln("import 'package:onedef_core/onedef_core.dart';")
    ..writeln("import 'providers.dart';")
    ..writeln("export 'providers.dart';");
  if (rootEndpoints.isNotEmpty) {
    buffer.writeln("import 'models.dart';");
  }
  for (final group in spec.groups) {
    buffer.writeln("import 'groups/${_groupFileName(group)}';");
  }

  buffer
    ..writeln()
    ..writeln('class ApiClient {')
    ..writeln('  factory ApiClient({')
    ..writeln('    required String baseUrl,');
  for (final header in allHeaders) {
    buffer.writeln(
      '    required HeaderValueProvider ${_headerProviderFieldName(header)},',
    );
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
      buffer.writeln('        $providerName: $providerName,');
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

String _renderGroupFile(
  GroupSpec group, {
  required List<GroupSpec> ancestors,
}) {
  final constructorHeaders = _headersForGroupTree(group, ancestors: ancestors);
  final buffer = StringBuffer()
    ..writeln('// ignore_for_file: unused_import')
    ..writeln()
    ..writeln("import 'package:onedef_core/onedef_core.dart';")
    ..writeln("import '../models.dart';")
    ..writeln("import '../providers.dart';");
  for (final child in group.groups) {
    buffer.writeln("import '${_groupFileName(child)}';");
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
        '    required HeaderValueProvider ${_headerProviderFieldName(header)},',
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
      '  final HeaderValueProvider ${_headerProviderStorageName(header)};',
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
        (endpoint) => _renderGroupedMethod(
          endpoint,
          group,
          ancestors,
          indent: '  ',
        ),
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
    ..writeln("import 'package:onedef_core/onedef_core.dart';")
    ..writeln("export 'providers.dart';");
  if (hasRootEndpoints) {
    buffer.writeln("import 'models.dart';");
  }
  for (final group in groups.keys) {
    buffer.writeln("import 'groups/$group.dart';");
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
    ..writeln("import 'package:onedef_core/onedef_core.dart';")
    ..writeln("import '../models.dart';")
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
  buffer.writeln('$indent  try {');
  final bodyIndent = '$indent  ';
  final innerIndent = '$indent    ';

  if (endpoint.request.headerParams.isNotEmpty) {
    buffer.writeln('${innerIndent}final headers = <String, String>{};');
    for (final parameter in endpoint.request.headerParams) {
      final name = _camelCase(parameter.name);
      if (parameter.required) {
        buffer.writeln(
          "${innerIndent}headers['${parameter.wireName}'] = $name;",
        );
      } else {
        buffer.writeln(
          "${innerIndent}if ($name != null) headers['${parameter.wireName}'] = $name;",
        );
      }
    }
  }

  if (endpoint.request.pathParams.isNotEmpty) {
    buffer.writeln(
      "${innerIndent}final pathParameters = <String, Object?>{${_pathParameterMap(endpoint.request.pathParams)}};",
    );
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

  final bodyArgument = endpoint.request.body == null
      ? ''
      : 'body: ${_bodyToJson(endpoint.request.body!, 'body')}, ';
  final headersArgument =
      endpoint.request.headerParams.isNotEmpty ? 'headers: headers, ' : '';
  buffer.writeln(
    "${innerIndent}final response = await _transport.request(method: '${endpoint.method}', path: '${_pathTemplate(endpoint.path)}', queryParameters: queryParameters, ${headersArgument}${bodyArgument});",
  );
  buffer.writeln(
    '${innerIndent}if (response.statusCode != ${endpoint.successStatus}) {',
  );
  buffer.writeln(
    "${innerIndent}  final errorJson = _transport.parseJsonBody(response, 'Expected a JSON error response body.');",
  );
  buffer.writeln(
    '${innerIndent}  final errorData = _transport.decodeResponseValue<$errorType>(response.statusCode, response.body, () => ${_decodeResponseValue(endpoint.error.body, "errorJson", "response.statusCode", "response.body")});',
  );
  buffer.writeln(
    '${innerIndent}  return ApiException<$returnType, $errorType>(',
  );
  buffer.writeln('${innerIndent}    statusCode: response.statusCode,');
  buffer.writeln('${innerIndent}    data: errorData,');
  buffer.writeln('${innerIndent}    rawBody: response.body,');
  buffer.writeln('${innerIndent}    stackTrace: StackTrace.current,');
  buffer.writeln('${innerIndent}  );');
  buffer.writeln('${innerIndent}}');

  if (endpoint.response.body == null) {
    final metadata = _successMetadata(endpoint.successStatus);
    buffer.writeln(
      "${innerIndent}return const Success<void, $errorType>(SuccessResponse<void>(status: ${endpoint.successStatus}, code: '${metadata.code}', title: '${metadata.title}', message: 'success', data: null));",
    );
    buffer.write(_renderResultCatchBlocks(returnType, errorType, bodyIndent));
    buffer.writeln('$indent}');
    return buffer.toString();
  }

  buffer.writeln(
    '${innerIndent}final envelope = _transport.parseSuccessEnvelope(response);',
  );
  buffer.writeln(
    '${innerIndent}final data = _transport.decodeResponseValue<$returnType>(response.statusCode, envelope.rawBody, () => ${_decodeResponseValue(endpoint.response.body!, "envelope.data", "response.statusCode", "envelope.rawBody")});',
  );
  buffer.writeln('${innerIndent}return Success<$returnType, $errorType>(');
  buffer.writeln('${innerIndent}  SuccessResponse<$returnType>(');
  buffer.writeln('${innerIndent}    status: response.statusCode,');
  buffer.writeln('${innerIndent}    code: envelope.code,');
  buffer.writeln('${innerIndent}    title: envelope.title,');
  buffer.writeln('${innerIndent}    message: envelope.message,');
  buffer.writeln('${innerIndent}    data: data,');
  buffer.writeln('${innerIndent}  ),');
  buffer.writeln('${innerIndent});');
  buffer.write(_renderResultCatchBlocks(returnType, errorType, bodyIndent));
  buffer.writeln('$indent}');
  return buffer.toString();
}

String _renderGroupedMethod(
  Endpoint endpoint,
  GroupSpec group,
  List<GroupSpec> ancestors, {
  required String indent,
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
  buffer.writeln('$indent  try {');
  buffer.writeln('$indent    final transport = _transport;');
  final bodyIndent = '$indent  ';
  final innerIndent = '$indent    ';

  final headerAncestors = ancestors
      .where((ancestor) => ancestor.requiredHeaders.isNotEmpty)
      .toList();
  final hasProviderHeaders =
      headerAncestors.isNotEmpty || group.requiredHeaders.isNotEmpty;

  if (hasProviderHeaders || endpoint.request.headerParams.isNotEmpty) {
    buffer.writeln('${innerIndent}final headers = <String, String>{};');
    for (final ancestor in headerAncestors) {
      for (final header in ancestor.requiredHeaders) {
        buffer.writeln(
          "${innerIndent}headers['$header'] = await ${_headerProviderStorageName(header)}();",
        );
      }
    }
    for (final header in group.requiredHeaders) {
      buffer.writeln(
        "${innerIndent}headers['$header'] = await ${_headerProviderStorageName(header)}();",
      );
    }
    for (final parameter in endpoint.request.headerParams) {
      final name = _camelCase(parameter.name);
      if (parameter.required) {
        buffer.writeln(
          "${innerIndent}headers['${parameter.wireName}'] = $name;",
        );
      } else {
        buffer.writeln(
          "${innerIndent}if ($name != null) headers['${parameter.wireName}'] = $name;",
        );
      }
    }
  }

  if (endpoint.request.pathParams.isNotEmpty) {
    buffer.writeln(
      "${innerIndent}final pathParameters = <String, Object?>{${_pathParameterMap(endpoint.request.pathParams)}};",
    );
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

  final bodyArgument = endpoint.request.body == null
      ? ''
      : 'body: ${_bodyToJson(endpoint.request.body!, 'body')}, ';
  final headersArgument =
      (hasProviderHeaders || endpoint.request.headerParams.isNotEmpty)
          ? 'headers: headers, '
          : '';
  buffer.writeln(
    "${innerIndent}final response = await transport.request(method: '${endpoint.method}', path: '${_pathTemplate(endpoint.path)}', queryParameters: queryParameters, ${headersArgument}${bodyArgument});",
  );
  buffer.writeln(
    '${innerIndent}if (response.statusCode != ${endpoint.successStatus}) {',
  );
  buffer.writeln(
    "${innerIndent}  final errorJson = transport.parseJsonBody(response, 'Expected a JSON error response body.');",
  );
  buffer.writeln(
    '${innerIndent}  final errorData = transport.decodeResponseValue<$errorType>(response.statusCode, response.body, () => ${_decodeResponseValue(endpoint.error.body, "errorJson", "response.statusCode", "response.body", transportExpr: "transport")});',
  );
  buffer.writeln(
    '${innerIndent}  return ApiException<$returnType, $errorType>(',
  );
  buffer.writeln('${innerIndent}    statusCode: response.statusCode,');
  buffer.writeln('${innerIndent}    data: errorData,');
  buffer.writeln('${innerIndent}    rawBody: response.body,');
  buffer.writeln('${innerIndent}    stackTrace: StackTrace.current,');
  buffer.writeln('${innerIndent}  );');
  buffer.writeln('${innerIndent}}');

  if (endpoint.response.body == null) {
    final metadata = _successMetadata(endpoint.successStatus);
    buffer.writeln(
      "${innerIndent}return const Success<void, $errorType>(SuccessResponse<void>(status: ${endpoint.successStatus}, code: '${metadata.code}', title: '${metadata.title}', message: 'success', data: null));",
    );
    buffer.write(_renderResultCatchBlocks(returnType, errorType, bodyIndent));
    buffer.writeln('$indent}');
    return buffer.toString();
  }

  buffer.writeln(
    '${innerIndent}final envelope = transport.parseSuccessEnvelope(response);',
  );
  buffer.writeln(
    '${innerIndent}final data = transport.decodeResponseValue<$returnType>(response.statusCode, envelope.rawBody, () => ${_decodeResponseValue(endpoint.response.body!, "envelope.data", "response.statusCode", "envelope.rawBody", transportExpr: "transport")});',
  );
  buffer.writeln('${innerIndent}return Success<$returnType, $errorType>(');
  buffer.writeln('${innerIndent}  SuccessResponse<$returnType>(');
  buffer.writeln('${innerIndent}    status: response.statusCode,');
  buffer.writeln('${innerIndent}    code: envelope.code,');
  buffer.writeln('${innerIndent}    title: envelope.title,');
  buffer.writeln('${innerIndent}    message: envelope.message,');
  buffer.writeln('${innerIndent}    data: data,');
  buffer.writeln('${innerIndent}  ),');
  buffer.writeln('${innerIndent});');
  buffer.write(_renderResultCatchBlocks(returnType, errorType, bodyIndent));
  buffer.writeln('$indent}');
  return buffer.toString();
}

String _renderResultCatchBlocks(
    String successType, String errorType, String indent) {
  final buffer = StringBuffer()
    ..writeln('${indent}} on ApiNetworkException catch (e) {')
    ..writeln(
        '${indent}  return ApiNetworkException<$successType, $errorType>(')
    ..writeln('${indent}    failureKind: e.failureKind,')
    ..writeln('${indent}    cause: e.cause,')
    ..writeln('${indent}    stackTrace: e.stackTrace,')
    ..writeln('${indent}  );')
    ..writeln('${indent}} on ApiContractViolationException catch (e) {')
    ..writeln(
      '${indent}  return ApiContractViolationException<$successType, $errorType>(',
    )
    ..writeln('${indent}    statusCode: e.statusCode,')
    ..writeln('${indent}    rawBody: e.rawBody,')
    ..writeln('${indent}    cause: e.cause,')
    ..writeln('${indent}    stackTrace: e.stackTrace,')
    ..writeln('${indent}  );')
    ..writeln('${indent}}');
  return buffer.toString();
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

String _renderModels(Spec spec) {
  final buffer = StringBuffer()
    ..writeln('// ignore_for_file: unused_element, unused_import')
    ..writeln()
    ..writeln("import 'package:onedef_core/onedef_core.dart';")
    ..writeln();

  for (final typeDef in spec.types) {
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
        buffer.writeln(
          "    if ($fieldName != null) '${field.wireName}': ${_encodeModelValue(field.type, fieldName)},",
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

({String code, String title}) _successMetadata(int status) {
  const titles = <int, String>{
    200: 'OK',
    201: 'Created',
    202: 'Accepted',
    204: 'No Content',
  };
  final title = titles[status] ?? 'Success';
  return (code: _snakeCase(title), title: title);
}

String _snakeCase(String value) {
  final buffer = StringBuffer();
  var lastWasUnderscore = false;
  for (final codeUnit in value.codeUnits) {
    final char = String.fromCharCode(codeUnit);
    final isLetter = RegExp(r'[A-Za-z0-9]').hasMatch(char);
    if (isLetter) {
      buffer.write(char.toLowerCase());
      lastWasUnderscore = false;
    } else if (!lastWasUnderscore && buffer.isNotEmpty) {
      buffer.write('_');
      lastWasUnderscore = true;
    }
  }
  final result = buffer.toString().replaceAll(RegExp(r'^_+|_+$'), '');
  return result.isEmpty ? 'success' : result;
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

  bool isInitialism(String value) => initialisms
      .any((initialism) => initialism.toUpperCase() == value.toUpperCase());
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

String _groupFileName(GroupSpec group) {
  return '${_groupUniqueName(group)}.dart';
}

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

String _headerProviderFieldName(String header) {
  return _camelCase(_sanitizeIdentifierSegment(header.replaceAll('-', '_')));
}

String _headerProviderStorageName(String header) {
  return '_${_headerProviderFieldName(header)}';
}

List<String> _headersFromGroups(List<GroupSpec> groups) {
  return _uniqueStrings(
    groups.expand((group) => group.requiredHeaders).toList(),
  );
}

List<String> _headersForGroupTree(
  GroupSpec group, {
  required List<GroupSpec> ancestors,
}) {
  final headers = <String>[
    ..._headersFromGroups(ancestors),
    ...group.requiredHeaders,
  ];
  for (final child in group.groups) {
    headers.addAll(
      _headersForGroupTree(
        child,
        ancestors: <GroupSpec>[...ancestors, group],
      ),
    );
  }
  return _uniqueStrings(headers);
}

List<String> _uniqueStrings(List<String> values) {
  final seen = <String>{};
  final result = <String>[];
  for (final value in values) {
    if (seen.add(value)) {
      result.add(value);
    }
  }
  return result;
}
