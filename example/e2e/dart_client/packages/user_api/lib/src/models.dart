class User {
  final String id;
  final String name;

  User({
    required this.id,
    required this.name,
  });

  factory User.fromJson(Map<String, dynamic> json) => User(
    id: json['id'] as String,
    name: json['name'] as String,
  );

  Map<String, dynamic> toJson() => {
    'id': id,
    'name': name,
  };
}

class CreateUserRequest {
  final String name;

  CreateUserRequest({
    required this.name,
  });

  factory CreateUserRequest.fromJson(Map<String, dynamic> json) => CreateUserRequest(
    name: json['name'] as String,
  );

  Map<String, dynamic> toJson() => {
    'name': name,
  };
}
