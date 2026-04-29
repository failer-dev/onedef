sealed class Result<T, E> {
  const Result();
}

final class Success<T, E> extends Result<T, E> {
  const Success(this.value);

  final SuccessResponse<T> value;
}

class SuccessResponse<T> {
  const SuccessResponse({
    required this.status,
    required this.code,
    required this.title,
    required this.message,
    required this.data,
  });

  final int status;
  final String code;
  final String title;
  final String message;
  final T? data;
}

enum NetworkFailureKind {
  socket,
  httpClient,
  unknown,
  ;

  bool get isSocket => this == NetworkFailureKind.socket;
  bool get isHttpClient => this == NetworkFailureKind.httpClient;
  bool get isUnknown => this == NetworkFailureKind.unknown;
}

final class ApiException<T, E> extends Result<T, E> implements Exception {
  const ApiException({
    required this.statusCode,
    required this.data,
    required this.rawBody,
    required this.stackTrace,
  });

  final int statusCode;
  final E data;
  final String rawBody;
  final StackTrace stackTrace;

  @override
  String toString() => 'ApiException(statusCode: $statusCode, data: $data)';
}

final class ApiNetworkException<T, E> extends Result<T, E>
    implements Exception {
  const ApiNetworkException({
    required this.failureKind,
    required this.cause,
    required this.stackTrace,
  });

  final NetworkFailureKind failureKind;
  final Object cause;
  final StackTrace stackTrace;

  @override
  String toString() =>
      'ApiNetworkException(failureKind: $failureKind, cause: $cause)';
}

final class ApiContractViolationException<T, E> extends Result<T, E>
    implements Exception {
  const ApiContractViolationException({
    required this.statusCode,
    required this.rawBody,
    this.cause,
    required this.stackTrace,
  });

  final int statusCode;
  final String rawBody;
  final Object? cause;
  final StackTrace stackTrace;

  @override
  String toString() =>
      'ApiContractViolationException(statusCode: $statusCode, cause: $cause)';
}
