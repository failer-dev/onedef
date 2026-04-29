import 'dart:convert';
import 'dart:io';

import 'package:onedef_dart_sdk_gen/onedef_dart_sdk_gen.dart';
import 'package:path/path.dart' as path;
import 'package:test/test.dart';

void main() {
  group('Spec conformance', () {
    for (final fixture in const [
      'simple.json',
      'grouped.json',
      'nested-groups.json',
      'custom-error.json',
    ]) {
      test('parses valid fixture $fixture', () {
        final spec = _readSpecFixture('valid', fixture);

        expect(spec.version, 'v1');
        expect(spec.types, isNotNull);
      });
    }

    final invalidCases = <String, String>{
      'duplicate-type.json': 'duplicate_type',
      'duplicate-header.json': 'duplicate_header',
      'unknown-type-ref.json': 'unknown_type_ref',
      'bad-path-param.json': 'path_param_mismatch',
      'bad-204-response-body.json': 'invalid_success_response',
    };

    for (final entry in invalidCases.entries) {
      test('rejects invalid fixture ${entry.key}', () {
        expect(
          () => _readSpecFixture('invalid', entry.key),
          throwsA(
            isA<SpecValidationException>().having(
              (error) => error.code,
              'code',
              entry.value,
            ),
          ),
        );
      });
    }

    test('normalizes group id, pathSegments, and default error', () {
      final spec = Spec.fromJson({
        'version': 'v1',
        'groups': [
          {
            'name': 'users',
            'endpoints': [
              {
                'name': 'DeleteUser',
                'method': 'DELETE',
                'path': '/users/{id}',
                'successStatus': 204,
                'request': {
                  'pathParams': [
                    {
                      'name': 'ID',
                      'wireName': 'id',
                      'type': {'kind': 'uuid'},
                      'required': true,
                    },
                  ],
                },
                'response': {'envelope': false},
              },
            ],
          },
        ],
        'types': [],
      });

      final group = spec.groups.single;
      expect(group.id, 'users');
      expect(group.pathSegments, ['users']);
      expect(group.endpoints.single.error.body.name, 'DefaultError');
    });

    test('rejects unknown type kind', () {
      expect(
        () => Spec.fromJson(
          jsonDecode('''
          {
            "version": "v1",
            "endpoints": [
              {
                "name": "GetClock",
                "method": "GET",
                "path": "/clock",
                "successStatus": 200,
                "request": {},
                "response": {
                  "envelope": true,
                  "body": { "kind": "datetime" }
                }
              }
            ],
            "types": []
          }
          ''') as Map<String, dynamic>,
        ),
        throwsA(
          isA<SpecValidationException>().having(
            (error) => error.code,
            'code',
            'invalid_type_ref',
          ),
        ),
      );
    });

    test('CLI exits non-zero for invalid IR', () async {
      final outDir = Directory.systemTemp.createTempSync(
        'onedef-invalid-ir-',
      );
      addTearDown(() => outDir.deleteSync(recursive: true));

      final result = await Process.run(Platform.resolvedExecutable, [
        'run',
        'bin/onedef_dart_sdk_gen.dart',
        'generate',
        '--input',
        path.join('..', '..', 'onedef_ir', 'fixtures', 'invalid',
            'duplicate-type.json'),
        '--out',
        outDir.path,
        '--package-name',
        'bad_api',
        '--core-path',
        '../sdk_core',
      ]);

      expect(result.exitCode, 65);
      expect(result.stderr as String, contains('duplicate_type'));
    });
  });
}

Spec _readSpecFixture(String kind, String name) {
  final file = File(path.join('..', '..', 'onedef_ir', 'fixtures', kind, name));
  return Spec.fromJson(
    jsonDecode(file.readAsStringSync()) as Map<String, dynamic>,
  );
}
