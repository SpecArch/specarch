// code_facts_dart reads Dart source by its syntax and writes the facts
// specarch extract dart reads, as a code-facts dump (docs/reading-code.md,
// ADR-092). It is run by tools/code-facts/dump-dart.sh in the root of the
// repository, which gives it the folder's path, the commit and the tracked
// Dart files on standard input, one per line.
//
//   dart run code_facts_dart <path> <commit> < files
//
// The program uses the analyzer's parseString, which gives syntax without
// resolving packages: no pub cache is read. It says what the syntax says
// and nothing more: imports, classes with their annotations, fields,
// constructors with their parameters, methods and functions, enums, the
// calls and instance creations, comparisons of a method, and reads of the
// environment, each with its file, line and column and its literal
// values. What a fact means is decided by specarch extract, in Go, so the
// rules of extract live in one place.

import 'dart:convert';
import 'dart:io';

import 'package:analyzer/dart/analysis/utilities.dart';
import 'package:analyzer/dart/ast/ast.dart';
import 'package:analyzer/dart/ast/visitor.dart';
import 'package:analyzer/source/line_info.dart';

const parserName = 'analyzer';
const parserVersion = '14.4.0';

// How deep a value is written; below this a value is written as its text.
const depth = 24;
const textLimit = 160;

final facts = <Map<String, Object?>>[];

String clip(String s) {
  var t = s.replaceAll(RegExp(r'\s+'), ' ');
  if (t.length > textLimit) t = '${t.substring(0, textLimit - 3)}...';
  return t;
}

class Reader extends RecursiveAstVisitor<void> {
  Reader(this.file, this.lines);

  final String file;
  final LineInfo lines;
  final scope = <String>[]; // the class and the member a node is in

  Map<String, Object?> at(AstNode node) {
    final l = lines.getLocation(node.offset);
    return {'file': file, 'line': l.lineNumber, 'column': l.columnNumber};
  }

  Map<String, Object?> atToken(int offset) {
    final l = lines.getLocation(offset);
    return {'line': l.lineNumber, 'column': l.columnNumber};
  }

  void emit(String kind, AstNode node, Map<String, Object?> fields) {
    facts.add({'kind': kind, ...at(node), ...fields});
  }

  String within() => scope.join('.');

  Map<String, Object?> where(AstNode node) {
    final out = <String, Object?>{};
    final w = within();
    if (w.isNotEmpty) out['within'] = w;
    var loop = false, condition = false;
    AstNode? prev = node;
    for (var n = node.parent; n != null; prev = n, n = n.parent) {
      if (n is FunctionExpression && n.parent is! FunctionDeclaration && !out.containsKey('function')) {
        out['function'] = lines.getLocation(n.offset).lineNumber;
      }
      if (n is ForStatement || n is WhileStatement || n is DoStatement || n is ForElement) loop = true;
      if (n is IfStatement && prev != n.expression) condition = true;
      if (n is IfElement && prev != n.expression) condition = true;
      if (n is ConditionalExpression && prev != n.condition) condition = true;
      if (n is SwitchStatement || n is SwitchExpression) condition = true;
      if (n is MethodInvocation && (n.methodName.name == 'map' || n.methodName.name == 'forEach' || n.methodName.name == 'expand')) {
        if (prev is ArgumentList) loop = true;
      }
      // The branch of a switch or an if on a request's method it is in.
      if (!out.containsKey('branch')) {
        if (n is SwitchMember && n.parent is SwitchStatement && (n.parent as SwitchStatement).expression.toSource().endsWith('.method')) {
          out['branch'] = n is SwitchCase
              ? n.expression.toSource()
              : n is SwitchPatternCase
                  ? n.guardedPattern.toSource()
                  : 'default';
        }
        if (n is IfStatement && prev == n.thenStatement) {
          final c = n.expression;
          if (c is BinaryExpression && c.operator.lexeme == '==' && c.leftOperand.toSource().endsWith('.method')) {
            out['branch'] = c.rightOperand.toSource();
          }
        }
      }
      if (n is ClassMember || n is CompilationUnitMember) break;
    }
    if (loop) out['inLoop'] = true;
    if (condition) out['inCondition'] = true;
    return out;
  }

  List<Map<String, Object?>> annotations(NodeList<Annotation> metadata) => [
        for (final a in metadata)
          {
            'name': a.name.toSource() + (a.constructorName != null ? '.${a.constructorName!.name}' : ''),
            if (a.arguments != null) 'arguments': arguments(a.arguments!, 0),
          }
      ];

  List<Map<String, Object?>> arguments(ArgumentList list, int d) => [
        for (final a in list.arguments)
          if (a is NamedArgument)
            {'name': a.name.lexeme, 'value': value(a.argumentExpression, d + 1)}
          else
            {'value': value(a.argumentExpression, d + 1)}
      ];

  Map<String, Object?> value(Expression e, int d) {
    if (d > depth) return {'text': clip(e.toSource())};
    if (e is ParenthesizedExpression) return value(e.expression, d);
    if (e is SimpleStringLiteral) return {'string': e.value};
    if (e is AdjacentStrings && e.stringValue != null) return {'string': e.stringValue};
    if (e is StringInterpolation) {
      return {
        'interpolated': [
          for (final el in e.elements)
            if (el is InterpolationString)
              if (el.value.isNotEmpty) {'text': el.value} else null
            else if (el is InterpolationExpression)
              {'expression': el.expression.toSource()}
        ].whereType<Map<String, Object?>>().toList()
      };
    }
    if (e is IntegerLiteral) return {'integer': e.literal.lexeme};
    if (e is DoubleLiteral) return {'double': e.literal.lexeme};
    if (e is BooleanLiteral) return {'boolean': e.value};
    if (e is NullLiteral) return {'null': true};
    if (e is SimpleIdentifier || e is PrefixedIdentifier || e is PropertyAccess) return {'name': e.toSource()};
    if (e is MethodInvocation) {
      return {
        'call': {
          'line': lines.getLocation(e.offset).lineNumber,
          if (e.realTarget != null) 'target': value(e.realTarget!, d + 1),
          'name': e.methodName.name,
          'arguments': arguments(e.argumentList, d),
        }
      };
    }
    if (e is InstanceCreationExpression) {
      final type = e.constructorName.type;
      final prefix = type.importPrefix != null ? '${type.importPrefix!.name.lexeme}.' : '';
      return {
        'call': {
          'line': lines.getLocation(e.offset).lineNumber,
          'name': prefix + type.name.lexeme + (e.constructorName.name != null ? '.${e.constructorName.name!.name}' : ''),
          'arguments': arguments(e.argumentList, d),
          if (e.isConst) 'const': true,
        }
      };
    }
    if (e is ListLiteral) {
      return {
        'list': [
          for (final el in e.elements) el is Expression ? value(el, d + 1) : {'text': clip(el.toSource())}
        ]
      };
    }
    if (e is SetOrMapLiteral) {
      return {
        'map': [
          for (final el in e.elements)
            if (el is MapLiteralEntry)
              {'key': value(el.key, d + 1), 'value': value(el.value, d + 1)}
            else
              {'text': clip(el.toSource())}
        ]
      };
    }
    if (e is FunctionExpression) {
      final out = <String, Object?>{
        'line': lines.getLocation(e.offset).lineNumber,
        'parameters': [for (final p in e.parameters?.parameters ?? <FormalParameter>[]) p.name?.lexeme ?? ''],
      };
      final body = e.body;
      if (body is ExpressionFunctionBody) {
        out['returns'] = [value(body.expression, d + 1)];
      } else if (body is BlockFunctionBody) {
        final returns = <Map<String, Object?>>[];
        body.block.accept(_Returns((r) => returns.add(value(r, d + 1))));
        out['returns'] = returns;
      }
      return {'function': out};
    }
    if (e is AssignmentExpression) {
      return {
        'assign': {'target': e.leftHandSide.toSource(), 'value': value(e.rightHandSide, d + 1)}
      };
    }
    if (e is AwaitExpression) return value(e.expression, d);
    if (e is PostfixExpression && e.operator.lexeme == '!') return value(e.operand, d);
    if (e is AsExpression) return value(e.expression, d);
    return {'text': clip(e.toSource())};
  }

  Map<String, Object?> parameter(FormalParameter p) {
    final out = <String, Object?>{'name': p.name?.lexeme ?? ''};
    if (p.type != null) out['type'] = p.type!.toSource();
    if (p.isNamed) out['named'] = true;
    if (p.isRequired) out['required'] = true;
    if (p.defaultClause != null) out['default'] = value(p.defaultClause!.value, 0);
    final m = p.metadata;
    if (m.isNotEmpty) out['annotations'] = annotations(m);
    if (p is FieldFormalParameter) out['field'] = true;
    return out;
  }

  @override
  void visitImportDirective(ImportDirective node) {
    emit('import', node, {
      'uri': node.uri.stringValue ?? node.uri.toSource(),
      if (node.prefix != null) 'prefix': node.prefix!.name,
    });
    super.visitImportDirective(node);
  }

  @override
  void visitClassDeclaration(ClassDeclaration node) {
    final name = node.namePart.typeName.lexeme;
    emit('class', node, {
      'name': name,
      if (node.abstractKeyword != null) 'abstract': true,
      if (node.extendsClause != null) 'extends': node.extendsClause!.superclass.toSource(),
      if (node.withClause != null) 'with': [for (final t in node.withClause!.mixinTypes) t.toSource()],
      if (node.implementsClause != null) 'implements': [for (final t in node.implementsClause!.interfaces) t.toSource()],
      if (node.metadata.isNotEmpty) 'annotations': annotations(node.metadata),
    });
    scope.add(name);
    super.visitClassDeclaration(node);
    scope.removeLast();
  }

  @override
  void visitEnumDeclaration(EnumDeclaration node) {
    final name = node.namePart.typeName.lexeme;
    emit('enum', node, {
      'name': name,
      if (node.metadata.isNotEmpty) 'annotations': annotations(node.metadata),
      'values': [
        for (final c in node.body.constants)
          {
            'name': c.name.lexeme,
            ...atToken(c.name.offset),
            if (c.metadata.isNotEmpty) 'annotations': annotations(c.metadata),
          }
      ],
    });
    scope.add(name);
    super.visitEnumDeclaration(node);
    scope.removeLast();
  }

  @override
  void visitFieldDeclaration(FieldDeclaration node) {
    for (final v in node.fields.variables) {
      facts.add({
        'kind': 'field',
        'file': file,
        ...atToken(v.name.offset),
        'within': within(),
        'name': v.name.lexeme,
        if (node.fields.type != null) 'type': node.fields.type!.toSource(),
        if (node.isStatic) 'static': true,
        if (node.fields.isFinal) 'final': true,
        if (node.metadata.isNotEmpty) 'annotations': annotations(node.metadata),
        if (v.initializer != null) 'value': value(v.initializer!, 0),
      });
    }
    super.visitFieldDeclaration(node);
  }

  @override
  void visitTopLevelVariableDeclaration(TopLevelVariableDeclaration node) {
    for (final v in node.variables.variables) {
      facts.add({
        'kind': 'variable',
        'file': file,
        ...atToken(v.name.offset),
        'name': v.name.lexeme,
        if (node.variables.type != null) 'type': node.variables.type!.toSource(),
        if (node.variables.isConst) 'const': true,
        if (v.initializer != null) 'value': value(v.initializer!, 0),
      });
    }
    super.visitTopLevelVariableDeclaration(node);
  }

  @override
  void visitVariableDeclarationStatement(VariableDeclarationStatement node) {
    for (final v in node.variables.variables) {
      if (v.initializer == null) continue;
      facts.add({
        'kind': 'variable',
        'file': file,
        ...atToken(v.name.offset),
        'name': v.name.lexeme,
        'within': within(),
        if (node.variables.type != null) 'type': node.variables.type!.toSource(),
        'value': value(v.initializer!, 0),
      });
    }
    super.visitVariableDeclarationStatement(node);
  }

  @override
  void visitConstructorDeclaration(ConstructorDeclaration node) {
    final name = node.name?.lexeme ?? '';
    emit('constructor', node, {
      'within': within(),
      'name': name,
      if (node.factoryKeyword != null) 'factory': true,
      if (node.redirectedConstructor != null) 'redirect': node.redirectedConstructor!.toSource(),
      'parameters': [for (final p in node.parameters.parameters) parameter(p)],
      if (node.metadata.isNotEmpty) 'annotations': annotations(node.metadata),
    });
    scope.add(name.isEmpty ? 'new' : name);
    super.visitConstructorDeclaration(node);
    scope.removeLast();
  }

  @override
  void visitMethodDeclaration(MethodDeclaration node) {
    emit('method', node, {
      'within': within(),
      'name': node.name.lexeme,
      'parameters': [for (final p in node.parameters?.parameters ?? <FormalParameter>[]) parameter(p)],
      if (node.returnType != null) 'returns': node.returnType!.toSource(),
      if (node.isStatic) 'static': true,
      if (node.isGetter) 'getter': true,
      if (node.metadata.isNotEmpty) 'annotations': annotations(node.metadata),
      ..._bodyReturns(node.body),
    });
    scope.add(node.name.lexeme);
    super.visitMethodDeclaration(node);
    scope.removeLast();
  }

  @override
  void visitFunctionDeclaration(FunctionDeclaration node) {
    final top = scope.isEmpty;
    emit('method', node, {
      if (!top) 'within': within(),
      'name': node.name.lexeme,
      'parameters': [for (final p in node.functionExpression.parameters?.parameters ?? <FormalParameter>[]) p.name == null ? <String, Object?>{} : parameter(p)],
      if (node.returnType != null) 'returns': node.returnType!.toSource(),
      if (node.metadata.isNotEmpty) 'annotations': annotations(node.metadata),
      ..._bodyReturns(node.functionExpression.body),
    });
    scope.add(node.name.lexeme);
    super.visitFunctionDeclaration(node);
    scope.removeLast();
  }

  Map<String, Object?> _bodyReturns(FunctionBody body) {
    final returns = <Map<String, Object?>>[];
    if (body is ExpressionFunctionBody) {
      returns.add(value(body.expression, 0));
    } else if (body is BlockFunctionBody) {
      body.block.accept(_Returns((r) => returns.add(value(r, 0))));
    }
    return returns.isEmpty ? {} : {'returnValues': returns};
  }

  // cascadeOf is the variable a cascade a call is a section of is the
  // initial value of: app in final app = Router()..get(...).
  String? cascadeOf(AstNode node) {
    AstNode? n = node.parent;
    while (n != null && n is! CascadeExpression) {
      if (n is Statement || n is ClassMember) return null;
      n = n.parent;
    }
    return n == null ? null : assignedTo(n);
  }

  String? assignedTo(AstNode node) {
    var n = node.parent;
    while (n is AwaitExpression || n is ParenthesizedExpression) {
      n = n!.parent;
    }
    if (n is VariableDeclaration && n.initializer != null) return n.name.lexeme;
    return null;
  }

  @override
  void visitMethodInvocation(MethodInvocation node) {
    emit('call', node, {
      if (node.realTarget != null) 'target': value(node.realTarget!, 0),
      if (node.isCascaded) 'cascaded': true,
      if (node.isCascaded && cascadeOf(node) != null) 'cascadeOf': cascadeOf(node),
      'name': node.methodName.name,
      'arguments': arguments(node.argumentList, 0),
      if (assignedTo(node) != null) 'assignedTo': assignedTo(node),
      ...where(node),
    });
    super.visitMethodInvocation(node);
  }

  @override
  void visitInstanceCreationExpression(InstanceCreationExpression node) {
    final v = value(node, 0)['call'] as Map<String, Object?>;
    v.remove('line');
    emit('call', node, {
      'name': v['name'],
      'arguments': v['arguments'],
      if (node.isConst) 'const': true,
      if (assignedTo(node) != null) 'assignedTo': assignedTo(node),
      ...where(node),
    });
    super.visitInstanceCreationExpression(node);
  }

  @override
  void visitBinaryExpression(BinaryExpression node) {
    if ((node.operator.lexeme == '==' || node.operator.lexeme == '!=') && node.leftOperand.toSource().endsWith('.method')) {
      emit('compare', node, {'on': node.leftOperand.toSource(), 'value': value(node.rightOperand, 0), ...where(node)});
    }
    super.visitBinaryExpression(node);
  }

  @override
  void visitSwitchStatement(SwitchStatement node) {
    if (node.expression.toSource().endsWith('.method')) {
      for (final m in node.members) {
        if (m is SwitchCase) {
          emit('compare', m, {'on': node.expression.toSource(), 'value': value(m.expression, 0), ...where(node)});
        } else if (m is SwitchPatternCase) {
          emit('compare', m, {'on': node.expression.toSource(), 'value': {'text': clip(m.guardedPattern.toSource())}, ...where(node)});
        } else if (m is SwitchDefault) {
          emit('compare', m, {'on': node.expression.toSource(), 'default': true, ...where(node)});
        }
      }
    }
    super.visitSwitchStatement(node);
  }

  @override
  void visitIndexExpression(IndexExpression node) {
    final t = node.realTarget.toSource();
    if (t.endsWith('environment')) {
      emit('index', node, {'on': t, 'key': value(node.index, 0), ...where(node)});
    }
    super.visitIndexExpression(node);
  }
}

class _Returns extends RecursiveAstVisitor<void> {
  _Returns(this.add);
  final void Function(Expression) add;

  @override
  void visitReturnStatement(ReturnStatement node) {
    if (node.expression != null) add(node.expression!);
  }

  @override
  void visitFunctionExpression(FunctionExpression node) {}
}

String json(Object? v) => jsonEncode(v);

void main(List<String> args) {
  if (args.length != 2) {
    stderr.writeln('usage: code_facts_dart <path> <commit> < files');
    exit(2);
  }
  final folder = args[0], commit = args[1];
  final files = <String>[];
  String? line;
  while ((line = stdin.readLineSync()) != null) {
    if (line!.isNotEmpty) files.add(line);
  }
  files.sort();
  final listed = <Map<String, Object?>>[];
  for (final f in files) {
    final src = File(f).readAsStringSync();
    final result = parseString(content: src, path: f, throwIfDiagnostics: false);
    listed.add({'file': f, 'syntaxErrors': result.errors.length});
    result.unit.accept(Reader(f, result.lineInfo));
  }
  final indexed = [for (var i = 0; i < facts.length; i++) (facts[i], i)];
  indexed.sort((a, b) {
    final fa = a.$1, fb = b.$1;
    final c = (fa['file'] as String).compareTo(fb['file'] as String);
    if (c != 0) return c;
    final l = (fa['line'] as int) - (fb['line'] as int);
    if (l != 0) return l;
    final col = (fa['column'] as int) - (fb['column'] as int);
    if (col != 0) return col;
    return a.$2 - b.$2;
  });
  final out = StringBuffer()
    ..writeln('{')
    ..writeln('    "codeFacts": 1,')
    ..writeln('    "language": "dart",')
    ..writeln('    "parser": {"name": ${json(parserName)}, "version": ${json(parserVersion)}},')
    ..writeln('    "path": ${json(folder)},')
    ..writeln('    "commit": ${json(commit)},')
    ..writeln('    "files": [')
    ..writeln([for (final f in listed) '        {"file": ${json(f['file'])}, "syntaxErrors": ${f['syntaxErrors']}}'].join(',\n'))
    ..writeln('    ],')
    ..writeln('    "facts": [')
    ..writeln([for (final f in indexed) '        ${json(f.$1)}'].join(',\n'))
    ..writeln('    ]')
    ..writeln('}');
  stdout.write(out.toString());
}
