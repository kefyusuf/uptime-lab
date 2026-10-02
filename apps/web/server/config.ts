export interface WebConfig {
  browserPort: number;
  allowedOrigins: readonly string[];
  upstreamOrigin: string;
  maxHeaderBytes: number;
  maxRequestBytes: number;
  maxResponseBytes: number;
  headerTimeoutMs: number;
  requestTimeoutMs: number;
  upstreamTimeoutMs: number;
}
export function loadWebConfig(env: NodeJS.ProcessEnv): WebConfig {
  const raw = env.UPTIME_LAB_WEB_PORT ?? '4173';
  if (!/^[1-9][0-9]*$/.test(raw) || Number(raw) < 1024 || Number(raw) > 65535)
    throw new Error('Invalid local Web port.');
  const browserPort = Number(raw);
  return {
    browserPort,
    allowedOrigins: [
      'http://127.0.0.1:' + browserPort,
      'http://localhost:' + browserPort,
    ],
    upstreamOrigin: 'http://api:8080',
    maxHeaderBytes: 16384,
    maxRequestBytes: 65536,
    maxResponseBytes: 262144,
    headerTimeoutMs: 5000,
    requestTimeoutMs: 10000,
    upstreamTimeoutMs: 10000,
  };
}
