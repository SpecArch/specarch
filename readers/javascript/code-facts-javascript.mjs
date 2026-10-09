#!/usr/bin/env node
// code-facts-javascript reads JavaScript and TypeScript with the
// TypeScript compiler and writes the facts specarch extract javascript
// reads, as a code-facts dump (docs/reading-code.md, ADR-089). It is run
// by tools/code-facts/dump-javascript.sh in the root of the repository,
// which gives it the folder's path, the commit and the tracked files
// under the folder on standard input, one per line.
//
//   code-facts-javascript <path> <commit> < files
//
// The program says what the syntax says, with the declarations the
// compiler's checker resolves among the files read and the types that
// come with TypeScript, and nothing more: imports, configuration files,
// type declarations, functions, the calls and JSX elements whose names the
// tables below list, variables, reads of the environment and of a
// request, each with its file, line and column and its literal values.
// No file outside the list is read: no node_modules, no package's types,
// so a name a package declares is known only by the import that names
// it. What a fact means is decided by specarch extract, in Go, so the
// rules of extract live in one place.

import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';

const require = createRequire(import.meta.url);
const ts = require('typescript');

const parserName = 'TypeScript';
const parserVersion = '6.0.3';

// The modules whose calls and elements the program writes wherever they
// are used: the libraries specarch extract javascript knows. A module is
// known when its name is one of these or starts with one and a slash.
const knownModules = [
  'express', 'fastify', 'zod', 'yup', 'joi', '@hapi/joi', 'react-router', 'react-router-dom',
  'react-hook-form', '@hookform/resolvers', 'formik', 'axios', 'i18next', 'react-i18next', 'next-intl',
];

// The names of the calls the program writes whatever their callee is:
// the methods of a router, a schema, a form, a client or a catalogue that
// are called on a value the program cannot name by its module.
const callNames = new Set([
  // routes and their middleware
  'get', 'post', 'put', 'patch', 'delete', 'del', 'all', 'options', 'head', 'use', 'route', 'register', 'listen', 'Router', 'addHook',
  // validation
  'parse', 'safeParse', 'parseAsync', 'safeParseAsync', 'validate', 'validateSync', 'validateAsync', 'cast',
  // forms
  'useForm', 'useFormik', 'getFieldProps', 'useField',
  // clients
  'fetch', 'request', 'create',
  // message catalogues
  't', 'useTranslation', 'useTranslations', 'getTranslations', 'init',
  // screens
  'createBrowserRouter', 'createHashRouter', 'createMemoryRouter', 'useRoutes', 'navigate', 'useNavigate', 'redirect',
  // modules
  'require',
]);

// The elements of HTML the program writes: a page's heading, a form and
// its controls, and a link.
const intrinsicElements = new Set(['h1', 'form', 'input', 'select', 'textarea', 'a']);

// The members of a request that the program writes reads of.
const requestMembers = new Set(['body', 'params', 'query', 'headers', 'method']);

// The calls of an array that run a function once per item: a function
// given to one runs in a loop.
const loopCalls = new Set(['map', 'forEach', 'flatMap', 'filter', 'reduce', 'some', 'every']);

const sourceExtensions = ['.ts', '.tsx', '.mts', '.cts', '.js', '.jsx', '.mjs', '.cjs'];
const configNames = new Set(['tsconfig.json', 'jsconfig.json']);

// How deep a value is written; a value below this is written as its text.
const depth = 32;
const textLimit = 160;

function fail(message) {
  process.stderr.write(`code-facts-javascript: ${message}\n`);
  process.exit(2);
}

if (process.argv.length !== 4) {
  fail('usage: code-facts-javascript <path> <commit> < files');
}
if (ts.version !== parserVersion) {
  fail(`the TypeScript compiler installed is ${ts.version}, and this reader writes dumps of ${parserVersion}; run npm ci in readers/javascript`);
}
const [folder, commit] = process.argv.slice(2);
const root = process.cwd();
const listed = fs.readFileSync(0, 'utf8').split('\n').filter((l) => l !== '');
const tracked = new Set(listed);
const slash = (p) => p.split(path.sep).join('/');
const rel = (f) => slash(path.relative(root, path.resolve(root, f)));
const isSource = (f) => sourceExtensions.some((e) => f.endsWith(e));
const sources = listed.filter(isSource).sort();
const configs = listed.filter((f) => configNames.has(path.posix.basename(f))).sort();

const trackedDirs = new Set();
for (const f of listed) {
  for (let d = path.posix.dirname(f); d !== '.' && d !== '/' && !trackedDirs.has(d); d = path.posix.dirname(d)) {
    trackedDirs.add(d);
  }
}

const libDir = path.dirname(ts.getDefaultLibFilePath({ target: ts.ScriptTarget.ESNext }));
const isLib = (f) => path.resolve(f).startsWith(libDir + path.sep);
const allowed = (f) => isLib(f) || tracked.has(rel(f));
const readAllowed = (f) => (allowed(f) && fs.existsSync(f) ? fs.readFileSync(f, 'utf8') : undefined);

const facts = [];
const fileList = [];

// The configuration files, as the compiler reads them; the one nearest
// the folder's root gives the program its module paths.
let programPaths = null;
const parseHost = {
  useCaseSensitiveFileNames: true,
  readDirectory: () => [],
  fileExists: (f) => allowed(f) && fs.existsSync(f),
  readFile: readAllowed,
};
const configFacts = [];
for (const file of [...configs].sort((a, b) => a.split('/').length - b.split('/').length || (a < b ? -1 : 1))) {
  const abs = path.resolve(root, file);
  const read = ts.readConfigFile(abs, readAllowed);
  const fact = { kind: 'configuration', file, line: 1, column: 1 };
  if (read.error) {
    fact.error = ts.flattenDiagnosticMessageText(read.error.messageText, ' ');
    configFacts.push(fact);
    continue;
  }
  const parsed = ts.parseJsonConfigFileContent(read.config, parseHost, path.dirname(abs), undefined, abs);
  const o = parsed.options;
  fact.checkJs = o.checkJs === true;
  fact.allowJs = o.allowJs === true;
  fact.strict = o.strict === true;
  if (read.config && typeof read.config.extends !== 'undefined') {
    fact.extends = [].concat(read.config.extends).map(String);
    fact.extendsRead = !parsed.errors.some((e) => e.code === 6053 || e.code === 5083);
  }
  if (programPaths === null && (o.paths || o.baseUrl)) {
    programPaths = { paths: o.paths, baseUrl: o.baseUrl, pathsBasePath: o.pathsBasePath };
  }
  configFacts.push(fact);
}

const options = {
  allowJs: true,
  checkJs: false,
  noEmit: true,
  jsx: ts.JsxEmit.Preserve,
  target: ts.ScriptTarget.ESNext,
  module: ts.ModuleKind.ESNext,
  moduleResolution: ts.ModuleResolutionKind.Bundler,
  allowImportingTsExtensions: true,
  strict: true,
  types: [],
  skipLibCheck: true,
  noResolve: false,
  ...(programPaths || {}),
};
const host = ts.createCompilerHost(options, true);
host.getCurrentDirectory = () => root;
host.fileExists = (f) => allowed(f) && fs.existsSync(f);
host.readFile = readAllowed;
host.directoryExists = (d) => isLib(d) || path.resolve(d) === libDir || trackedDirs.has(rel(d)) || rel(d) === '';
host.getDirectories = () => [];
host.realpath = (f) => f;
const getSourceFile = host.getSourceFile.bind(host);
host.getSourceFile = (f, ...rest) => (allowed(f) ? getSourceFile(f, ...rest) : undefined);

const program = ts.createProgram({ rootNames: sources.map((f) => path.resolve(root, f)), options, host });
const checker = program.getTypeChecker();

let sf = null; // the file being read

function where(node) {
  const start = node.getStart(sf);
  const lc = sf.getLineAndCharacterOfPosition(start);
  return { line: lc.line + 1, column: lc.character + 1 };
}

function at(node) {
  const file = node.getSourceFile();
  const lc = file.getLineAndCharacterOfPosition(node.getStart(file));
  return { file: rel(file.fileName), line: lc.line + 1, column: lc.character + 1 };
}

function text(node) {
  let t = node.getText(sf).replace(/\s+/g, ' ');
  if (t.length > textLimit) {
    t = t.slice(0, textLimit - 3) + '...';
  }
  return t;
}

function emit(kind, node, fields) {
  facts.push({ kind, file: rel(sf.fileName), ...where(node), ...fields });
}

const isFunctionLike = (n) => ts.isFunctionDeclaration(n) || ts.isFunctionExpression(n) || ts.isArrowFunction(n) || ts.isMethodDeclaration(n);

// The node a declaration is cited by: a variable given a function is
// cited by the function, so that a name and the function fact meet.
function declarationNode(d) {
  if (ts.isVariableDeclaration(d) && d.initializer) {
    const init = unwrap(d.initializer);
    if (ts.isArrowFunction(init) || ts.isFunctionExpression(init)) {
      return init;
    }
  }
  if (ts.isExportAssignment(d)) {
    return unwrap(d.expression);
  }
  return d;
}

// The module and the name an import binds a name to, by its syntax.
function importOf(d) {
  if (ts.isImportSpecifier(d)) {
    return { module: d.parent.parent.parent.moduleSpecifier.text, imported: (d.propertyName || d.name).text };
  }
  if (ts.isImportClause(d)) {
    return { module: d.parent.moduleSpecifier.text, imported: 'default' };
  }
  if (ts.isNamespaceImport(d)) {
    return { module: d.parent.parent.moduleSpecifier.text, imported: '*' };
  }
  if (ts.isImportEqualsDeclaration(d) && ts.isExternalModuleReference(d.moduleReference) && ts.isStringLiteral(d.moduleReference.expression)) {
    return { module: d.moduleReference.expression.text, imported: '*' };
  }
  // CommonJS: const x = require('m'), and const { a } = require('m').
  if (ts.isVariableDeclaration(d) && d.initializer) {
    const m = requiredModule(d.initializer);
    if (m !== null && ts.isIdentifier(d.name)) {
      return { module: m, imported: '*' };
    }
  }
  if (ts.isBindingElement(d) && ts.isObjectBindingPattern(d.parent) && ts.isVariableDeclaration(d.parent.parent) && d.parent.parent.initializer) {
    const m = requiredModule(d.parent.parent.initializer);
    if (m !== null) {
      return { module: m, imported: (d.propertyName || d.name).getText() };
    }
  }
  return null;
}

function requiredModule(node) {
  const n = unwrap(node);
  if (ts.isCallExpression(n) && ts.isIdentifier(n.expression) && n.expression.text === 'require' && n.arguments.length === 1 && ts.isStringLiteralLike(n.arguments[0])) {
    return n.arguments[0].text;
  }
  return null;
}

// What a name resolves to: the import that binds it, and the declaration
// among the files read, or the library of TypeScript, that declares it.
function resolveName(id) {
  const out = {};
  let sym = checker.getSymbolAtLocation(id);
  if (!sym && ts.isShorthandPropertyAssignment(id.parent)) {
    sym = checker.getShorthandAssignmentValueSymbol(id.parent);
  }
  if (!sym) {
    return out;
  }
  const first = sym.declarations && sym.declarations[0];
  if (first) {
    const imp = importOf(first);
    if (imp) {
      out.import = imp;
    }
  }
  let target = sym;
  if (sym.flags & ts.SymbolFlags.Alias) {
    try {
      target = checker.getAliasedSymbol(sym);
    } catch {
      target = null;
    }
  }
  const d = target && target.declarations && target.declarations[0];
  if (d) {
    const file = d.getSourceFile().fileName;
    if (isLib(file)) {
      out.library = true;
    } else if (tracked.has(rel(file))) {
      if (!(out.import && d === first && importOf(d))) {
        out.declaration = at(declarationNode(d));
      }
      if (ts.isParameter(d) && isFunctionLike(d.parent)) {
        out.parameter = { index: d.parent.parameters.indexOf(d), function: at(d.parent) };
      }
    }
  }
  return out;
}

function unwrap(node) {
  let n = node;
  for (;;) {
    if (ts.isParenthesizedExpression(n) || ts.isAsExpression(n) || ts.isSatisfiesExpression(n) || ts.isNonNullExpression(n) || ts.isTypeAssertionExpression(n) || ts.isAwaitExpression(n)) {
      n = n.expression;
    } else {
      return n;
    }
  }
}

function propertyName(name) {
  if (ts.isIdentifier(name) || ts.isPrivateIdentifier(name) || ts.isStringLiteralLike(name) || ts.isNumericLiteral(name)) {
    return name.text;
  }
  return null;
}

// value writes an expression as one of its keys: a literal, a template
// by its parts, a name with what it resolves to, a member, a call, an
// object, an array, a function by where it is, an element, or its text.
function value(node, d = 0) {
  const n = unwrap(node);
  if (d > depth) {
    return { text: text(n) };
  }
  switch (n.kind) {
    case ts.SyntaxKind.StringLiteral:
    case ts.SyntaxKind.NoSubstitutionTemplateLiteral:
      return { string: n.text };
    case ts.SyntaxKind.TemplateExpression: {
      const parts = [];
      if (n.head.text !== '') parts.push({ text: n.head.text });
      for (const s of n.templateSpans) {
        parts.push({ expression: value(s.expression, d + 1) });
        if (s.literal.text !== '') parts.push({ text: s.literal.text });
      }
      return { template: parts };
    }
    case ts.SyntaxKind.NumericLiteral:
      return { number: n.text };
    case ts.SyntaxKind.TrueKeyword:
      return { boolean: true };
    case ts.SyntaxKind.FalseKeyword:
      return { boolean: false };
    case ts.SyntaxKind.NullKeyword:
      return { null: true };
    case ts.SyntaxKind.RegularExpressionLiteral:
      return { regex: n.text };
    case ts.SyntaxKind.Identifier:
      return { name: n.text, ...resolveName(n) };
    case ts.SyntaxKind.MetaProperty:
      return { name: text(n) };
    case ts.SyntaxKind.PropertyAccessExpression:
      return { member: { object: value(n.expression, d + 1), name: n.name.text } };
    case ts.SyntaxKind.ElementAccessExpression:
      if (ts.isStringLiteralLike(n.argumentExpression) || ts.isNumericLiteral(n.argumentExpression)) {
        return { member: { object: value(n.expression, d + 1), name: n.argumentExpression.text, computed: true } };
      }
      return { text: text(n) };
    case ts.SyntaxKind.CallExpression:
      return { call: callOf(n, d) };
    case ts.SyntaxKind.NewExpression:
      return { new: callOf(n, d) };
    case ts.SyntaxKind.ObjectLiteralExpression:
      return { object: n.properties.map((p) => property(p, d)) };
    case ts.SyntaxKind.ArrayLiteralExpression:
      return { array: n.elements.map((e) => (ts.isSpreadElement(e) ? { spread: value(e.expression, d + 1) } : value(e, d + 1))) };
    case ts.SyntaxKind.ArrowFunction:
    case ts.SyntaxKind.FunctionExpression:
      return { function: at(n) };
    case ts.SyntaxKind.JsxElement:
    case ts.SyntaxKind.JsxSelfClosingElement:
      return { jsx: element(n, d) };
    case ts.SyntaxKind.PrefixUnaryExpression:
      if (n.operator === ts.SyntaxKind.MinusToken && ts.isNumericLiteral(n.operand)) {
        return { number: '-' + n.operand.text };
      }
      return { text: text(n) };
    default:
      if (n.kind === ts.SyntaxKind.UndefinedKeyword) {
        return { name: 'undefined' };
      }
      return { text: text(n) };
  }
}

function property(p, d) {
  if (ts.isPropertyAssignment(p)) {
    const key = propertyName(p.name);
    const line = where(p).line;
    return key === null ? { computed: text(p.name), line, value: value(p.initializer, d + 1) } : { key, line, value: value(p.initializer, d + 1) };
  }
  if (ts.isShorthandPropertyAssignment(p)) {
    return { key: p.name.text, line: where(p).line, value: { name: p.name.text, ...resolveName(p.name) } };
  }
  if (ts.isSpreadAssignment(p)) {
    return { spread: value(p.expression, d + 1) };
  }
  if (ts.isMethodDeclaration(p)) {
    const key = propertyName(p.name);
    return key === null ? { computed: text(p.name), value: { function: at(p) } } : { key, value: { function: at(p) } };
  }
  return { computed: text(p) };
}

function callOf(n, d) {
  const c = { callee: value(n.expression, d + 1), arguments: (n.arguments || []).map((a) => (ts.isSpreadElement(a) ? { spread: value(a.expression, d + 1) } : value(a, d + 1))) };
  if (n.typeArguments && n.typeArguments.length > 0) {
    c.typeArguments = n.typeArguments.map(typeOf);
  }
  return c;
}

function tagOf(tag) {
  if (ts.isIdentifier(tag) && /^[a-z]/.test(tag.text) && !tag.text.includes('.')) {
    return { intrinsic: tag.text };
  }
  return value(tag);
}

function opening(n) {
  return ts.isJsxElement(n) ? n.openingElement : n;
}

// element writes a JSX element: its tag, its attributes, and the literal
// text it holds.
function element(n, d = 0) {
  const o = opening(n);
  const e = { tag: tagOf(o.tagName), attributes: [] };
  for (const a of o.attributes.properties) {
    if (ts.isJsxSpreadAttribute(a)) {
      e.attributes.push({ spread: value(a.expression, d + 1) });
      continue;
    }
    const name = a.name.getText(sf);
    if (!a.initializer) {
      e.attributes.push({ name, value: { boolean: true } });
    } else if (ts.isStringLiteral(a.initializer)) {
      e.attributes.push({ name, value: { string: a.initializer.text } });
    } else if (ts.isJsxExpression(a.initializer) && a.initializer.expression) {
      e.attributes.push({ name, value: value(a.initializer.expression, d + 1) });
    } else if (ts.isJsxElement(a.initializer) || ts.isJsxSelfClosingElement(a.initializer)) {
      e.attributes.push({ name, value: { jsx: element(a.initializer, d + 1) } });
    }
  }
  if (ts.isJsxElement(n)) {
    const parts = [];
    for (const c of n.children) {
      if (ts.isJsxText(c)) {
        const t = c.text.replace(/\s+/g, ' ').trim();
        if (t !== '') parts.push({ text: t });
      } else if (ts.isJsxExpression(c) && c.expression) {
        parts.push({ expression: value(c.expression, d + 1) });
      }
    }
    if (parts.length > 0) e.children = parts;
  }
  return e;
}

// typeOf writes a type as its source states it, with the declaration
// among the files read that a name refers to.
function typeOf(node) {
  if (!node) return null;
  switch (node.kind) {
    case ts.SyntaxKind.StringKeyword: return { keyword: 'string' };
    case ts.SyntaxKind.NumberKeyword: return { keyword: 'number' };
    case ts.SyntaxKind.BooleanKeyword: return { keyword: 'boolean' };
    case ts.SyntaxKind.BigIntKeyword: return { keyword: 'bigint' };
    case ts.SyntaxKind.AnyKeyword: return { keyword: 'any' };
    case ts.SyntaxKind.UnknownKeyword: return { keyword: 'unknown' };
    case ts.SyntaxKind.VoidKeyword: return { keyword: 'void' };
    case ts.SyntaxKind.UndefinedKeyword: return { keyword: 'undefined' };
    case ts.SyntaxKind.NeverKeyword: return { keyword: 'never' };
    case ts.SyntaxKind.ObjectKeyword: return { keyword: 'object' };
    case ts.SyntaxKind.JSDocAllType: return { keyword: 'any' };
    case ts.SyntaxKind.JSDocUnknownType: return { keyword: 'unknown' };
    case ts.SyntaxKind.LiteralType: {
      const l = node.literal;
      if (l.kind === ts.SyntaxKind.NullKeyword) return { keyword: 'null' };
      if (ts.isStringLiteral(l)) return { literal: { string: l.text } };
      if (ts.isNumericLiteral(l)) return { literal: { number: l.text } };
      if (l.kind === ts.SyntaxKind.TrueKeyword) return { literal: { boolean: true } };
      if (l.kind === ts.SyntaxKind.FalseKeyword) return { literal: { boolean: false } };
      return { text: text(node) };
    }
    case ts.SyntaxKind.ParenthesizedType:
      return typeOf(node.type);
    case ts.SyntaxKind.JSDocTypeExpression:
      return typeOf(node.type);
    case ts.SyntaxKind.JSDocNullableType:
      return { union: [typeOf(node.type), { keyword: 'null' }] };
    case ts.SyntaxKind.JSDocNonNullableType:
      return typeOf(node.type);
    case ts.SyntaxKind.JSDocOptionalType:
      return { union: [typeOf(node.type), { keyword: 'undefined' }] };
    case ts.SyntaxKind.ArrayType:
      return { array: typeOf(node.elementType) };
    case ts.SyntaxKind.UnionType:
      return { union: node.types.map(typeOf) };
    case ts.SyntaxKind.TypeLiteral:
      return { object: node.members.filter((m) => ts.isPropertySignature(m)).map(member) };
    case ts.SyntaxKind.JSDocTypeLiteral:
      return { object: (node.jsDocPropertyTags || []).map(jsDocProperty) };
    case ts.SyntaxKind.TypeQuery: {
      const t = { typeof: node.exprName.getText(sf) };
      const id = ts.isQualifiedName(node.exprName) ? node.exprName.right : node.exprName;
      Object.assign(t, resolveName(id));
      delete t.parameter;
      return t;
    }
    case ts.SyntaxKind.TypeReference: {
      const t = { reference: node.typeName.getText(sf) };
      const id = ts.isQualifiedName(node.typeName) ? leftmost(node.typeName) : node.typeName;
      const r = resolveName(id);
      if (ts.isQualifiedName(node.typeName)) {
        if (r.import) t.import = r.import;
      } else {
        Object.assign(t, r);
        delete t.parameter;
      }
      if (node.typeArguments && node.typeArguments.length > 0) {
        t.arguments = node.typeArguments.map(typeOf);
      }
      return t;
    }
    case ts.SyntaxKind.ImportType: {
      const t = { reference: node.qualifier ? node.qualifier.getText(sf) : '', importType: ts.isLiteralTypeNode(node.argument) ? node.argument.literal.text : text(node.argument) };
      const type = checker.getTypeFromTypeNode(node);
      const sym = type && (type.aliasSymbol || type.symbol);
      const d = sym && sym.declarations && sym.declarations[0];
      if (d && tracked.has(rel(d.getSourceFile().fileName))) {
        t.declaration = at(declarationNode(d));
      }
      if (node.typeArguments && node.typeArguments.length > 0) {
        t.arguments = node.typeArguments.map(typeOf);
      }
      return t;
    }
    default:
      return { text: text(node) };
  }
}

function leftmost(q) {
  let n = q;
  while (ts.isQualifiedName(n)) n = n.left;
  return n;
}

function member(m) {
  const out = { name: propertyName(m.name) ?? text(m.name), ...where(m) };
  if (m.type) {
    out.type = typeOf(m.type);
  }
  if (m.questionToken) out.optional = true;
  if (m.modifiers && m.modifiers.some((x) => x.kind === ts.SyntaxKind.ReadonlyKeyword)) out.readonly = true;
  return out;
}

function jsDocProperty(tag) {
  const out = { name: tag.name.getText(sf), ...where(tag) };
  if (tag.typeExpression) {
    const t = tag.typeExpression.type;
    if (t && t.kind === ts.SyntaxKind.JSDocOptionalType) {
      out.type = typeOf(t.type);
      out.optional = true;
    } else {
      out.type = typeOf(tag.typeExpression);
    }
  }
  if (tag.isBracketed) out.optional = true;
  return out;
}

function decorators(node) {
  const list = ts.canHaveDecorators(node) ? ts.getDecorators(node) || [] : [];
  return list.map((dec) => {
    const e = unwrap(dec.expression);
    if (ts.isCallExpression(e)) {
      return { name: text(e.expression), arguments: e.arguments.map((a) => value(a)) };
    }
    return { name: text(e) };
  });
}

function isExported(node) {
  const mods = ts.canHaveModifiers(node) ? ts.getModifiers(node) || [] : [];
  if (mods.some((m) => m.kind === ts.SyntaxKind.ExportKeyword)) {
    return mods.some((m) => m.kind === ts.SyntaxKind.DefaultKeyword) ? 'default' : 'named';
  }
  return null;
}

function jsDocText(node) {
  const docs = node.jsDoc;
  if (!docs || docs.length === 0) return null;
  return docs.map((doc) => doc.getText(sf)).join('\n');
}

// The function a node is in, and whether it is in a loop or behind a
// condition there; a function given to an array's map or forEach runs in
// a loop.
function context(node) {
  const out = {};
  let loop = false;
  let condition = false;
  let n = node.parent;
  let prev = node;
  for (; n && !ts.isSourceFile(n); prev = n, n = n.parent) {
    if (isFunctionLike(n)) {
      out.within = at(n);
      const call = n.parent;
      if (call && ts.isCallExpression(call) && call.arguments.includes(n) && ts.isPropertyAccessExpression(call.expression) && loopCalls.has(call.expression.name.text)) {
        loop = true;
      }
      break;
    }
    switch (n.kind) {
      case ts.SyntaxKind.ForStatement:
      case ts.SyntaxKind.ForInStatement:
      case ts.SyntaxKind.ForOfStatement:
      case ts.SyntaxKind.WhileStatement:
      case ts.SyntaxKind.DoStatement:
        loop = true;
        break;
      case ts.SyntaxKind.IfStatement:
        if (prev !== n.expression) condition = true;
        // if (req.method === 'POST') { ... }: the branch of one value.
        if (!out.branch && prev === n.thenStatement && ts.isBinaryExpression(n.expression) &&
          (n.expression.operatorToken.kind === ts.SyntaxKind.EqualsEqualsEqualsToken || n.expression.operatorToken.kind === ts.SyntaxKind.EqualsEqualsToken)) {
          out.branch = { on: text(n.expression.left), value: value(n.expression.right) };
        }
        break;
      case ts.SyntaxKind.ConditionalExpression:
        if (prev !== n.condition) condition = true;
        break;
      case ts.SyntaxKind.SwitchStatement:
        condition = true;
        break;
      case ts.SyntaxKind.CaseClause:
        condition = true;
        // case 'GET': the branch of one value of the switch.
        if (!out.branch) out.branch = { on: text(n.parent.parent.expression), value: value(n.expression) };
        break;
      case ts.SyntaxKind.BinaryExpression:
        if (prev === n.right && (n.operatorToken.kind === ts.SyntaxKind.AmpersandAmpersandToken || n.operatorToken.kind === ts.SyntaxKind.BarBarToken || n.operatorToken.kind === ts.SyntaxKind.QuestionQuestionToken)) {
          condition = true;
        }
        break;
      default:
    }
  }
  if (loop) out.inLoop = true;
  if (condition) out.inCondition = true;
  return out;
}

// The root of a callee: the name a chain of members and calls starts at.
function calleeRoot(node) {
  let n = unwrap(node);
  for (;;) {
    if (ts.isPropertyAccessExpression(n) || ts.isElementAccessExpression(n)) n = unwrap(n.expression);
    else if (ts.isCallExpression(n)) n = unwrap(n.expression);
    else return n;
  }
}

function calleeName(node) {
  const n = unwrap(node);
  if (ts.isIdentifier(n)) return n.text;
  if (ts.isPropertyAccessExpression(n)) return n.name.text;
  return null;
}

const knownModule = (m) => knownModules.some((k) => m === k || m.startsWith(k + '/'));

// importedFrom is the module a name is imported from, or null.
function importedFrom(id) {
  if (!ts.isIdentifier(id)) return null;
  const r = resolveName(id);
  return r.import ? r.import.module : null;
}

function wantedCall(n) {
  // Only the outermost call of a chain is written; the calls it is made
  // on are in its callee.
  const p = n.parent;
  if (p && ts.isPropertyAccessExpression(p) && p.expression === n && p.parent && ts.isCallExpression(p.parent) && p.parent.expression === p) return false;
  const name = calleeName(n.expression);
  if (name !== null && callNames.has(name)) return true;
  const root = calleeRoot(n.expression);
  if (ts.isIdentifier(root)) {
    const m = importedFrom(root);
    if (m !== null) return true;
  }
  return false;
}

// assignedTo is the name a call's value is given to, when it is the
// initial value of a variable.
function assignedTo(node) {
  let n = node;
  while (n.parent && (ts.isParenthesizedExpression(n.parent) || ts.isAwaitExpression(n.parent) || ts.isAsExpression(n.parent) || ts.isSatisfiesExpression(n.parent) || ts.isNonNullExpression(n.parent))) {
    n = n.parent;
  }
  if (n.parent && ts.isVariableDeclaration(n.parent) && n.parent.initializer === n && ts.isIdentifier(n.parent.name)) {
    return n.parent.name.text;
  }
  return null;
}

// readAccess writes a read of the environment or of a request's members,
// a chain of names from its root.
function readAccess(node) {
  const chain = [];
  let n = node;
  for (;;) {
    if (ts.isPropertyAccessExpression(n)) {
      chain.unshift(n.name.text);
      n = n.expression;
    } else if (ts.isElementAccessExpression(n) && ts.isStringLiteralLike(n.argumentExpression)) {
      chain.unshift(n.argumentExpression.text);
      n = n.expression;
    } else if (ts.isElementAccessExpression(n)) {
      chain.unshift(null);
      n = n.expression;
    } else {
      break;
    }
  }
  let rootName = null;
  let parameter = null;
  if (ts.isMetaProperty(n)) {
    rootName = 'import.meta';
  } else if (ts.isIdentifier(n)) {
    rootName = n.text;
    const r = resolveName(n);
    parameter = r.parameter || null;
    if (rootName === 'process' && (r.declaration || r.parameter)) return null;
  } else {
    return null;
  }
  const env = (rootName === 'process' || rootName === 'import.meta') && chain[0] === 'env';
  const req = parameter !== null && requestMembers.has(chain[0]);
  if (!env && !req) return null;
  const fact = { chain: [rootName, ...chain.map((c) => (c === null ? '[computed]' : c))] };
  if (parameter) fact.parameter = parameter;
  return fact;
}

// directives is the prologue of a file or a function body: 'use server'.
function directives(statements) {
  const out = [];
  for (const st of statements) {
    if (ts.isExpressionStatement(st) && ts.isStringLiteral(st.expression)) out.push(st.expression.text);
    else break;
  }
  return out;
}

// returnValues is what a function returns: its expression body, or the
// expression of each return statement in it, not in a function inside it.
function returnValues(fn) {
  if (!fn.body) return [];
  if (!ts.isBlock(fn.body)) return [fn.body];
  const out = [];
  const walk = (n) => {
    if (n !== fn && isFunctionLike(n)) return;
    if (ts.isReturnStatement(n) && n.expression) out.push(n.expression);
    ts.forEachChild(n, walk);
  };
  ts.forEachChild(fn.body, walk);
  return out;
}

function accessDetails(node, fact) {
  let n = node;
  while (n.parent && (ts.isParenthesizedExpression(n.parent) || ts.isAsExpression(n.parent) || ts.isNonNullExpression(n.parent))) n = n.parent;
  const p = n.parent;
  if (p && ts.isBinaryExpression(p) && p.left === n) {
    const op = p.operatorToken.kind;
    if (op === ts.SyntaxKind.QuestionQuestionToken || op === ts.SyntaxKind.BarBarToken) {
      fact.default = value(p.right);
    } else if (op === ts.SyntaxKind.EqualsEqualsEqualsToken || op === ts.SyntaxKind.ExclamationEqualsEqualsToken || op === ts.SyntaxKind.EqualsEqualsToken || op === ts.SyntaxKind.ExclamationEqualsToken) {
      fact.compared = value(p.right);
    }
  }
  if (p && ts.isSwitchStatement(p) && p.expression === n) {
    fact.cases = p.caseBlock.clauses.filter((c) => ts.isCaseClause(c)).map((c) => value(c.expression));
    if (p.caseBlock.clauses.some((c) => ts.isDefaultClause(c))) fact.hasDefault = true;
  }
  // The call it is given to, through a default: Number(process.env.PORT ?? '8080').
  let w = n;
  let q = p;
  if (fact.default && q) {
    w = q;
    while (w.parent && ts.isParenthesizedExpression(w.parent)) w = w.parent;
    q = w.parent;
  }
  if (q && ts.isCallExpression(q) && q.arguments[0] === w) {
    fact.wrappedBy = text(q.expression);
  }
  return fact;
}

function isAccessTop(node) {
  const p = node.parent;
  if (!p) return true;
  if (ts.isPropertyAccessExpression(p) && p.expression === node) return false;
  if (ts.isElementAccessExpression(p) && p.expression === node) return false;
  return true;
}

function functionName(n) {
  if ((ts.isFunctionDeclaration(n) || ts.isFunctionExpression(n)) && n.name) return n.name.text;
  if (ts.isMethodDeclaration(n)) return propertyName(n.name);
  const p = n.parent;
  if (p && ts.isVariableDeclaration(p) && ts.isIdentifier(p.name)) return p.name.text;
  if (p && ts.isPropertyAssignment(p)) return propertyName(p.name);
  if (p && ts.isExportAssignment(p)) return 'default';
  return null;
}

function parameterOf(p) {
  const out = ts.isIdentifier(p.name) ? { name: p.name.text } : { pattern: text(p.name) };
  if (p.type) {
    out.type = typeOf(p.type);
    out.typeFrom = 'annotation';
  } else {
    const jt = ts.getJSDocType(p);
    if (jt) {
      out.type = typeOf(jt);
      out.typeFrom = 'jsdoc';
    }
  }
  if (p.questionToken || p.initializer) out.optional = true;
  return out;
}

function visit(node) {
  if (ts.isImportDeclaration(node) && ts.isStringLiteral(node.moduleSpecifier)) {
    const names = [];
    const c = node.importClause;
    if (c) {
      if (c.name) names.push({ imported: 'default', local: c.name.text });
      if (c.namedBindings && ts.isNamespaceImport(c.namedBindings)) names.push({ imported: '*', local: c.namedBindings.name.text });
      if (c.namedBindings && ts.isNamedImports(c.namedBindings)) {
        for (const s of c.namedBindings.elements) names.push({ imported: (s.propertyName || s.name).text, local: s.name.text });
      }
    }
    const f = { module: node.moduleSpecifier.text, names };
    if (c && c.isTypeOnly) f.typeOnly = true;
    const resolved = resolvedModule(node.moduleSpecifier.text);
    if (resolved) f.resolved = resolved;
    emit('import', node, f);
    return;
  }
  if (ts.isInterfaceDeclaration(node)) {
    const f = { declaration: 'interface', name: node.name.text, members: node.members.filter((m) => ts.isPropertySignature(m)).map(member) };
    const ex = isExported(node);
    if (ex) f.exported = ex;
    const ext = (node.heritageClauses || []).flatMap((h) => h.types.map((t) => t.expression.getText(sf)));
    if (ext.length > 0) f.extends = ext;
    emit('type', node, f);
  } else if (ts.isTypeAliasDeclaration(node)) {
    const f = { declaration: 'alias', name: node.name.text, type: typeOf(node.type) };
    const ex = isExported(node);
    if (ex) f.exported = ex;
    emit('type', node, f);
  } else if (ts.isClassDeclaration(node) && node.name) {
    const f = { declaration: 'class', name: node.name.text, members: [] };
    const ex = isExported(node);
    if (ex) f.exported = ex;
    const ext = (node.heritageClauses || []).flatMap((h) => h.types.map((t) => t.expression.getText(sf)));
    if (ext.length > 0) f.extends = ext;
    const decs = decorators(node);
    if (decs.length > 0) f.decorators = decs;
    for (const m of node.members) {
      if (!ts.isPropertyDeclaration(m)) continue;
      const mf = member(m);
      const md = decorators(m);
      if (md.length > 0) mf.decorators = md;
      if (!m.type) {
        const jt = ts.getJSDocType(m);
        if (jt) {
          mf.type = typeOf(jt);
          mf.typeFrom = 'jsdoc';
        }
      }
      const mods = ts.getModifiers(m) || [];
      if (mods.some((x) => x.kind === ts.SyntaxKind.StaticKeyword)) mf.static = true;
      f.members.push(mf);
    }
    emit('type', node, f);
  } else if (ts.isEnumDeclaration(node)) {
    const f = { declaration: 'enum', name: node.name.text, values: node.members.map((m) => ({ name: propertyName(m.name) ?? text(m.name), ...(m.initializer ? { value: value(m.initializer) } : {}) })) };
    const ex = isExported(node);
    if (ex) f.exported = ex;
    emit('type', node, f);
  }
  if (node.jsDoc) {
    for (const doc of node.jsDoc) {
      for (const tag of doc.tags || []) {
        if (ts.isJSDocTypedefTag(tag) && tag.name) {
          const f = { declaration: 'typedef', name: tag.name.getText(sf), comment: doc.getText(sf) };
          if (tag.typeExpression) f.type = typeOf(tag.typeExpression);
          emit('type', tag, f);
        }
      }
    }
  }
  if (isFunctionLike(node)) {
    const f = {};
    const name = functionName(node);
    if (name !== null) f.name = name;
    f.parameters = node.parameters.map(parameterOf);
    if (node.type) f.returns = typeOf(node.type);
    const holder = ts.isVariableDeclaration(node.parent) ? node.parent.parent.parent : node;
    const ex = ts.isExportAssignment(node.parent) ? 'default' : isExported(holder) || (ts.isVariableStatement(holder) ? isExported(holder) : null);
    if (ex) f.exported = ex;
    const doc = jsDocText(ts.isVariableDeclaration(node.parent) ? node.parent.parent.parent : node);
    if (doc && sf.fileName.match(/\.[cm]?jsx?$/)) f.jsdoc = doc;
    const dirs = directives(node.body && ts.isBlock(node.body) ? node.body.statements : []);
    if (dirs.length > 0) f.directives = dirs;
    // What generateStaticParams returns gives a route's parameters' values.
    if (name === 'generateStaticParams') f.returnValues = returnValues(node).map((r) => value(r));
    Object.assign(f, context(node));
    emit('function', node, f);
  }
  if (ts.isVariableDeclaration(node) && node.initializer) {
    const init = unwrap(node.initializer);
    const ctx = context(node);
    if (!ctx.within || ts.isCallExpression(init) || ts.isNewExpression(init)) {
      const f = {};
      if (ts.isIdentifier(node.name)) {
        f.name = node.name.text;
      } else if (ts.isObjectBindingPattern(node.name)) {
        f.names = node.name.elements.map((e) => ({ local: e.name.getText(sf), ...(e.propertyName ? { property: e.propertyName.getText(sf) } : {}), ...(e.initializer ? { default: value(e.initializer) } : {}), ...(e.dotDotDotToken ? { rest: true } : {}) }));
      } else {
        f.pattern = text(node.name);
      }
      f.declaration = node.parent.flags & ts.NodeFlags.Const ? 'const' : node.parent.flags & ts.NodeFlags.Let ? 'let' : 'var';
      f.value = value(node.initializer);
      if (node.type) f.type = typeOf(node.type);
      else {
        const jt = ts.getJSDocType(node);
        if (jt) {
          f.type = typeOf(jt);
          f.typeFrom = 'jsdoc';
        }
      }
      const stmt = node.parent.parent;
      if (ts.isVariableStatement(stmt)) {
        const ex = isExported(stmt);
        if (ex) f.exported = ex;
      }
      Object.assign(f, ctx);
      emit('variable', node, f);
    }
    // Reads of the environment or of a request by destructuring.
    if (ts.isObjectBindingPattern(node.name)) {
      const base = readAccess(init);
      if (base) {
        for (const e of node.name.elements) {
          if (e.dotDotDotToken) continue;
          const key = e.propertyName ? propertyName(e.propertyName) : e.name.getText(sf);
          const f = { chain: [...base.chain, key ?? '[computed]'] };
          if (base.parameter) f.parameter = base.parameter;
          if (e.initializer) f.default = value(e.initializer);
          Object.assign(f, context(node));
          emit('access', e, f);
        }
      }
    }
  }
  if (ts.isSourceFile(node)) {
    for (const d of directives(node.statements)) emit('directive', node.statements[0], { name: d });
  }
  // export { handler as GET }, from this module only.
  if (ts.isExportDeclaration(node) && !node.moduleSpecifier && node.exportClause && ts.isNamedExports(node.exportClause)) {
    for (const e of node.exportClause.elements) {
      const local = e.propertyName || e.name;
      if (ts.isIdentifier(local)) emit('export', e, { name: e.name.text, value: { name: local.text, ...resolveName(local) } });
    }
  }
  if (ts.isExportAssignment(node)) {
    emit('export', node, { name: 'default', value: value(node.expression) });
  }
  if (ts.isCallExpression(node)) {
    const callee = unwrap(node.expression);
    if (node.expression.kind === ts.SyntaxKind.ImportKeyword) {
      const a = node.arguments[0];
      if (a && ts.isStringLiteralLike(a)) {
        const f = { module: a.text, names: [], dynamic: true };
        const resolved = resolvedModule(a.text);
        if (resolved) f.resolved = resolved;
        emit('import', node, f);
      } else {
        emit('dynamic', node, { what: 'import', value: a ? value(a) : { text: '' }, ...context(node) });
      }
    } else if (ts.isIdentifier(callee) && callee.text === 'require' && !resolveName(callee).declaration) {
      const a = node.arguments[0];
      if (a && ts.isStringLiteralLike(a)) {
        const f = { module: a.text, names: [], require: true };
        const resolved = resolvedModule(a.text);
        if (resolved) f.resolved = resolved;
        emit('import', node, f);
      } else {
        emit('dynamic', node, { what: 'require', value: a ? value(a) : { text: '' }, ...context(node) });
      }
    } else if (wantedCall(node)) {
      const f = { ...callOf(node, 0) };
      const to = assignedTo(node);
      if (to !== null) f.assignedTo = to;
      Object.assign(f, context(node));
      emit('call', node, f);
    }
  }
  if (ts.isBinaryExpression(node) && node.operatorToken.kind === ts.SyntaxKind.EqualsToken) {
    const l = node.left;
    if (ts.isElementAccessExpression(l) && !ts.isStringLiteralLike(l.argumentExpression) && /^(module\.exports|exports)$/.test(l.expression.getText(sf))) {
      emit('dynamic', node, { what: 'export', value: value(l.argumentExpression), ...context(node) });
    }
  }
  if ((ts.isPropertyAccessExpression(node) || ts.isElementAccessExpression(node)) && isAccessTop(node)) {
    const f = readAccess(node);
    if (f) {
      accessDetails(node, f);
      Object.assign(f, context(node));
      emit('access', node, f);
    }
  }
  if (ts.isJsxElement(node) || ts.isJsxSelfClosingElement(node)) {
    const o = opening(node);
    let wanted = false;
    if (ts.isIdentifier(o.tagName) && /^[a-z]/.test(o.tagName.text)) {
      wanted = intrinsicElements.has(o.tagName.text);
    } else {
      const root = calleeRoot(o.tagName);
      const m = importedFrom(root);
      wanted = m !== null && knownModule(m);
    }
    if (wanted) {
      const f = element(node);
      for (let p = node.parent; p && !isFunctionLike(p); p = p.parent) {
        if ((ts.isJsxElement(p) || ts.isJsxSelfClosingElement(p)) && p !== node) {
          const po = opening(p);
          if (!(ts.isIdentifier(po.tagName) && /^[a-z]/.test(po.tagName.text))) {
            f.parent = where(p);
            f.parentTag = tagOf(po.tagName);
            break;
          }
        }
      }
      Object.assign(f, context(node));
      emit('jsx', node, f);
    }
  }
  ts.forEachChild(node, visit);
}

function resolvedModule(specifier) {
  const r = ts.resolveModuleName(specifier, sf.fileName, options, host);
  const m = r.resolvedModule;
  if (m && tracked.has(rel(m.resolvedFileName))) {
    return rel(m.resolvedFileName);
  }
  return null;
}

for (const file of [...sources, ...configs].sort()) {
  if (configNames.has(path.posix.basename(file))) {
    const f = configFacts.find((c) => c.file === file);
    fileList.push({ file, syntaxErrors: f && f.error ? 1 : 0 });
    if (f) facts.push(f);
    continue;
  }
  sf = program.getSourceFile(path.resolve(root, file));
  if (!sf) {
    fileList.push({ file, syntaxErrors: 1 });
    continue;
  }
  fileList.push({ file, syntaxErrors: program.getSyntacticDiagnostics(sf).length });
  visit(sf);
}

const indexed = facts.map((f, i) => [f, i]);
indexed.sort((a, b) => (a[0].file < b[0].file ? -1 : a[0].file > b[0].file ? 1 : a[0].line - b[0].line || a[0].column - b[0].column || a[1] - b[1]));

const lines = [];
lines.push('{');
lines.push('    "codeFacts": 1,');
lines.push('    "language": "javascript",');
lines.push(`    "parser": ${JSON.stringify({ name: parserName, version: parserVersion }).replace(/,/g, ', ').replace(/":/g, '": ')},`);
lines.push(`    "path": ${JSON.stringify(folder)},`);
lines.push(`    "commit": ${JSON.stringify(commit)},`);
lines.push('    "files": [');
lines.push(fileList.map((f) => `        {"file": ${JSON.stringify(f.file)}, "syntaxErrors": ${f.syntaxErrors}}`).join(',\n'));
lines.push('    ],');
lines.push('    "facts": [');
lines.push(indexed.map(([f]) => '        ' + JSON.stringify(f)).join(',\n'));
lines.push('    ]');
lines.push('}');
process.stdout.write(lines.join('\n') + '\n');
