import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import assert from "node:assert/strict";
import { checkWebArchitecture } from "./check-web-architecture.mjs";

const cases = [
  [
    "entities/monitor/model.ts",
    "import '../../pages/Detail';",
    "reverse layer",
  ],
  [
    "shared/api/client.ts",
    "export * from '../../features/create/index';",
    "reverse export",
  ],
  [
    "features/create/form.ts",
    "import '../refresh/private';",
    "cross-feature private",
  ],
  [
    "pages/Detail.ts",
    "import('../server/gateway');",
    "unresolved dynamic import",
  ],
  ["pages/Detail.ts", "import '../../../server/gateway';", "server import"],
  ["shared/api/client.ts", "import 'node:fs';", "Node import"],
  ["shared/api/client.ts", "import 'fs';", "bare Node import"],
  ["entities/monitor/model.ts", "import '@/pages/Detail';", "reverse alias"],
  [
    "entities/monitor/model.ts",
    "const p = '../pages'; import(p);",
    "opaque dynamic import",
  ],
];
let count = 0;
for (const [file, content, label] of cases) {
  const root = mkdtempSync(join(tmpdir(), "uptime-web-boundary-"));
  try {
    const put = (path, text = "export {};") => {
      const full = join(root, "apps/web", path);
      mkdirSync(join(full, ".."), { recursive: true });
      writeFileSync(full, text);
    };
    put(
      "tsconfig.json",
      JSON.stringify({
        compilerOptions: {
          baseUrl: ".",
          paths: { "@/*": ["src/*"] },
          moduleResolution: "Bundler",
          module: "ESNext",
        },
      }),
    );
    for (const path of [
      "src/pages/Detail.ts",
      "src/features/create/index.ts",
      "src/features/refresh/private.ts",
      "server/gateway.ts",
    ])
      put(path);
    put("src/" + file, content);
    assert.ok(checkWebArchitecture(root).length > 0, label);
    count++;
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
}
const root = mkdtempSync(join(tmpdir(), "uptime-web-boundary-"));
try {
  for (const [path, content] of [
    ["apps/web/src/app/main.ts", "import './styles.css';"],
    ["apps/web/src/app/styles.css", "body { color: black; }"],
    ["apps/web/src/pages/Detail.ts", "import '../entities/monitor/index';"],
    ["apps/web/src/entities/monitor/index.ts", "export {};"],
  ]) {
    const full = join(root, path);
    mkdirSync(join(full, ".."), { recursive: true });
    writeFileSync(full, content);
  }
  assert.deepEqual(checkWebArchitecture(root), []);
  count++;
} finally {
  rmSync(root, { recursive: true, force: true });
}
console.log("Web architecture tests: " + count + " passed");
