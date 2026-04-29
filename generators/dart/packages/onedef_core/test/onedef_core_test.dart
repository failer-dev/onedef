import 'dart:convert';

import 'package:http/http.dart' as http;
import 'package:onedef_core/onedef_core.dart';
import 'package:test/test.dart';

void main() {
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

  test('Transport parses success envelopes and reports contract violations',
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
  });

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
}
