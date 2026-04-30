package validator

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

func (t TypeRef) MarshalJSON() ([]byte, error) {
	expr, err := formatTypeRef(t)
	if err != nil {
		return nil, err
	}
	return []byte(strconv.Quote(expr)), nil
}

func (t *TypeRef) UnmarshalJSON(data []byte) error {
	var expr string
	if err := json.Unmarshal(data, &expr); err != nil {
		return fmt.Errorf("type ref must be a string expression: %w", err)
	}
	parsed, err := parseTypeExpr(expr)
	if err != nil {
		return err
	}
	*t = parsed
	return nil
}

func formatTypeRef(typeRef TypeRef) (string, error) {
	var expr string
	switch typeRef.Kind {
	case TypeKindAny, TypeKindBool, TypeKindFloat, TypeKindInt, TypeKindString, TypeKindUUID:
		expr = string(typeRef.Kind)
	case TypeKindNamed:
		if typeRef.Name == "" {
			return "", fmt.Errorf("named type ref must declare name")
		}
		expr = typeRef.Name
	case TypeKindList:
		if typeRef.Elem == nil {
			return "", fmt.Errorf("list type ref must declare elem")
		}
		elem, err := formatTypeRef(*typeRef.Elem)
		if err != nil {
			return "", err
		}
		expr = "list<" + elem + ">"
	case TypeKindMap:
		key := TypeRef{Kind: TypeKindString}
		if typeRef.Key != nil {
			key = *typeRef.Key
		}
		keyExpr, err := formatTypeRef(key)
		if err != nil {
			return "", err
		}
		value, err := formatTypeRefValue(typeRef)
		if err != nil {
			return "", err
		}
		expr = "map<" + keyExpr + ", " + value + ">"
	default:
		return "", fmt.Errorf("unsupported type kind %q", typeRef.Kind)
	}
	if typeRef.Nullable {
		expr += "?"
	}
	return expr, nil
}

func formatTypeRefValue(typeRef TypeRef) (string, error) {
	if typeRef.Value == nil {
		return "", fmt.Errorf("map type ref must declare value")
	}
	return formatTypeRef(*typeRef.Value)
}

func parseTypeExpr(expr string) (TypeRef, error) {
	parser := typeExprParser{input: expr}
	typeRef, err := parser.parseType()
	if err != nil {
		return TypeRef{}, err
	}
	parser.skipSpaces()
	if !parser.done() {
		return TypeRef{}, parser.errorf("unexpected token %q", parser.input[parser.pos:])
	}
	return typeRef, nil
}

type typeExprParser struct {
	input string
	pos   int
}

func (p *typeExprParser) parseType() (TypeRef, error) {
	p.skipSpaces()
	ident, err := p.parseIdent()
	if err != nil {
		return TypeRef{}, err
	}

	var typeRef TypeRef
	switch ident {
	case string(TypeKindAny):
		typeRef = TypeRef{Kind: TypeKindAny}
	case string(TypeKindBool):
		typeRef = TypeRef{Kind: TypeKindBool}
	case string(TypeKindFloat):
		typeRef = TypeRef{Kind: TypeKindFloat}
	case string(TypeKindInt):
		typeRef = TypeRef{Kind: TypeKindInt}
	case string(TypeKindString):
		typeRef = TypeRef{Kind: TypeKindString}
	case string(TypeKindUUID):
		typeRef = TypeRef{Kind: TypeKindUUID}
	case string(TypeKindList):
		elem, err := p.parseOneArgType(ident)
		if err != nil {
			return TypeRef{}, err
		}
		typeRef = TypeRef{Kind: TypeKindList, Elem: &elem}
	case string(TypeKindMap):
		key, value, err := p.parseTwoArgType(ident)
		if err != nil {
			return TypeRef{}, err
		}
		typeRef = TypeRef{Kind: TypeKindMap, Key: &key, Value: &value}
	default:
		typeRef = TypeRef{Kind: TypeKindNamed, Name: ident}
	}

	p.skipSpaces()
	if p.consume('?') {
		typeRef.Nullable = true
	}
	return typeRef, nil
}

func (p *typeExprParser) parseOneArgType(name string) (TypeRef, error) {
	if err := p.expect('<'); err != nil {
		return TypeRef{}, p.wrapTypeArgError(name, err)
	}
	elem, err := p.parseType()
	if err != nil {
		return TypeRef{}, err
	}
	if err := p.expect('>'); err != nil {
		return TypeRef{}, p.wrapTypeArgError(name, err)
	}
	return elem, nil
}

func (p *typeExprParser) parseTwoArgType(name string) (TypeRef, TypeRef, error) {
	if err := p.expect('<'); err != nil {
		return TypeRef{}, TypeRef{}, p.wrapTypeArgError(name, err)
	}
	key, err := p.parseType()
	if err != nil {
		return TypeRef{}, TypeRef{}, err
	}
	if err := p.expect(','); err != nil {
		return TypeRef{}, TypeRef{}, p.wrapTypeArgError(name, err)
	}
	value, err := p.parseType()
	if err != nil {
		return TypeRef{}, TypeRef{}, err
	}
	if err := p.expect('>'); err != nil {
		return TypeRef{}, TypeRef{}, p.wrapTypeArgError(name, err)
	}
	return key, value, nil
}

func (p *typeExprParser) parseIdent() (string, error) {
	p.skipSpaces()
	start := p.pos
	if p.done() {
		return "", p.errorf("expected type name")
	}
	for !p.done() {
		r := rune(p.input[p.pos])
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_') {
			break
		}
		p.pos++
	}
	if start == p.pos {
		return "", p.errorf("expected type name")
	}
	value := p.input[start:p.pos]
	if unicode.IsDigit(rune(value[0])) {
		return "", p.errorf("type name must not start with a digit")
	}
	return value, nil
}

func (p *typeExprParser) expect(ch byte) error {
	p.skipSpaces()
	if !p.consume(ch) {
		return p.errorf("expected %q", string(ch))
	}
	return nil
}

func (p *typeExprParser) consume(ch byte) bool {
	if p.done() || p.input[p.pos] != ch {
		return false
	}
	p.pos++
	return true
}

func (p *typeExprParser) skipSpaces() {
	for !p.done() && unicode.IsSpace(rune(p.input[p.pos])) {
		p.pos++
	}
}

func (p *typeExprParser) done() bool {
	return p.pos >= len(p.input)
}

func (p *typeExprParser) errorf(format string, args ...any) error {
	return fmt.Errorf("invalid type expression %q at byte %d: %s", p.input, p.pos, fmt.Sprintf(format, args...))
}

func (p *typeExprParser) wrapTypeArgError(name string, err error) error {
	if strings.Contains(err.Error(), "invalid type expression") {
		return err
	}
	return p.errorf("%s type arguments are invalid: %v", name, err)
}
