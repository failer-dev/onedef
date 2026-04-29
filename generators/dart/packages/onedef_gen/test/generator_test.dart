import 'dart:convert';
import 'dart:io';

import 'package:onedef_gen/onedef_dart_generator.dart';
import 'package:path/path.dart' as path;
import 'package:test/test.dart';

void main() {
  test('renderPackage emits expected Dart SDK structure', () async {
    final fixture = File(
      path.join('test', 'fixtures', 'simple_spec.json'),
    ).readAsStringSync();
    final spec = Spec.fromJson(jsonDecode(fixture) as Map<String, dynamic>);

    final files = renderPackage(
      spec: spec,
      packageName: 'user_api',
      corePath: '../../onedef_core',
    );

    expect(files.containsKey('pubspec.yaml'), isTrue);
    expect(files.containsKey('lib/user_api.dart'), isTrue);
    expect(files.containsKey('lib/src/client.dart'), isTrue);
    expect(files.containsKey('lib/src/groups/user.dart'), isTrue);
    expect(files.containsKey('lib/src/models.dart'), isTrue);

    final client = files['lib/src/client.dart']!;
    expect(client, contains('class ApiClient'));
    expect(client, contains('final UserGroup user;'));
    expect(
        client, contains('Future<Result<HealthStatus, DefaultError>> health('));
    expect(
      client,
      contains("import 'package:onedef_core/onedef_core.dart';"),
    );

    final group = files['lib/src/groups/user.dart']!;
    expect(group, contains('class UserGroup'));
    expect(group, contains('Future<Result<User, DefaultError>> createUser('));
    expect(group, contains('Future<Result<void, DefaultError>> deleteUser('));
    expect(
      group,
      contains("import 'package:onedef_core/onedef_core.dart';"),
    );

    final models = files['lib/src/models.dart']!;
    expect(
      models,
      contains("import 'package:onedef_core/onedef_core.dart';"),
    );
    expect(models, contains('class CreateUserRequest'));
    expect(models, contains('class User'));
    expect(models, isNot(contains('Map<String, dynamic> _expectObject(')));
    expect(models, isNot(contains('List<dynamic> _expectList(')));

    final barrel = files['lib/user_api.dart']!;
    expect(
      barrel,
      contains("export 'package:onedef_core/onedef_core.dart';"),
    );
    expect(barrel, contains("export 'src/groups/user.dart';"));

    final pubspec = files['pubspec.yaml']!;
    expect(pubspec, contains('onedef_core:'));
    expect(pubspec, contains('path: ../../onedef_core'));
  });

  test('writePackage writes package files to disk', () async {
    final fixture = File(
      path.join('test', 'fixtures', 'simple_spec.json'),
    ).readAsStringSync();
    final spec = Spec.fromJson(jsonDecode(fixture) as Map<String, dynamic>);

    final tempDir = Directory.systemTemp.createTempSync(
      'onedef-dart-generator-',
    );
    addTearDown(() => tempDir.deleteSync(recursive: true));

    final outDir = path.join(tempDir.path, 'user_api');
    await writePackage(
      spec: spec,
      packageName: 'user_api',
      outputDir: outDir,
      corePath: '../../onedef_core',
    );

    expect(File(path.join(outDir, 'pubspec.yaml')).existsSync(), isTrue);
    expect(
      File(path.join(outDir, 'lib', 'user_api.dart')).existsSync(),
      isTrue,
    );
    expect(
      File(path.join(outDir, 'lib', 'src', 'groups', 'user.dart')).existsSync(),
      isTrue,
    );
    expect(
      File(path.join(outDir, 'lib', 'src', 'core')).existsSync(),
      isFalse,
    );
  });

  test('renderPackage uses sdkName override for method names', () async {
    final spec = Spec(
      version: 'v1',
      naming: NamingSpec(initialisms: const []),
      endpoints: const [],
      groups: [
        GroupSpec(
          id: 'customer',
          name: 'customer',
          pathSegments: const ['customer'],
          requiredHeaders: const [],
          endpoints: [
            Endpoint(
              name: 'GetUser',
              sdkName: 'get',
              method: 'GET',
              path: '/customers',
              successStatus: 204,
              group: '',
              requiredHeaders: const [],
              request: RequestSpec(
                pathParams: const [],
                queryParams: const [],
                headerParams: const [],
                body: null,
              ),
              response: ResponseSpec(envelope: false, body: null),
            ),
            Endpoint(
              name: 'ListUsers',
              method: 'GET',
              path: '/customers',
              successStatus: 204,
              group: '',
              requiredHeaders: const [],
              request: RequestSpec(
                pathParams: const [],
                queryParams: const [],
                headerParams: const [],
                body: null,
              ),
              response: ResponseSpec(envelope: false, body: null),
            ),
          ],
          groups: const [],
        ),
      ],
      types: const [],
    );

    final files = renderPackage(
      spec: spec,
      packageName: 'customer_api',
      corePath: '../../onedef_core',
    );

    final group = files['lib/src/groups/customer.dart']!;
    expect(group, contains('Future<Result<void, DefaultError>> get('));
    expect(
        group, isNot(contains('Future<Result<void, DefaultError>> getUser(')));
    expect(group, contains('Future<Result<void, DefaultError>> listUsers('));
  });

  test('renderPackage does not apply undeclared initialisms', () async {
    final spec = Spec(
      version: 'v1',
      naming: NamingSpec(initialisms: const []),
      endpoints: [
        Endpoint(
          name: 'FindByID',
          method: 'GET',
          path: '/bookings/{id}',
          successStatus: 204,
          group: 'booking',
          requiredHeaders: const [],
          request: RequestSpec(
            pathParams: [
              Parameter(
                name: 'ID',
                wireName: 'id',
                type: TypeRef(kind: 'string', name: '', nullable: false),
                required: true,
              ),
            ],
            queryParams: const [],
            headerParams: const [],
            body: null,
          ),
          response: ResponseSpec(envelope: false, body: null),
        ),
      ],
      groups: const [],
      types: const [],
    );

    final files = renderPackage(
      spec: spec,
      packageName: 'booking_api',
      corePath: '../../onedef_core',
    );

    final group = files['lib/src/groups/booking.dart']!;
    expect(group, contains('Future<Result<void, DefaultError>> findByID('));
    expect(group, isNot(contains('findById')));
  });

  test('renderPackage applies only IR-declared initialisms', () async {
    final stringType = TypeRef(kind: 'string', name: '', nullable: false);
    final resourceType =
        TypeRef(kind: 'named', name: 'Resource', nullable: false);
    final spec = Spec(
      version: 'v1',
      naming: NamingSpec(initialisms: const ['API', 'URL', 'OAuth']),
      endpoints: [
        Endpoint(
          name: 'GetAPIURL',
          method: 'GET',
          path: '/resources/{api_url}',
          successStatus: 200,
          group: 'resource',
          requiredHeaders: const [],
          request: RequestSpec(
            pathParams: [
              Parameter(
                name: 'APIURL',
                wireName: 'api_url',
                type: stringType,
                required: true,
              ),
            ],
            queryParams: [
              Parameter(
                name: 'OAuthToken',
                wireName: 'oauth_token',
                type: stringType,
                required: false,
              ),
            ],
            headerParams: const [],
            body: null,
          ),
          response: ResponseSpec(envelope: true, body: resourceType),
        ),
      ],
      groups: const [],
      types: [
        TypeDef(
          name: 'Resource',
          kind: 'object',
          fields: [
            FieldDef(
              name: 'APIURL',
              wireName: 'api_url',
              type: stringType,
              required: true,
              nullable: false,
              omitEmpty: false,
            ),
            FieldDef(
              name: 'OAuthToken',
              wireName: 'oauth_token',
              type: stringType,
              required: true,
              nullable: false,
              omitEmpty: false,
            ),
          ],
        ),
      ],
    );

    final files = renderPackage(
      spec: spec,
      packageName: 'resource_api',
      corePath: '../../onedef_core',
    );

    final group = files['lib/src/groups/resource.dart']!;
    expect(
        group, contains('Future<Result<Resource, DefaultError>> getApiUrl('));
    expect(group, contains('required String apiUrl'));
    expect(group, contains('String? oauthToken'));
    expect(group, isNot(contains('apiurl')));
    expect(group, isNot(contains('oAuthToken')));

    final models = files['lib/src/models.dart']!;
    expect(models, contains('final String apiUrl;'));
    expect(models, contains('final String oauthToken;'));
  });

  test('renderPackage emits grouped SDK structure', () async {
    final fixture = File(
      path.join('test', 'fixtures', 'grouped_spec.json'),
    ).readAsStringSync();
    final spec = Spec.fromJson(jsonDecode(fixture) as Map<String, dynamic>);

    final files = renderPackage(
      spec: spec,
      packageName: 'user_api',
      corePath: '../../onedef_core',
    );

    expect(files.containsKey('pubspec.yaml'), isTrue);
    expect(files.containsKey('lib/user_api.dart'), isTrue);
    expect(files.containsKey('lib/src/client.dart'), isTrue);
    expect(files.containsKey('lib/src/groups/user.dart'), isTrue);
    expect(files.containsKey('lib/src/strategy.dart'), isFalse);

    final client = files['lib/src/client.dart']!;
    expect(client, contains('class ApiClient'));
    expect(client, contains('final UserGroup user;'));
    expect(client, contains('required HeaderValueProvider authorization,'));
    expect(client, contains("import 'groups/user.dart';"));

    final providers = files['lib/src/providers.dart']!;
    expect(providers, contains('typedef HeaderValueProvider'));

    final group = files['lib/src/groups/user.dart']!;
    expect(group, isNot(contains('ScopeStrategy')));
    expect(group, contains('class UserGroup'));
    expect(group, contains('Future<Result<User, DefaultError>> createUser('));
    expect(
      group,
      contains("headers['Authorization'] = await _authorization();"),
    );
    expect(group, contains('required String idempotencyKey'));
    expect(group, contains("headers['Idempotency-Key'] = idempotencyKey;"));
    expect(group, contains('String? requestId'));
    expect(
      group,
      contains("if (requestId != null) headers['X-Request-Id'] = requestId;"),
    );
  });

  test('renderPackage emits unique nested group files and inherited providers',
      () async {
    final fixture = File(
      path.join('test', 'fixtures', 'grouped_nested_spec.json'),
    ).readAsStringSync();
    final spec = Spec.fromJson(jsonDecode(fixture) as Map<String, dynamic>);

    final files = renderPackage(
      spec: spec,
      packageName: 'user_api',
      corePath: '../../onedef_core',
    );

    expect(files.containsKey('pubspec.yaml'), isTrue);
    expect(files.containsKey('lib/user_api.dart'), isTrue);
    expect(files.containsKey('lib/src/client.dart'), isTrue);
    expect(files.containsKey('lib/src/groups/branch.dart'), isTrue);
    expect(files.containsKey('lib/src/groups/branch_booking.dart'), isTrue);
    expect(files.containsKey('lib/src/groups/customer.dart'), isTrue);
    expect(files.containsKey('lib/src/groups/customer_booking.dart'), isTrue);

    final client = files['lib/src/client.dart']!;
    expect(client, isNot(contains('ScopeStrategy')));
    expect(client, contains('class ApiClient'));
    expect(client, contains('final BranchGroup branch;'));
    expect(client, contains('final CustomerGroup customer;'));
    expect(client, contains('required HeaderValueProvider authorization,'));
    expect(client, contains('required HeaderValueProvider xBranchId,'));
    expect(client, contains('required HeaderValueProvider xBookingScope,'));
    expect(client, contains('required HeaderValueProvider xCustomerId,'));

    final branch = files['lib/src/groups/branch.dart']!;
    expect(branch, isNot(contains('ScopeStrategy')));
    expect(branch, contains("import 'branch_booking.dart';"));
    expect(branch, contains('class BranchGroup'));
    expect(branch, contains('required HeaderValueProvider xBookingScope,'));
    expect(branch, contains('BranchBookingGroup get booking'));
    expect(branch, contains('authorization: _authorization,'));
    expect(branch, contains('xBranchId: _xBranchId,'));
    expect(branch, contains('xBookingScope: _xBookingScope,'));

    final branchBooking = files['lib/src/groups/branch_booking.dart']!;
    expect(branchBooking, isNot(contains('ScopeStrategy')));
    expect(branchBooking, contains('class BranchBookingGroup'));
    expect(
        branchBooking, contains('final HeaderValueProvider _authorization;'));
    expect(branchBooking, contains('final HeaderValueProvider _xBranchId;'));
    expect(
        branchBooking, contains('final HeaderValueProvider _xBookingScope;'));
    expect(
      branchBooking,
      contains("headers['Authorization'] = await _authorization();"),
    );
    expect(
      branchBooking,
      contains("headers['X-Branch-Id'] = await _xBranchId();"),
    );
    expect(
      branchBooking,
      contains("headers['X-Booking-Scope'] = await _xBookingScope();"),
    );
    expect(branchBooking, contains('required String idempotencyKey'));
    expect(
      branchBooking,
      contains("headers['Idempotency-Key'] = idempotencyKey;"),
    );

    final customerBooking = files['lib/src/groups/customer_booking.dart']!;
    expect(
        customerBooking, contains('final HeaderValueProvider _authorization;'));
    expect(
        customerBooking, contains('final HeaderValueProvider _xCustomerId;'));
    expect(customerBooking,
        contains('Future<Result<Booking, DefaultError>> findById('));
    expect(customerBooking, isNot(contains('findByID')));
    expect(
      customerBooking,
      contains("headers['Authorization'] = await _authorization();"),
    );
    expect(
      customerBooking,
      contains("headers['X-Customer-Id'] = await _xCustomerId();"),
    );
  });
}
