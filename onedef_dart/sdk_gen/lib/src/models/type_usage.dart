part of 'index.dart';

enum TypeUsageKind { any, bool, float, int, string, uuid, named, list, map }

class TypeUsage {
  const TypeUsage({
    required this.kind,
    this.name = '',
    this.nullable = false,
    this.elem,
    this.key,
    this.value,
  });

  factory TypeUsage.fromJson(Object? json) {
    if (json is String) {
      return _TypeExpressionParser(json).parse();
    }
    throw FormatException('Type usage must be a string expression.', json);
  }

  final TypeUsageKind kind;

  final String name;

  final bool nullable;

  final TypeUsage? elem;
  final TypeUsage? key;
  final TypeUsage? value;

  Object toJson() => _formatTypeUsage(this);
}

String _formatTypeUsage(TypeUsage type) {
  final base = switch (type.kind) {
    TypeUsageKind.any => 'any',
    TypeUsageKind.bool => 'bool',
    TypeUsageKind.float => 'float',
    TypeUsageKind.int => 'int',
    TypeUsageKind.string => 'string',
    TypeUsageKind.uuid => 'uuid',
    TypeUsageKind.named => type.name,
    TypeUsageKind.list => 'list<${_formatTypeUsage(type.elem!)}>',
    TypeUsageKind.map =>
      'map<${_formatTypeUsage(type.key ?? const TypeUsage(kind: TypeUsageKind.string))}, ${_formatTypeUsage(type.value!)}>',
  };
  return type.nullable ? '$base?' : base;
}

class _TypeExpressionParser {
  _TypeExpressionParser(this.input);

  final String input;
  int _index = 0;

  TypeUsage parse() {
    final type = _parseType();
    _skipSpaces();
    if (!_done) {
      throw _error('Unexpected token "${input.substring(_index)}".');
    }
    return type;
  }

  TypeUsage _parseType() {
    _skipSpaces();
    final identifier = _parseIdentifier();
    final type = switch (identifier) {
      'any' => const TypeUsage(kind: TypeUsageKind.any),
      'bool' => const TypeUsage(kind: TypeUsageKind.bool),
      'float' => const TypeUsage(kind: TypeUsageKind.float),
      'int' => const TypeUsage(kind: TypeUsageKind.int),
      'string' => const TypeUsage(kind: TypeUsageKind.string),
      'uuid' => const TypeUsage(kind: TypeUsageKind.uuid),
      'list' => TypeUsage(kind: TypeUsageKind.list, elem: _parseOneArg('list')),
      'map' => _parseMap(),
      _ => TypeUsage(kind: TypeUsageKind.named, name: identifier),
    };

    _skipSpaces();
    if (_consume('?')) {
      return TypeUsage(
        kind: type.kind,
        name: type.name,
        nullable: true,
        elem: type.elem,
        key: type.key,
        value: type.value,
      );
    }
    return type;
  }

  TypeUsage _parseOneArg(String name) {
    _expect('<', name);
    final elem = _parseType();
    _expect('>', name);
    return elem;
  }

  TypeUsage _parseMap() {
    _expect('<', 'map');
    final key = _parseType();
    _expect(',', 'map');
    final value = _parseType();
    _expect('>', 'map');
    return TypeUsage(kind: TypeUsageKind.map, key: key, value: value);
  }

  String _parseIdentifier() {
    _skipSpaces();
    final start = _index;
    while (!_done) {
      final code = input.codeUnitAt(_index);
      final isIdentifierChar = _isLetter(code) || _isDigit(code) || code == 95;
      if (!isIdentifierChar) {
        break;
      }
      _index++;
    }
    if (start == _index) {
      throw _error('Expected type name.');
    }
    final value = input.substring(start, _index);
    if (_isDigit(value.codeUnitAt(0))) {
      throw _error('Type name must not start with a digit.');
    }
    return value;
  }

  void _expect(String char, String typeName) {
    _skipSpaces();
    if (!_consume(char)) {
      throw _error('Expected "$char" in $typeName type expression.');
    }
  }

  bool _consume(String char) {
    if (_done || input[_index] != char) {
      return false;
    }
    _index++;
    return true;
  }

  void _skipSpaces() {
    while (!_done && input.codeUnitAt(_index) <= 32) {
      _index++;
    }
  }

  bool get _done => _index >= input.length;

  FormatException _error(String message) {
    return FormatException(
      'Invalid type expression at byte $_index: $message',
      input,
      _index,
    );
  }
}

bool _isLetter(int code) =>
    (code >= 65 && code <= 90) || (code >= 97 && code <= 122);

bool _isDigit(int code) => code >= 48 && code <= 57;
