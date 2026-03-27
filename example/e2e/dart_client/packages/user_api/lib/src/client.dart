import 'dart:convert';

import 'package:http/http.dart' as http;

import 'models.dart';

class UserApi {
  final String baseUrl;
  final http.Client _client;

  UserApi({required this.baseUrl, http.Client? client})
      : _client = client ?? http.Client();

  // GET /users/{id}
  Future<User> getUser({required String id}) async {
    final uri = Uri.parse('$baseUrl/users/$id');
    final resp = await _client.get(uri);
    if (resp.statusCode >= 400) {
      throw Exception('HTTP ${resp.statusCode}: ${resp.body}');
    }
    return User.fromJson(jsonDecode(resp.body) as Map<String, dynamic>);
  }

  // POST /users
  Future<User> createUser({required CreateUserRequest body}) async {
    final uri = Uri.parse('$baseUrl/users');
    final resp = await _client.post(
      uri,
      headers: {'Content-Type': 'application/json'},
      body: jsonEncode(body.toJson()),
    );
    if (resp.statusCode >= 400) {
      throw Exception('HTTP ${resp.statusCode}: ${resp.body}');
    }
    return User.fromJson(jsonDecode(resp.body) as Map<String, dynamic>);
  }
}
