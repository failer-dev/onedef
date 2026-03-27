import 'package:user_api/user_api.dart';

void main() async {
  final client = UserApi(baseUrl: 'http://localhost:8081');

  // 1. Create a user
  print('--- POST /users ---');
  final created = await client.createUser(
    body: CreateUserRequest(name: 'Bob'),
  );
  print('Created: ${created.id}, ${created.name}');

  // 2. Get a user
  print('--- GET /users/123 ---');
  final user = await client.getUser(id: '123');
  print('Fetched: ${user.id}, ${user.name}');

  print('--- Done ---');
}
