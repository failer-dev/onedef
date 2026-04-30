import 'dart:convert';
import 'dart:io';

import 'package:onedef_dart_sdk_gen/onedef_dart_sdk_gen.dart';
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
      corePath: '../sdk_core',
    );

    expect(files.containsKey('pubspec.yaml'), isTrue);
    expect(files.containsKey('lib/user_api.dart'), isTrue);
    expect(files.containsKey('lib/src/client.dart'), isTrue);
    expect(files.containsKey('lib/src/groups/user/client.dart'), isTrue);
    expect(files.containsKey('lib/src/groups/user/models.dart'), isTrue);
    expect(files.containsKey('lib/src/models.dart'), isTrue);

    final client = files['lib/src/client.dart']!;
    expect(client, contains('class ApiClient'));
    expect(client, contains('UserGroup user = const UserGroup(),'));
    expect(client, contains('final UserGroupClient user;'));
    expect(
      client,
      contains('Future<Result<HealthStatus, DefaultError>> health('),
    );
    expect(
      client,
      contains(
        "import 'package:onedef_dart_sdk_core/onedef_dart_sdk_core.dart';",
      ),
    );

    final group = files['lib/src/groups/user/client.dart']!;
    expect(group, contains("import '../../models.dart';"));
    expect(group, contains("import 'models.dart';"));
    expect(group, contains('class UserGroup'));
    expect(group, contains('class UserGroupClient'));
    expect(group, contains('Future<Result<User, DefaultError>> createUser('));
    expect(group, contains('Future<Result<void, DefaultError>> deleteUser('));
    expect(
      group,
      contains('return transport.requestResult<User, DefaultError>('),
    );
    expect(
      group,
      contains(
        '    final transport = _transport;\n'
        '\n'
        '    const queryParameters = <String, String>{};\n'
        '\n'
        '    return transport.requestResult<User, DefaultError>(',
      ),
    );
    expect(group, contains('method: HttpMethod.post,'));
    expect(
      group,
      contains('return transport.requestResult<void, DefaultError>('),
    );
    expect(
      group,
      contains(
        "    final pathParameters = <String, Object?>{'id': id};\n"
        '\n'
        '    const queryParameters = <String, String>{};\n'
        '\n'
        '    return transport.requestResult<void, DefaultError>(',
      ),
    );
    expect(group, contains('method: HttpMethod.delete,'));
    expect(group, isNot(contains('ApiException(')));
    expect(group, isNot(contains('parseSuccessEnvelope(')));
    expect(group, isNot(contains('} on ApiNetworkException catch')));
    expect(
      group,
      contains(
        "import 'package:onedef_dart_sdk_core/onedef_dart_sdk_core.dart';",
      ),
    );

    final models = files['lib/src/models.dart']!;
    expect(
      models,
      contains(
        "import 'package:onedef_dart_sdk_core/onedef_dart_sdk_core.dart';",
      ),
    );
    expect(models, contains('class HealthStatus'));
    expect(models, isNot(contains('class CreateUserRequest')));
    expect(models, isNot(contains('class User')));
    expect(models, isNot(contains('Map<String, dynamic> _expectObject(')));
    expect(models, isNot(contains('List<dynamic> _expectList(')));

    final groupModels = files['lib/src/groups/user/models.dart']!;
    expect(groupModels, contains('class CreateUserRequest'));
    expect(groupModels, contains('class User'));

    final barrel = files['lib/user_api.dart']!;
    expect(
      barrel,
      contains(
        "export 'package:onedef_dart_sdk_core/onedef_dart_sdk_core.dart';",
      ),
    );
    expect(barrel, contains("export 'src/groups/user/client.dart';"));
    expect(barrel, contains("export 'src/groups/user/models.dart';"));

    final pubspec = files['pubspec.yaml']!;
    expect(pubspec, contains('publish_to: none'));
    expect(pubspec, contains('onedef_dart_sdk_core:'));
    expect(pubspec, contains('path: ../sdk_core'));
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
      corePath: '../sdk_core',
    );

    expect(File(path.join(outDir, 'pubspec.yaml')).existsSync(), isTrue);
    expect(
      File(path.join(outDir, 'lib', 'user_api.dart')).existsSync(),
      isTrue,
    );
    expect(
      File(
        path.join(outDir, 'lib', 'src', 'groups', 'user', 'client.dart'),
      ).existsSync(),
      isTrue,
    );
    expect(
      File(
        path.join(outDir, 'lib', 'src', 'groups', 'user', 'models.dart'),
      ).existsSync(),
      isTrue,
    );
    expect(File(path.join(outDir, 'lib', 'src', 'core')).existsSync(), isFalse);
  });

  test('writePackage can format generated Dart files', () async {
    final fixture = File(
      path.join('test', 'fixtures', 'simple_spec.json'),
    ).readAsStringSync();
    final spec = Spec.fromJson(jsonDecode(fixture) as Map<String, dynamic>);

    final tempDir = Directory.systemTemp.createTempSync(
      'onedef-dart-generator-format-',
    );
    addTearDown(() => tempDir.deleteSync(recursive: true));

    final outDir = path.join(tempDir.path, 'user_api');
    await writePackage(
      spec: spec,
      packageName: 'user_api',
      outputDir: outDir,
      corePath: '../sdk_core',
      format: true,
    );

    final group = File(
      path.join(outDir, 'lib', 'src', 'groups', 'user', 'client.dart'),
    ).readAsStringSync();
    expect(group, contains('requestResult<User, DefaultError>'));
  });

  test('renderPackage uses sdkName override for method names', () async {
    final spec = Spec(
      version: 'v1',
      routes: RoutesSpec(
        groups: [
          GroupSpec(
            name: 'customer',
            endpoints: [
              Endpoint(
                name: 'GetUser',
                sdkName: 'get',
                method: 'GET',
                path: '/customers',
                successStatus: 204,
                request: RequestSpec(
                  paths: const [],
                  queries: const [],
                  headers: const [],
                  body: null,
                ),
                response: ResponseSpec(envelope: false, body: null),
              ),
              Endpoint(
                name: 'ListUsers',
                method: 'GET',
                path: '/customers',
                successStatus: 204,
                request: RequestSpec(
                  paths: const [],
                  queries: const [],
                  headers: const [],
                  body: null,
                ),
                response: ResponseSpec(envelope: false, body: null),
              ),
            ],
            groups: const [],
          ),
        ],
      ),
      models: const [],
    );

    final files = renderPackage(
      spec: spec,
      packageName: 'customer_api',
      corePath: '../sdk_core',
    );

    final group = files['lib/src/groups/customer/client.dart']!;
    expect(group, contains('Future<Result<void, DefaultError>> get('));
    expect(
      group,
      isNot(contains('Future<Result<void, DefaultError>> getUser(')),
    );
    expect(group, contains('Future<Result<void, DefaultError>> listUsers('));
  });

  test('renderPackage does not apply undeclared initialisms', () async {
    final spec = Spec(
      version: 'v1',
      routes: RoutesSpec(
        groups: [
          GroupSpec(
            name: 'booking',
            endpoints: [
              Endpoint(
                name: 'FindByID',
                method: 'GET',
                path: '/bookings/{id}',
                successStatus: 204,
                request: RequestSpec(
                  paths: [
                    Parameter(
                      name: 'ID',
                      key: 'id',
                      type: TypeUsage(
                        kind: TypeUsageKind.string,
                        name: '',
                        nullable: false,
                      ),
                    ),
                  ],
                  queries: const [],
                  headers: const [],
                  body: null,
                ),
                response: ResponseSpec(envelope: false, body: null),
              ),
            ],
          ),
        ],
      ),
      models: const [],
    );

    final files = renderPackage(
      spec: spec,
      packageName: 'booking_api',
      corePath: '../sdk_core',
    );

    final group = files['lib/src/groups/booking/client.dart']!;
    expect(group, contains('Future<Result<void, DefaultError>> findByID('));
    expect(group, isNot(contains('findById')));
  });

  test('renderPackage applies only IR-declared initialisms', () async {
    final stringType = TypeUsage(
      kind: TypeUsageKind.string,
      name: '',
      nullable: false,
    );
    final resourceType = TypeUsage(
      kind: TypeUsageKind.named,
      name: 'Resource',
      nullable: false,
    );
    final spec = Spec(
      version: 'v1',
      initialisms: const ['API', 'URL', 'OAuth'],
      routes: RoutesSpec(
        groups: [
          GroupSpec(
            name: 'resource',
            endpoints: [
              Endpoint(
                name: 'GetAPIURL',
                method: 'GET',
                path: '/resources/{api_url}',
                successStatus: 200,
                request: RequestSpec(
                  paths: [
                    Parameter(name: 'APIURL', key: 'api_url', type: stringType),
                  ],
                  queries: [
                    Parameter(
                      name: 'OAuthToken',
                      key: 'oauth_token',
                      type: stringType,
                    ),
                  ],
                  headers: const [],
                  body: null,
                ),
                response: ResponseSpec(envelope: true, body: resourceType),
              ),
            ],
          ),
        ],
      ),
      models: [
        ModelDef(
          name: 'Resource',
          kind: 'object',
          fields: [
            FieldDef(
              name: 'APIURL',
              key: 'api_url',
              type: stringType,
              required: true,
              omitEmpty: false,
            ),
            FieldDef(
              name: 'OAuthToken',
              key: 'oauth_token',
              type: stringType,
              required: true,
              omitEmpty: false,
            ),
          ],
        ),
      ],
    );

    final files = renderPackage(
      spec: spec,
      packageName: 'resource_api',
      corePath: '../sdk_core',
    );

    final group = files['lib/src/groups/resource/client.dart']!;
    expect(
      group,
      contains('Future<Result<Resource, DefaultError>> getApiUrl('),
    );
    expect(group, contains('required String apiUrl'));
    expect(group, contains('String? oauthToken'));
    expect(group, isNot(contains('apiurl')));
    expect(group, isNot(contains('oAuthToken')));

    final models = files['lib/src/groups/resource/models.dart']!;
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
      corePath: '../sdk_core',
    );

    expect(files.containsKey('pubspec.yaml'), isTrue);
    expect(files.containsKey('lib/user_api.dart'), isTrue);
    expect(files.containsKey('lib/src/client.dart'), isTrue);
    expect(files.containsKey('lib/src/groups/user/client.dart'), isTrue);
    expect(files.containsKey('lib/src/groups/user/models.dart'), isTrue);
    expect(files.containsKey('lib/src/strategy.dart'), isFalse);

    final client = files['lib/src/client.dart']!;
    expect(client, contains('class ApiClient'));
    expect(client, contains('required UserGroup user,'));
    expect(client, contains('final UserGroupClient user;'));
    expect(client, contains('user: UserGroupClient._bind('));
    expect(client, contains("import 'groups/user/client.dart';"));

    final providers = files['lib/src/providers.dart']!;
    expect(providers, contains('typedef HeaderValueProvider'));

    final group = files['lib/src/groups/user/client.dart']!;
    expect(group, isNot(contains('ScopeStrategy')));
    expect(group, contains("import '../../models.dart';"));
    expect(group, contains("import '../../providers.dart';"));
    expect(group, contains("import 'models.dart';"));
    expect(group, contains('class UserGroup'));
    expect(group, contains('class UserGroupClient'));
    expect(
      group,
      contains('required HeaderValueProvider<String> authorization,'),
    );
    expect(group, contains('Future<Result<User, DefaultError>> createUser('));
    expect(
      group,
      contains('return transport.requestResult<User, DefaultError>('),
    );
    expect(
      group,
      contains(
        '    final transport = _transport;\n'
        '\n'
        '    final headers = <String, String>{};\n'
        "    headers['Authorization'] = (await _authorization()).toString();\n"
        "    headers['Idempotency-Key'] = idempotencyKey.toString();\n"
        "    if (requestId != null) headers['X-Request-Id'] = requestId.toString();\n"
        '\n'
        '    const queryParameters = <String, String>{};\n'
        '\n'
        '    return transport.requestResult<User, DefaultError>(',
      ),
    );
    expect(group, contains('method: HttpMethod.post,'));
    expect(group, isNot(contains('ApiException(')));
    expect(group, isNot(contains('parseSuccessEnvelope(')));
    expect(group, isNot(contains('} on ApiNetworkException catch')));
    expect(
      group,
      contains(
        "headers['Authorization'] = (await _authorization()).toString();",
      ),
    );
    expect(group, contains('required int idempotencyKey'));
    expect(
      group,
      contains("headers['Idempotency-Key'] = idempotencyKey.toString();"),
    );
    expect(group, contains('String? requestId'));
    expect(
      group,
      contains(
        "if (requestId != null) headers['X-Request-Id'] = requestId.toString();",
      ),
    );

    final sharedModels = files['lib/src/models.dart']!;
    expect(sharedModels, isNot(contains('class CreateUserRequest')));
    expect(sharedModels, isNot(contains('class User')));

    final groupModels = files['lib/src/groups/user/models.dart']!;
    expect(groupModels, contains('class CreateUserRequest'));
    expect(groupModels, contains('class User'));
  });

  test('renderPackage keeps sibling group header providers separate', () {
    final spec = Spec(
      version: 'v1',
      routes: RoutesSpec(
        groups: [
          GroupSpec(
            name: 'customer',
            headers: const [
              HeaderSpec(
                key: 'Authorization',
                type: TypeUsage(kind: TypeUsageKind.string),
              ),
            ],
            endpoints: const [],
            groups: const [],
          ),
          GroupSpec(
            name: 'merchant',
            headers: const [
              HeaderSpec(
                key: 'Authorization',
                type: TypeUsage(kind: TypeUsageKind.string),
              ),
            ],
            endpoints: const [],
            groups: const [],
          ),
        ],
      ),
      models: const [],
    );

    final files = renderPackage(
      spec: spec,
      packageName: 'chat_api',
      corePath: '../sdk_core',
    );

    final client = files['lib/src/client.dart']!;
    expect(client, contains('required CustomerGroup customer,'));
    expect(client, contains('required MerchantGroup merchant,'));
    expect(
      client,
      isNot(contains('required HeaderValueProvider<String> authorization,')),
    );
    expect(client, contains('customer: CustomerGroupClient._bind('));
    expect(client, contains('merchant: MerchantGroupClient._bind('));

    final customer = files['lib/src/groups/customer/client.dart']!;
    expect(
      customer,
      contains('required HeaderValueProvider<String> authorization,'),
    );

    final merchant = files['lib/src/groups/merchant/client.dart']!;
    expect(
      merchant,
      contains('required HeaderValueProvider<String> authorization,'),
    );
  });

  test(
    'renderPackage inherits route root headers into root and child methods',
    () {
      final spec = Spec(
        version: 'v1',
        routes: RoutesSpec(
          headers: const [
            HeaderSpec(
              key: 'Authorization',
              type: TypeUsage(kind: TypeUsageKind.string),
            ),
          ],
          endpoints: const [
            Endpoint(
              name: 'Health',
              method: 'GET',
              path: '/health',
              successStatus: 204,
              request: RequestSpec(),
              response: ResponseSpec(envelope: false),
            ),
          ],
          groups: const [
            GroupSpec(
              name: 'branch',
              endpoints: [
                Endpoint(
                  name: 'ListBranches',
                  method: 'GET',
                  path: '/api/v1/branches',
                  successStatus: 204,
                  request: RequestSpec(),
                  response: ResponseSpec(envelope: false),
                ),
              ],
            ),
          ],
        ),
        models: const [],
      );

      final files = renderPackage(
        spec: spec,
        packageName: 'branch_api',
        corePath: '../sdk_core',
      );

      final client = files['lib/src/client.dart']!;
      expect(
        client,
        contains('required HeaderValueProvider<String> authorization,'),
      );
      expect(client, contains('authorization: authorization,'));
      expect(
        client,
        contains(
          "headers['Authorization'] = (await _authorization()).toString();",
        ),
      );

      final branchClient = files['lib/src/groups/branch/client.dart']!;
      expect(
        branchClient,
        contains('required HeaderValueProvider<String> authorization,'),
      );
      expect(
        branchClient,
        contains(
          "headers['Authorization'] = (await _authorization()).toString();",
        ),
      );
    },
  );

  test(
    'renderPackage emits unique nested group files and inherited providers',
    () async {
      final fixture = File(
        path.join('test', 'fixtures', 'grouped_nested_spec.json'),
      ).readAsStringSync();
      final spec = Spec.fromJson(jsonDecode(fixture) as Map<String, dynamic>);

      final files = renderPackage(
        spec: spec,
        packageName: 'user_api',
        corePath: '../sdk_core',
      );

      expect(files.containsKey('pubspec.yaml'), isTrue);
      expect(files.containsKey('lib/user_api.dart'), isTrue);
      expect(files.containsKey('lib/src/client.dart'), isTrue);
      expect(files.containsKey('lib/src/groups/branch/client.dart'), isTrue);
      expect(files.containsKey('lib/src/groups/branch/models.dart'), isTrue);
      expect(
        files.containsKey('lib/src/groups/branch_booking/client.dart'),
        isTrue,
      );
      expect(
        files.containsKey('lib/src/groups/branch_booking/models.dart'),
        isTrue,
      );
      expect(files.containsKey('lib/src/groups/customer/client.dart'), isTrue);
      expect(files.containsKey('lib/src/groups/customer/models.dart'), isTrue);
      expect(
        files.containsKey('lib/src/groups/customer_booking/client.dart'),
        isTrue,
      );
      expect(
        files.containsKey('lib/src/groups/customer_booking/models.dart'),
        isTrue,
      );

      final client = files['lib/src/client.dart']!;
      expect(client, isNot(contains('ScopeStrategy')));
      expect(client, contains('class ApiClient'));
      expect(client, contains('required BranchGroup branch,'));
      expect(client, contains('required CustomerGroup customer,'));
      expect(client, contains('final BranchGroupClient branch;'));
      expect(client, contains('final CustomerGroupClient customer;'));
      expect(
        client,
        isNot(contains('required HeaderValueProvider<String> authorization,')),
      );
      expect(client, isNot(contains('branchBranchId')));
      expect(client, isNot(contains('branchBookingScope')));
      expect(client, contains('branch: BranchGroupClient._bind('));
      expect(client, contains('customer: CustomerGroupClient._bind('));

      final branch = files['lib/src/groups/branch/client.dart']!;
      expect(branch, isNot(contains('ScopeStrategy')));
      expect(branch, contains("import '../branch_booking/client.dart';"));
      expect(branch, contains('class BranchGroup'));
      expect(branch, contains('class BranchGroupClient'));
      expect(branch, contains('required this.xBranchId,'));
      expect(branch, contains('required this.booking,'));
      expect(branch, contains('final BranchBookingGroupClient booking;'));
      expect(branch, contains('xBranchId: config.xBranchId,'));

      final branchBooking = files['lib/src/groups/branch_booking/client.dart']!;
      expect(branchBooking, isNot(contains('ScopeStrategy')));
      expect(branchBooking, contains('class BranchBookingGroup'));
      expect(branchBooking, contains('class BranchBookingGroupClient'));
      expect(
        branchBooking,
        contains('final HeaderValueProvider<String> _authorization;'),
      );
      expect(
        branchBooking,
        contains('final HeaderValueProvider<int> _xBranchId;'),
      );
      expect(
        branchBooking,
        contains('final HeaderValueProvider<String> _xBookingScope;'),
      );
      expect(
        branchBooking,
        contains('return transport.requestResult<Booking, DefaultError>('),
      );
      expect(branchBooking, contains('method: HttpMethod.post,'));
      expect(
        branchBooking,
        contains(
          "headers['Authorization'] = (await _authorization()).toString();",
        ),
      );
      expect(
        branchBooking,
        contains("headers['X-Branch-Id'] = (await _xBranchId()).toString();"),
      );
      expect(
        branchBooking,
        contains(
          "headers['X-Booking-Scope'] = (await _xBookingScope()).toString();",
        ),
      );
      expect(branchBooking, contains('required String idempotencyKey'));
      expect(
        branchBooking,
        contains("headers['Idempotency-Key'] = idempotencyKey.toString();"),
      );

      final customerBooking =
          files['lib/src/groups/customer_booking/client.dart']!;
      expect(
        customerBooking,
        contains('final HeaderValueProvider<String> _authorization;'),
      );
      expect(
        customerBooking,
        contains('final HeaderValueProvider<String> _xCustomerId;'),
      );
      expect(
        customerBooking,
        contains('Future<Result<Booking, DefaultError>> findById('),
      );
      expect(customerBooking, isNot(contains('findByID')));
      expect(
        customerBooking,
        contains(
          "headers['Authorization'] = (await _authorization()).toString();",
        ),
      );
      expect(
        customerBooking,
        contains(
          "headers['X-Customer-Id'] = (await _xCustomerId()).toString();",
        ),
      );

      final sharedModels = files['lib/src/models.dart']!;
      expect(sharedModels, contains('class Booking'));
      expect(sharedModels, isNot(contains('class CreateBookingRequest')));

      final branchBookingModels =
          files['lib/src/groups/branch_booking/models.dart']!;
      expect(branchBookingModels, contains('class CreateBookingRequest'));
      expect(branchBookingModels, isNot(contains('class Booking')));

      final customerBookingModels =
          files['lib/src/groups/customer_booking/models.dart']!;
      expect(customerBookingModels, isNot(contains('class Booking')));
      expect(
        customerBookingModels,
        isNot(contains('class CreateBookingRequest')),
      );
    },
  );
}
