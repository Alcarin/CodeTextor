import 'dart:async';
import 'package:http/http.dart' as http;
export 'package:path/path.dart';

/// A simple user class.
class User {
  final String _name;
  final int age;

  User(this._name, this.age);
  factory User.redirect() = User;
  factory User.anonymous() => User("Guest", 0);

  String get displayName => _name.toUpperCase();

  void sayHello() {
    print("Hello, my name is $_name");
  }

  // TODO: Add support for profiles
  void _updateProfile() {
    // Hidden logic
  }
}

enum Status {
  pending,
  active,
  archived
}

typedef Callback = void Function(String message);

void main() {
  final user = User("Alice", 30);
  user.sayHello();
}
