class DefaultError {
  const DefaultError({required this.code, required this.message, this.details});

  factory DefaultError.fromJson(Map<String, dynamic> json) {
    return DefaultError(
      code: json['code'] as String,
      message: json['message'] as String,
      details: json['details'],
    );
  }

  final String code;
  final String message;
  final Object? details;

  Map<String, dynamic> toJson() {
    return <String, dynamic>{
      'code': code,
      'message': message,
      if (details != null) 'details': details,
    };
  }
}
