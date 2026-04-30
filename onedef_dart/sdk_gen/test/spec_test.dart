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
        expect(spec.models, isNotNull);
      });
    }

    test('parses group and default error', () {
      final spec = Spec.fromJson({
        'version': 'v1',
        'routes': {
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
                    'paths': [
                      {'name': 'ID', 'key': 'id', 'type': 'uuid'},
                    ],
                  },
                  'response': {'envelope': false},
                },
              ],
            },
          ],
        },
        'models': [],
      });

      final group = spec.routes.groups.single;
      expect(group.name, 'users');
      expect(group.endpoints.single.error.body.name, 'DefaultError');
    });

    test('parses readable type expressions into structural usage', () {
      final type = TypeUsage.fromJson('map<string, list<Booking?>>');

      expect(type.kind, TypeUsageKind.map);
      expect(type.key?.kind, TypeUsageKind.string);
      expect(type.value?.kind, TypeUsageKind.list);
      expect(type.value?.elem?.kind, TypeUsageKind.named);
      expect(type.value?.elem?.name, 'Booking');
      expect(type.value?.elem?.nullable, isTrue);
      expect(type.toJson(), 'map<string, list<Booking?>>');
    });
  });
}

Spec _readSpecFixture(String kind, String name) {
  final file = File(path.join('..', '..', 'onedef_ir', 'fixtures', kind, name));
  return Spec.fromJson(
    jsonDecode(file.readAsStringSync()) as Map<String, dynamic>,
  );
}
