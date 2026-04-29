import 'dart:convert';
import 'dart:io';

import 'package:http/http.dart' as http;

import 'exceptions.dart';

class Transport {
  Transport({required this.baseUrl, http.Client? client})
      : client = client ?? http.Client();

  final String baseUrl;
  final http.Client client;

  Future<http.Response> request({
    required String method,
    required String path,
    required Map<String, String> queryParameters,
    Object? body,
    Map<String, String> headers = const {},
  }) async {
    final uri = Uri.parse('$baseUrl$path').replace(
      queryParameters: queryParameters.isEmpty ? null : queryParameters,
    );

    try {
      switch (method) {
        case 'GET':
          return await client.get(uri,
              headers: headers.isEmpty ? null : headers);
        case 'POST':
          return await client.post(
            uri,
            headers: {
              ...headers,
              'Content-Type': 'application/json',
            },
            body: jsonEncode(body),
          );
        case 'PUT':
          return await client.put(
            uri,
            headers: {
              ...headers,
              'Content-Type': 'application/json',
            },
            body: jsonEncode(body),
          );
        case 'PATCH':
          return await client.patch(
            uri,
            headers: {
              ...headers,
              'Content-Type': 'application/json',
            },
            body: jsonEncode(body),
          );
        case 'DELETE':
          return await client.delete(uri,
              headers: headers.isEmpty ? null : headers);
        default:
          throw StateError('Unsupported method $method');
      }
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

  T decodeResponseValue<T>(
    int status,
    String rawBody,
    T Function() decode,
  ) {
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

  int expectInt(
    Object? value,
    int status,
    String rawBody,
    String message,
  ) {
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

  bool expectBool(
    Object? value,
    int status,
    String rawBody,
    String message,
  ) {
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
