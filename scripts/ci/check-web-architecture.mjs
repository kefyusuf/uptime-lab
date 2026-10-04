import { existsSync, readdirSync, readFileSync } from "node:fs";
import { builtinModules, createRequire } from "node:module";
import { dirname, isAbsolute, relative, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";
const require = createRequire(
  new URL("../../apps/web/package.json", import.meta.url),
);
const ts = require("typescript");
const layers = ["shared", "entities", "features", "widgets", "pages", "app"];
const builtins = new Set(
  builtinModules.map((name) => name.replace(/^node:/, "")),
);
export function checkWebArchitecture(root) {
  const web = resolve(root, "apps/web"),
    src = resolve(web, "src"),
    issues = [];
  let options = {
    moduleResolution: ts.ModuleResolutionKind.Bundler,
    module: ts.ModuleKind.ESNext,
  };
  const config = resolve(web, "tsconfig.json");
  if (existsSync(config)) {
    const parsed = ts.readConfigFile(config, ts.sys.readFile);
    if (parsed.error) return ["Invalid Web TypeScript configuration."];
    options = ts.parseJsonConfigFileContent(parsed.config, ts.sys, web).options;
  }
  function walk(directory) {
    if (!existsSync(directory)) return;
    for (const entry of readdirSync(directory, { withFileTypes: true })) {
      const path = resolve(directory, entry.name);
      if (entry.isDirectory()) walk(path);
      else if (
        /\.[cm]?[jt]sx?$/.test(path) &&
        !/\.test\.[jt]sx?$/.test(path) &&
        !path.endsWith(".generated.ts")
      )
        checkFile(path);
    }
  }
  function checkFile(path) {
    const source = ts.createSourceFile(
      path,
      readFileSync(path, "utf8"),
      ts.ScriptTarget.Latest,
      true,
    );
    const owner = relative(src, path).split(sep);
    const report = (message) =>
      issues.push(relative(web, path) + ": " + message);
    if (!layers.includes(owner[0])) return;
    function checkImport(specifier) {
      if (specifier.startsWith("node:") || builtins.has(specifier)) {
        report("Browser code cannot import Node modules.");
        return;
      }
      let resolved = ts.resolveModuleName(
        specifier,
        path,
        options,
        ts.sys,
      ).resolvedModule;
      if (
        !resolved &&
        specifier.startsWith(".") &&
        specifier.endsWith(".css")
      ) {
        const asset = resolve(dirname(path), specifier);
        if (existsSync(asset))
          resolved = {
            resolvedFileName: asset,
            isExternalLibraryImport: false,
          };
      }
      if (!resolved) {
        if (specifier.startsWith(".") || specifier.startsWith("@/"))
          report("Unresolved local import " + specifier);
        return;
      }
      const target = resolve(resolved.resolvedFileName);
      const distance = relative(src, target);
      if (distance.startsWith(".." + sep) || isAbsolute(distance)) {
        if (!resolved.isExternalLibraryImport)
          report("Browser import escapes src: " + specifier);
        return;
      }
      const other = distance.split(sep);
      if (layers.indexOf(other[0]) > layers.indexOf(owner[0]))
        report("Reversed frontend layer import " + specifier);
      if (
        ["features", "entities"].includes(other[0]) &&
        (owner[0] !== other[0] || owner[1] !== other[1])
      ) {
        if (other.length !== 3 || !/^index\.[cm]?[jt]sx?$/.test(other[2]))
          report("Private module import " + specifier);
      }
    }
    function visit(node) {
      if (
        (ts.isImportDeclaration(node) || ts.isExportDeclaration(node)) &&
        node.moduleSpecifier &&
        ts.isStringLiteral(node.moduleSpecifier)
      )
        checkImport(node.moduleSpecifier.text);
      if (
        ts.isCallExpression(node) &&
        (node.expression.kind === ts.SyntaxKind.ImportKeyword ||
          (ts.isIdentifier(node.expression) &&
            node.expression.text === "require"))
      ) {
        const argument = node.arguments[0];
        if (argument && ts.isStringLiteralLike(argument))
          checkImport(argument.text);
        else report("Opaque dynamic import cannot be boundary-checked.");
      }
      ts.forEachChild(node, visit);
    }
    visit(source);
  }
  walk(src);
  return issues;
}
if (
  process.argv[1] &&
  resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  const issues = checkWebArchitecture(resolve(process.argv[2] || "."));
  if (issues.length) {
    console.error(issues.join("\n"));
    process.exitCode = 1;
  } else console.log("Web architecture boundaries passed.");
}
