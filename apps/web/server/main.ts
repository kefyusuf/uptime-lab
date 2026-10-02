import { fileURLToPath } from 'node:url';
import { loadAssets } from './assets.js';
import { loadWebConfig } from './config.js';
import { createWebServer, shutdownWebServer } from './server.js';
try {
  const server = createWebServer(
    loadWebConfig(process.env),
    loadAssets(fileURLToPath(new URL('../dist', import.meta.url))),
  );
  server.listen(8080, '0.0.0.0');
  server.on('error', () => {
    console.error('Web failed to listen.');
    process.exitCode = 1;
  });
  let stopping = false;
  const stop = () => {
    if (stopping) return;
    stopping = true;
    void shutdownWebServer(server).catch(() => {
      process.exitCode = 1;
    });
  };
  process.once('SIGTERM', stop);
  process.once('SIGINT', stop);
} catch {
  console.error('Web failed to start.');
  process.exitCode = 1;
}
