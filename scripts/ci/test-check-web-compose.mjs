import { test } from "node:test";
import assert from "node:assert/strict";
import { checkWebCompose } from "./check-web-compose.mjs";
function configs(port = 4173) {
  const canonical = {
    services: Object.fromEntries(
      ["api", "checker", "db", "web"].map((name) => [name, {}]),
    ),
  };
  canonical.services.web = {
    build: { dockerfile: "apps/web/Dockerfile" },
    init: true,
    read_only: true,
    environment: { UPTIME_LAB_WEB_PORT: String(port) },
  };
  const local = structuredClone(canonical);
  local.services.web.ports = [
    {
      host_ip: "127.0.0.1",
      published: String(port),
      target: 8080,
      protocol: "tcp",
    },
  ];
  return { canonical, local };
}
for (const port of [4173, 4817])
  test(`valid resolved pair ${port}`, () => {
    const { canonical, local } = configs(port);
    assert.deepEqual(checkWebCompose(canonical, local, port), []);
  });
const mutations = {
  canonicalPort: (c) => {
    c.canonical.services.web.ports = [{ target: 8080 }];
  },
  wildcard: (c) => {
    c.local.services.web.ports[0].host_ip = "0.0.0.0";
  },
  apiPort: (c) => {
    c.local.services.api.ports = [{ target: 8080 }];
  },
  dbPort: (c) => {
    c.local.services.db.ports = [{ target: 5432 }];
  },
  checkerPort: (c) => {
    c.local.services.checker.ports = [{ target: 8080 }];
  },
  extraService: (c) => {
    c.local.services.extra = {};
  },
  wrongPort: (c) => {
    c.local.services.web.ports[0].published = "9999";
  },
  wrongOriginPort: (c) => {
    c.local.services.web.environment.UPTIME_LAB_WEB_PORT = "9999";
  },
  wrongBuild: (c) => {
    c.local.services.web.build.dockerfile = "placeholder";
  },
  writable: (c) => {
    c.local.services.web.read_only = false;
  },
};
for (const [name, mutate] of Object.entries(mutations))
  test(`rejects ${name}`, () => {
    const config = configs();
    mutate(config);
    assert.ok(checkWebCompose(config.canonical, config.local, 4173).length);
  });
for (const port of [0, 1023, 65536, NaN])
  test(`rejects invalid port ${port}`, () => {
    const { canonical, local } = configs();
    assert.ok(checkWebCompose(canonical, local, port).length);
  });
