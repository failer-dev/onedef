import 'dart:convert';
import 'dart:io';

import 'package:http/http.dart' as http;

import 'exceptions.dart';
import 'result.dart';

enum HttpMethod {
  get('GET'),
  post('POST'),
  put('PUT'),
  patch('PATCH'),
  delete('DELETE'),
  head('HEAD'),
  options('OPTIONS');

  const HttpMethod(this.wireName);

  final String wireName;

  bool get sendsJsonBody =>
      this == HttpMethod.post ||
      this == HttpMethod.put ||
      this == HttpMethod.patch;
}

typedef ResponseDecoder<T> = T Function(
    Object? data, int statusCode, String rawBody);

class Transport {
  Transport({required this.baseUrl, http.Client? client})
      : client = client ?? http.Client();

  final String baseUrl;
  final http.Client client;

  Future<http.Response> request({
    required HttpMethod method,
    required String path,
    required Map<String, String> queryParameters,
    Object? body,
    Map<String, String> headers = const {},
  }) async {
    final uri = Uri.parse('$baseUrl$path').replace(
      queryParameters: queryParameters.isEmpty ? null : queryParameters,
    );

    try {
      final request = http.Request(method.wireName, uri);
      request.headers.addAll(headers);
      if (method.sendsJsonBody && body != null) {
        request.headers['Content-Type'] = 'application/json';
        request.body = jsonEncode(body);
      }
      return http.Response.fromStream(await client.send(request));
    } on SocketException catch (e) {
      throw ApiNetworkException<Never, Never>(
        failureKind: NetworkFailureKind.socket,
        cause: e,
        stackTrace: StackTrace.current,
      );
    } on http.ClientException catch (e) {
      throw ApiNetworkException<Never, Never>(
        failureKind: NetworkFailureKind.httpClient,
        cause: e,
        stackTrace: StackTrace.current,
      );
    } on Exception catch (e) {
      throw ApiNetworkException<Never, Never>(
        failureKind: NetworkFailureKind.unknown,
        cause: e,
        stackTrace: StackTrace.current,
      );
    }
  }

  Future<Result<T, E>> requestResult<T, E>({
    required HttpMethod method,
    required String path,
    required int expectedStatus,
    required Map<String, String> queryParameters,
    Object? body,
    Map<String, String> headers = const {},
    ResponseDecoder<T>? decodeSuccess,
    required ResponseDecoder<E> decodeError,
  }) async {
    try {
      final response = await request(
        method: method,
        path: path,
        queryParameters: queryParameters,
        headers: headers,
        body: body,
      );

      if (response.statusCode != expectedStatus) {
        final errorJson = parseJsonBody(
          response,
          'Expected a JSON error response body.',
        );
        final errorData = decodeResponseValue<E>(
          response.statusCode,
          response.body,
          () => decodeError(errorJson, response.statusCode, response.body),
        );
        return ApiException<T, E>(
          statusCode: response.statusCode,
          data: errorData,
          rawBody: response.body,
          stackTrace: StackTrace.current,
        );
      }

      if (decodeSuccess == null) {
        final metadata = _successMetadata(response.statusCode);
        return Success<T, E>(
          SuccessResponse<T>(
            status: response.statusCode,
            code: metadata.code,
            title: metadata.title,
            message: 'success',
            data: null,
          ),
        );
      }

      final envelope = parseSuccessEnvelope(response);
      final data = decodeResponseValue<T>(
        response.statusCode,
        envelope.rawBody,
        () =>
            decodeSuccess(envelope.data, response.statusCode, envelope.rawBody),
      );
      return Success<T, E>(
        SuccessResponse<T>(
          status: response.statusCode,
          code: envelope.code,
          title: envelope.title,
          message: envelope.message,
          data: data,
        ),
      );
    } on ApiNetworkException catch (e) {
      return ApiNetworkException<T, E>(
        failureKind: e.failureKind,
        cause: e.cause,
        stackTrace: e.stackTrace,
      );
    } on ApiContractViolationException catch (e) {
      return ApiContractViolationException<T, E>(
        statusCode: e.statusCode,
        rawBody: e.rawBody,
        cause: e.cause,
        stackTrace: e.stackTrace,
      );
    }
  }

  SuccessEnvelope parseSuccessEnvelope(http.Response response) {
    final rawBody = response.body;
    final decoded = parseJsonBody(
      response,
      'Expected a JSON success response body.',
    );

    final envelope = expectJsonObject(
      decoded,
      response.statusCode,
      rawBody,
      'Expected success response to be a JSON object.',
    );
    final code = envelope['code'];
    final title = envelope['title'];
    final message = envelope['message'];
    if (code is! String || title is! String || message is! String) {
      throw ApiContractViolationException<Never, Never>(
        statusCode: response.statusCode,
        rawBody: rawBody,
        cause:
            'Expected success response envelope to include string code, title, and message.',
        stackTrace: StackTrace.current,
      );
    }

    return SuccessEnvelope(
      code: code,
      title: title,
      message: message,
      data: envelope['data'],
      rawBody: rawBody,
    );
  }

  Object? parseJsonBody(http.Response response, String emptyMessage) {
    final rawBody = response.body;
    if (rawBody.isEmpty) {
      throw ApiContractViolationException<Never, Never>(
        statusCode: response.statusCode,
        rawBody: rawBody,
        cause: emptyMessage,
        stackTrace: StackTrace.current,
      );
    }

    try {
      return jsonDecode(rawBody);
    } on FormatException catch (e) {
      throw ApiContractViolationException<Never, Never>(
        statusCode: response.statusCode,
        rawBody: rawBody,
        cause: e,
        stackTrace: StackTrace.current,
      );
    }
  }

  Map<String, dynamic> expectJsonObject(
    Object? value,
    int status,
    String rawBody,
    String message,
  ) {
    if (value is Map<String, dynamic>) {
      return value;
    }
    if (value is Map) {
      return value.map((key, value) => MapEntry(key.toString(), value));
    }
    throw ApiContractViolationException<Never, Never>(
      statusCode: status,
      rawBody: rawBody,
      cause: message,
      stackTrace: StackTrace.current,
    );
  }

  T decodeResponseValue<T>(int status, String rawBody, T Function() decode) {
    try {
      return decode();
    } on ApiContractViolationException {
      rethrow;
    } catch (e, stackTrace) {
      throw ApiContractViolationException<Never, Never>(
        statusCode: status,
        rawBody: rawBody,
        cause: e,
        stackTrace: stackTrace,
      );
    }
  }

  List<dynamic> expectJsonList(
    Object? value,
    int status,
    String rawBody,
    String message,
  ) {
    if (value is List<dynamic>) {
      return value;
    }
    if (value is List) {
      return List<dynamic>.from(value);
    }
    throw ApiContractViolationException<Never, Never>(
      statusCode: status,
      rawBody: rawBody,
      cause: message,
      stackTrace: StackTrace.current,
    );
  }

  String expectString(
    Object? value,
    int status,
    String rawBody,
    String message,
  ) {
    if (value is String) {
      return value;
    }
    throw ApiContractViolationException<Never, Never>(
      statusCode: status,
      rawBody: rawBody,
      cause: message,
      stackTrace: StackTrace.current,
    );
  }

  int expectInt(Object? value, int status, String rawBody, String message) {
    if (value is int) {
      return value;
    }
    if (value is num && value == value.roundToDouble()) {
      return value.toInt();
    }
    throw ApiContractViolationException<Never, Never>(
      statusCode: status,
      rawBody: rawBody,
      cause: message,
      stackTrace: StackTrace.current,
    );
  }

  double expectDouble(
    Object? value,
    int status,
    String rawBody,
    String message,
  ) {
    if (value is num) {
      return value.toDouble();
    }
    throw ApiContractViolationException<Never, Never>(
      statusCode: status,
      rawBody: rawBody,
      cause: message,
      stackTrace: StackTrace.current,
    );
  }

  bool expectBool(Object? value, int status, String rawBody, String message) {
    if (value is bool) {
      return value;
    }
    throw ApiContractViolationException<Never, Never>(
      statusCode: status,
      rawBody: rawBody,
      cause: message,
      stackTrace: StackTrace.current,
    );
  }
}

class SuccessEnvelope {
  const SuccessEnvelope({
    required this.code,
    required this.title,
    required this.message,
    required this.data,
    required this.rawBody,
  });

  final String code;
  final String title;
  final String message;
  final Object? data;
  final String rawBody;
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
