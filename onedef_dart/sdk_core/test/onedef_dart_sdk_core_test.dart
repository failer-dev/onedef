import 'dart:convert';

import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:onedef_dart_sdk_core/onedef_dart_sdk_core.dart';
import 'package:test/test.dart';

void main() {
  test('HttpMethod exposes wire method names', () {
    expect(HttpMethod.get.wireName, 'GET');
    expect(HttpMethod.post.wireName, 'POST');
    expect(HttpMethod.put.wireName, 'PUT');
    expect(HttpMethod.patch.wireName, 'PATCH');
    expect(HttpMethod.delete.wireName, 'DELETE');
    expect(HttpMethod.head.wireName, 'HEAD');
    expect(HttpMethod.options.wireName, 'OPTIONS');
  });

  test('network failure helpers expose boolean checks', () {
    expect(NetworkFailureKind.socket.isSocket, isTrue);
    expect(NetworkFailureKind.httpClient.isHttpClient, isTrue);
    expect(NetworkFailureKind.unknown.isUnknown, isTrue);
  });

  test('Result variants carry typed success and error values', () {
    const success = Success<String, DefaultError>(
      SuccessResponse<String>(
        status: 200,
        code: 'ok',
        title: 'OK',
        message: 'success',
        data: 'done',
      ),
    );
    expect(success.value.data, 'done');

    final failure = ApiException<String, DefaultError>(
      statusCode: 400,
      data: const DefaultError(code: 'bad_request', message: 'nope'),
      rawBody: '{}',
      stackTrace: StackTrace.current,
    );
    expect(failure.data.code, 'bad_request');
  });

  test('DefaultError parses JSON', () {
    final error = DefaultError.fromJson({
      'code': 'bad_request',
      'message': 'nope',
      'details': {'field': 'name'},
    });

    expect(error.code, 'bad_request');
    expect(error.details, isA<Map<String, dynamic>>());
  });

  test(
    'Transport parses success envelopes and reports contract violations',
    () {
      final transport = Transport(baseUrl: 'http://example.com');
      final response = http.Response(
        jsonEncode({
          'code': 'ok',
          'title': 'OK',
          'message': 'success',
          'data': {'id': '1'},
        }),
        200,
      );

      final envelope = transport.parseSuccessEnvelope(response);
      expect(envelope.code, 'ok');
      expect(envelope.data, isA<Map<String, dynamic>>());

      expect(
        () => transport.parseSuccessEnvelope(http.Response('not-json', 200)),
        throwsA(isA<ApiContractViolationException>()),
      );
    },
  );

  test('Transport wraps response decode failures as contract violations', () {
    final transport = Transport(baseUrl: 'http://example.com');

    expect(
      () => transport.decodeResponseValue<String>(
        200,
        '{"data":1}',
        () => throw TypeError(),
      ),
      throwsA(isA<ApiContractViolationException>()),
    );
  });

  test('Transport.request sends enum wire methods', () async {
    final methods = <String>[];
    final transport = Transport(
      baseUrl: 'http://example.com',
      client: MockClient((request) async {
        methods.add(request.method);
        return http.Response('', 204);
      }),
    );

    await transport.request(
      method: HttpMethod.patch,
      path: '/orders/1',
      queryParameters: const {},
      body: {'status': 'shipped'},
    );
    await transport.request(
      method: HttpMethod.options,
      path: '/orders',
      queryParameters: const {},
    );

    expect(methods, ['PATCH', 'OPTIONS']);
  });

  test('Transport.requestResult decodes success envelopes', () async {
    late Transport transport;
    transport = Transport(
      baseUrl: 'http://example.com',
      client: MockClient(
        (_) async => http.Response(
          jsonEncode({
            'code': 'ok',
            'title': 'OK',
            'message': 'success',
            'data': 'done',
          }),
          200,
        ),
      ),
    );

    final result = await transport.requestResult<String, DefaultError>(
      method: HttpMethod.get,
      path: '/ok',
      expectedStatus: 200,
      queryParameters: const {},
      decodeSuccess: (data, status, rawBody) => transport.expectString(
        data,
        status,
        rawBody,
        'Expected response data to be a string.',
      ),
      decodeError: (data, status, rawBody) => DefaultError.fromJson(
        transport.expectJsonObject(
          data,
          status,
          rawBody,
          'Expected error response to be a JSON object.',
        ),
      ),
    );

    final success = result as Success<String, DefaultError>;
    expect(success.value.data, 'done');
  });

  test('Transport.requestResult decodes non-2xx errors', () async {
    late Transport transport;
    transport = Transport(
      baseUrl: 'http://example.com',
      client: MockClient(
        (_) async => http.Response(
          jsonEncode({'code': 'bad_request', 'message': 'nope'}),
          400,
        ),
      ),
    );

    final result = await transport.requestResult<String, DefaultError>(
      method: HttpMethod.get,
      path: '/bad',
      expectedStatus: 200,
      queryParameters: const {},
      decodeSuccess: (data, status, rawBody) => transport.expectString(
        data,
        status,
        rawBody,
        'Expected response data to be a string.',
      ),
      decodeError: (data, status, rawBody) => DefaultError.fromJson(
        transport.expectJsonObject(
          data,
          status,
          rawBody,
          'Expected error response to be a JSON object.',
        ),
      ),
    );

    final failure = result as ApiException<String, DefaultError>;
    expect(failure.statusCode, 400);
    expect(failure.data.code, 'bad_request');
  });

  test('Transport.requestResult returns contract violations', () async {
    final transport = Transport(
      baseUrl: 'http://example.com',
      client: MockClient((_) async => http.Response('not-json', 200)),
    );

    final result = await transport.requestResult<String, DefaultError>(
      method: HttpMethod.get,
      path: '/broken',
      expectedStatus: 200,
      queryParameters: const {},
      decodeSuccess: (data, status, rawBody) => data as String,
      decodeError: (data, status, rawBody) => DefaultError.fromJson(
        transport.expectJsonObject(
          data,
          status,
          rawBody,
          'Expected error response to be a JSON object.',
        ),
      ),
    );

    expect(result, isA<ApiContractViolationException<String, DefaultError>>());
  });

  test('Transport.requestResult supports void success responses', () async {
    final transport = Transport(
      baseUrl: 'http://example.com',
      client: MockClient((_) async => http.Response('', 204)),
    );

    final result = await transport.requestResult<void, DefaultError>(
      method: HttpMethod.delete,
      path: '/orders/1',
      expectedStatus: 204,
      queryParameters: const {},
      decodeError: (data, status, rawBody) => DefaultError.fromJson(
        transport.expectJsonObject(
          data,
          status,
          rawBody,
          'Expected error response to be a JSON object.',
        ),
      ),
    );

    final success = result as Success<void, DefaultError>;
    expect(success.value.code, 'no_content');
  });
}
