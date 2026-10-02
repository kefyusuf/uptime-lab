#!/usr/bin/env node

import fs from 'node:fs';
import path from 'node:path';
import { isDeepStrictEqual } from 'node:util';
import { pathToFileURL } from 'node:url';

export class ContractInvariantError extends Error {}

function fail(message) {
  throw new ContractInvariantError(message);
}

function assert(condition, message) {
  if (!condition) fail(message);
}

function isObject(value) {
  return value !== null && typeof value === 'object' && !Array.isArray(value);
}

function own(object, key) {
  return isObject(object) && Object.prototype.hasOwnProperty.call(object, key);
}

function sorted(values) {
  return [...values].sort();
}

function assertExactKeys(object, expected, label) {
  assert(isObject(object), `${label} must be an object`);
  const actual = sorted(Object.keys(object));
  const wanted = sorted(expected);
  assert(
    isDeepStrictEqual(actual, wanted),
    `${label} keys must be exactly [${wanted.join(', ')}], got [${actual.join(', ')}]`,
  );
}

function assertExactSet(actualValues, expectedValues, label) {
  const actual = sorted(actualValues);
  const expected = sorted(expectedValues);
  assert(
    isDeepStrictEqual(actual, expected),
    `${label} must be exactly [${expected.join(', ')}], got [${actual.join(', ')}]`,
  );
}

function decodePointerToken(token) {
  return token.replaceAll('~1', '/').replaceAll('~0', '~');
}

function resolveLocalRef(document, value, label) {
  if (!isObject(value) || typeof value.$ref !== 'string') return value;

  const ref = value.$ref;
  assert(ref.startsWith('#/'), `${label} must use a local JSON Pointer reference`);

  let current = document;
  for (const rawToken of ref.slice(2).split('/')) {
    const token = decodePointerToken(rawToken);
    assert(isObject(current) && own(current, token), `${label} has unresolved reference ${ref}`);
    current = current[token];
  }
  return current;
}

function assertSchemaTarget(document, value, expectedName, label) {
  const expected = document.components.schemas[expectedName];
  const resolved = resolveLocalRef(document, value, label);
  assert(
    isDeepStrictEqual(resolved, expected),
    `${label} must resolve to components.schemas.${expectedName}`,
  );
}

function operationMethods(pathItem) {
  const methods = new Set(['get', 'put', 'post', 'delete', 'options', 'head', 'patch', 'trace']);
  return Object.keys(pathItem).filter((key) => methods.has(key));
}

function assertNoOperationSecurityOrServers(operation, label) {
  assert(!own(operation, 'security'), `${label} must not define security`);
  assert(!own(operation, 'servers'), `${label} must not define servers`);
}

function assertResponseContent(document, response, mediaType, schemaName, label) {
  assert(isObject(response), `${label} response must be an object`);
  assertExactKeys(response.content, [mediaType], `${label} content`);
  assertSchemaTarget(
    document,
    response.content[mediaType].schema,
    schemaName,
    `${label} schema`,
  );
}

function assertProblemResponses(document, operation, statuses, label) {
  for (const status of statuses) {
    const response = operation.responses[status];
    assert(response, `${label} must define response ${status}`);
    assertResponseContent(
      document,
      response,
      'application/problem+json',
      'Problem',
      `${label} ${status}`,
    );
  }
}

function assertStringFormat(schema, format, label) {
  assert(isObject(schema), `${label} must be an object`);
  assert(schema.type === 'string', `${label}.type must be string`);
  assert(schema.format === format, `${label}.format must be ${format}`);
}

function assertStringWithoutFormat(schema, label) {
  assert(isObject(schema), `${label} must be an object`);
  assert(schema.type === 'string', `${label}.type must be string`);
  assert(!own(schema, 'format'), `${label}.format must be absent`);
}

function assertObjectShape(schema, properties, required, label) {
  assert(isObject(schema), `${label} must be an object`);
  assert(schema.type === 'object', `${label}.type must be object`);
  assert(schema.additionalProperties === false, `${label}.additionalProperties must be false`);
  assertExactKeys(schema.properties, properties, `${label}.properties`);
  assert(Array.isArray(schema.required), `${label}.required must be an array`);
  assertExactSet(schema.required, required, `${label}.required`);
}

function assertIntegerRange(schema, minimum, maximum, label) {
  assert(isObject(schema), `${label} must be an object`);
  assert(schema.type === 'integer', `${label}.type must be integer`);
  assert(schema.minimum === minimum, `${label}.minimum must be ${minimum}`);
  assert(schema.maximum === maximum, `${label}.maximum must be ${maximum}`);
}

function assertStringConst(schema, value, label) {
  assert(isObject(schema), `${label} must be an object`);
  assert(schema.type === 'string', `${label}.type must be string`);
  assert(schema.const === value, `${label}.const must be ${value}`);
}

function assertStringEnum(schema, values, label) {
  assert(isObject(schema), `${label} must be an object`);
  assert(schema.type === 'string', `${label}.type must be string`);
  assert(Array.isArray(schema.enum), `${label}.enum must be an array`);
  assertExactSet(schema.enum, values, `${label}.enum`);
}

function assertOneOfRefs(schema, refs, label) {
  assert(isObject(schema), `${label} must be an object`);
  assertExactKeys(schema, ['oneOf'], label);
  assert(Array.isArray(schema.oneOf), `${label}.oneOf must be an array`);
  const actual = schema.oneOf.map((entry) => {
    assert(isObject(entry) && typeof entry.$ref === 'string', `${label}.oneOf entries must be refs`);
    return entry.$ref;
  });
  assertExactSet(actual, refs, `${label}.oneOf refs`);
}

function readJSONFixture(repositoryRoot, filename) {
  const fixturePath = path.join(repositoryRoot, 'contracts', 'fixtures', 'public', filename);
  assert(fs.existsSync(fixturePath), `public fixture ${filename} must exist`);
  try {
    return JSON.parse(fs.readFileSync(fixturePath, 'utf8'));
  } catch (error) {
    fail(`public fixture ${filename} must be valid JSON: ${error.message}`);
  }
}

function assertFixtureString(value, label) {
  assert(typeof value === 'string' && value.length > 0, `${label} must be a non-empty string`);
}

function validateLatestResultFixtures(repositoryRoot) {
  const fixtureDir = path.join(repositoryRoot, 'contracts', 'fixtures', 'public');
  assert(fs.existsSync(fixtureDir), 'contracts/fixtures/public must exist');
  assertExactSet(
    fs.readdirSync(fixtureDir).filter((name) => name.endsWith('.json')),
    [
      'latest-result-failure.json',
      'latest-result-http-response.json',
      'latest-result-worker-timeout.json',
      ...availabilityFixtureCases.map(([name]) => `availability-${name}.json`),
    ],
    'public fixture file set',
  );

  const httpResult = readJSONFixture(repositoryRoot, 'latest-result-http-response.json');
  assertExactKeys(
    httpResult,
    ['checkId', 'resultKind', 'httpStatus', 'durationMs', 'completedAt'],
    'latest-result-http-response fixture',
  );
  assertFixtureString(httpResult.checkId, 'latest-result-http-response.checkId');
  assert(httpResult.resultKind === 'http_response', 'latest-result-http-response.resultKind must be http_response');
  assert(Number.isInteger(httpResult.httpStatus) && httpResult.httpStatus >= 100 && httpResult.httpStatus <= 599,
    'latest-result-http-response.httpStatus must be 100..599');
  assert(Number.isInteger(httpResult.durationMs) && httpResult.durationMs >= 0 && httpResult.durationMs <= 20000,
    'latest-result-http-response.durationMs must be 0..20000');
  assertFixtureString(httpResult.completedAt, 'latest-result-http-response.completedAt');

  const failure = readJSONFixture(repositoryRoot, 'latest-result-failure.json');
  assertExactKeys(
    failure,
    ['checkId', 'resultKind', 'durationMs', 'completedAt'],
    'latest-result-failure fixture',
  );
  assertFixtureString(failure.checkId, 'latest-result-failure.checkId');
  assert(
    ['dns_error', 'policy_rejected', 'timeout', 'connect_error', 'tls_error', 'protocol_error', 'internal_error']
      .includes(failure.resultKind),
    'latest-result-failure.resultKind must be a classified failure',
  );
  assert(Number.isInteger(failure.durationMs) && failure.durationMs >= 0 && failure.durationMs <= 20000,
    'latest-result-failure.durationMs must be 0..20000');
  assertFixtureString(failure.completedAt, 'latest-result-failure.completedAt');

  const workerTimeout = readJSONFixture(repositoryRoot, 'latest-result-worker-timeout.json');
  assertExactKeys(
    workerTimeout,
    ['checkId', 'resultKind', 'completedAt'],
    'latest-result-worker-timeout fixture',
  );
  assertFixtureString(workerTimeout.checkId, 'latest-result-worker-timeout.checkId');
  assert(workerTimeout.resultKind === 'worker_timeout',
    'latest-result-worker-timeout.resultKind must be worker_timeout');
  assertFixtureString(workerTimeout.completedAt, 'latest-result-worker-timeout.completedAt');
}

export function validatePublicContract(document, repositoryRoot = '.') {
  assert(isObject(document), 'document must be a JSON object');

  assert(document.openapi === '3.1.2', 'openapi must be exactly 3.1.2');
  assert(isObject(document.info), 'info must be an object');
  assert(document.info.version === '0.1.0', 'info.version must be exactly 0.1.0');
  assert(!own(document, 'servers'), 'root servers must be absent');
  assert(!own(document, 'security'), 'root security must be absent');

  assert(isObject(document.components), 'components must be an object');
  assert(!own(document.components, 'securitySchemes'), 'components.securitySchemes must be absent');
  assert(isObject(document.components.schemas), 'components.schemas must be an object');
  assertExactKeys(
    document.components.schemas,
    [
      'CreateMonitorRequest',
      'LatestCheckResult',
      'LatestFailureResult',
      'LatestHTTPResponseResult',
      'LatestWorkerTimeoutResult',
      'Monitor',
      'Problem',
      'MonitorAvailability', 'AvailableMonitorAvailability', 'UnavailableMonitorAvailability',
      'UnknownMonitorAvailability', 'NoResultMonitorAvailability', 'MonitorAvailabilityEvidence',
    ],
    'components.schemas',
  );

  assert(isObject(document.paths), 'paths must be an object');
  const publicPaths = Object.keys(document.paths).filter((key) => key.startsWith('/'));
  assertExactSet(
    publicPaths,
    ['/monitors', '/monitors/{monitorId}', '/monitors/{monitorId}/latest-result', '/monitors/{monitorId}/availability'],
    'public path set',
  );

  const createPath = document.paths['/monitors'];
  const getPath = document.paths['/monitors/{monitorId}'];
  const latestResultPath = document.paths['/monitors/{monitorId}/latest-result'];
  assert(isObject(createPath), '/monitors path item must be an object');
  assert(isObject(getPath), '/monitors/{monitorId} path item must be an object');
  assert(isObject(latestResultPath), '/monitors/{monitorId}/latest-result path item must be an object');
  assert(!own(createPath, 'servers'), '/monitors path item must not define servers');
  assert(!own(getPath, 'servers'), '/monitors/{monitorId} path item must not define servers');
  assert(!own(latestResultPath, 'servers'), '/monitors/{monitorId}/latest-result path item must not define servers');
  assertExactSet(operationMethods(createPath), ['post'], '/monitors operations');
  assertExactSet(operationMethods(getPath), ['get'], '/monitors/{monitorId} operations');
  assertExactSet(operationMethods(latestResultPath), ['get'], '/monitors/{monitorId}/latest-result operations');

  const register = createPath.post;
  const getMonitor = getPath.get;
  const getLatestResult = latestResultPath.get;
  assert(isObject(register), 'POST /monitors must exist');
  assert(isObject(getMonitor), 'GET /monitors/{monitorId} must exist');
  assert(isObject(getLatestResult), 'GET /monitors/{monitorId}/latest-result must exist');
  assert(register.operationId === 'registerMonitor', 'POST /monitors operationId must be registerMonitor');
  assert(getMonitor.operationId === 'getMonitor', 'GET /monitors/{monitorId} operationId must be getMonitor');
  assert(
    getLatestResult.operationId === 'getLatestCheckResult',
    'GET /monitors/{monitorId}/latest-result operationId must be getLatestCheckResult',
  );
  assertNoOperationSecurityOrServers(register, 'POST /monitors');
  assertNoOperationSecurityOrServers(getMonitor, 'GET /monitors/{monitorId}');
  assertNoOperationSecurityOrServers(getLatestResult, 'GET /monitors/{monitorId}/latest-result');

  assert(isObject(register.requestBody), 'POST /monitors requestBody must exist');
  assert(register.requestBody.required === true, 'POST /monitors requestBody.required must be true');
  assertExactKeys(register.requestBody.content, ['application/json'], 'POST /monitors requestBody.content');
  assertSchemaTarget(
    document,
    register.requestBody.content['application/json'].schema,
    'CreateMonitorRequest',
    'POST /monitors request schema',
  );

  assertExactKeys(
    register.responses,
    ['201', '400', '415', '422', '500'],
    'POST /monitors responses',
  );
  assertResponseContent(
    document,
    register.responses['201'],
    'application/json',
    'Monitor',
    'POST /monitors 201',
  );
  assert(isObject(register.responses['201'].headers), 'POST /monitors 201 headers must be an object');
  assert(own(register.responses['201'].headers, 'Location'), 'POST /monitors 201 must define Location header');
  const locationSchema = register.responses['201'].headers.Location?.schema;
  assertStringFormat(locationSchema, 'uri-reference', 'POST /monitors 201 Location schema');
  assertProblemResponses(document, register, ['400', '415', '422', '500'], 'POST /monitors');

  assert(Array.isArray(getMonitor.parameters), 'GET /monitors/{monitorId} parameters must be an array');
  assert(getMonitor.parameters.length === 1, 'GET /monitors/{monitorId} must define exactly one parameter');
  const monitorId = getMonitor.parameters[0];
  assert(isObject(monitorId), 'monitorId parameter must be an object');
  assert(monitorId.name === 'monitorId', 'path parameter name must be monitorId');
  assert(monitorId.in === 'path', 'monitorId parameter must be in path');
  assert(monitorId.required === true, 'monitorId path parameter must be required');
  assertStringFormat(monitorId.schema, 'uuid', 'monitorId parameter schema');

  assertExactKeys(
    getMonitor.responses,
    ['200', '400', '404', '500'],
    'GET /monitors/{monitorId} responses',
  );
  assertResponseContent(
    document,
    getMonitor.responses['200'],
    'application/json',
    'Monitor',
    'GET /monitors/{monitorId} 200',
  );
  assertProblemResponses(document, getMonitor, ['400', '404', '500'], 'GET /monitors/{monitorId}');

  assert(
    Array.isArray(getLatestResult.parameters),
    'GET /monitors/{monitorId}/latest-result parameters must be an array',
  );
  assert(
    getLatestResult.parameters.length === 1,
    'GET /monitors/{monitorId}/latest-result must define exactly one parameter',
  );
  const latestMonitorId = getLatestResult.parameters[0];
  assert(isObject(latestMonitorId), 'latest-result monitorId parameter must be an object');
  assert(latestMonitorId.name === 'monitorId', 'latest-result path parameter name must be monitorId');
  assert(latestMonitorId.in === 'path', 'latest-result monitorId parameter must be in path');
  assert(latestMonitorId.required === true, 'latest-result monitorId path parameter must be required');
  assertStringFormat(latestMonitorId.schema, 'uuid', 'latest-result monitorId parameter schema');

  assertExactKeys(
    getLatestResult.responses,
    ['200', '204', '400', '404', '500'],
    'GET /monitors/{monitorId}/latest-result responses',
  );
  assertResponseContent(
    document,
    getLatestResult.responses['200'],
    'application/json',
    'LatestCheckResult',
    'GET /monitors/{monitorId}/latest-result 200',
  );
  const noResult = getLatestResult.responses['204'];
  assert(isObject(noResult), 'GET /monitors/{monitorId}/latest-result 204 response must be an object');
  assert(!own(noResult, 'content'), 'GET /monitors/{monitorId}/latest-result 204 content must be absent');
  assertProblemResponses(
    document,
    getLatestResult,
    ['400', '404', '500'],
    'GET /monitors/{monitorId}/latest-result',
  );

  const createSchema = document.components.schemas.CreateMonitorRequest;
  assertObjectShape(createSchema, ['targetUrl'], ['targetUrl'], 'CreateMonitorRequest');
  assertStringWithoutFormat(createSchema.properties.targetUrl, 'CreateMonitorRequest.targetUrl');

  const monitorSchema = document.components.schemas.Monitor;
  assertObjectShape(
    monitorSchema,
    ['id', 'targetUrl', 'createdAt'],
    ['id', 'targetUrl', 'createdAt'],
    'Monitor',
  );
  assertStringFormat(monitorSchema.properties.id, 'uuid', 'Monitor.id');
  assertStringWithoutFormat(monitorSchema.properties.targetUrl, 'Monitor.targetUrl');
  assertStringFormat(monitorSchema.properties.createdAt, 'date-time', 'Monitor.createdAt');

  const idDescription = String(monitorSchema.properties.id.description ?? '');
  assert(
    !/(?:uuid\s*)?v7\b|version\s*7\b/i.test(idDescription),
    'Monitor.id must not promise UUID v7',
  );

  const latestResultSchema = document.components.schemas.LatestCheckResult;
  assertOneOfRefs(
    latestResultSchema,
    [
      '#/components/schemas/LatestHTTPResponseResult',
      '#/components/schemas/LatestFailureResult',
      '#/components/schemas/LatestWorkerTimeoutResult',
    ],
    'LatestCheckResult',
  );

  const latestHTTP = document.components.schemas.LatestHTTPResponseResult;
  assertObjectShape(
    latestHTTP,
    ['checkId', 'resultKind', 'httpStatus', 'durationMs', 'completedAt'],
    ['checkId', 'resultKind', 'httpStatus', 'durationMs', 'completedAt'],
    'LatestHTTPResponseResult',
  );
  assertStringFormat(latestHTTP.properties.checkId, 'uuid', 'LatestHTTPResponseResult.checkId');
  assertStringConst(latestHTTP.properties.resultKind, 'http_response', 'LatestHTTPResponseResult.resultKind');
  assertIntegerRange(latestHTTP.properties.httpStatus, 100, 599, 'LatestHTTPResponseResult.httpStatus');
  assertIntegerRange(latestHTTP.properties.durationMs, 0, 20000, 'LatestHTTPResponseResult.durationMs');
  assertStringFormat(latestHTTP.properties.completedAt, 'date-time', 'LatestHTTPResponseResult.completedAt');

  const latestFailure = document.components.schemas.LatestFailureResult;
  assertObjectShape(
    latestFailure,
    ['checkId', 'resultKind', 'durationMs', 'completedAt'],
    ['checkId', 'resultKind', 'durationMs', 'completedAt'],
    'LatestFailureResult',
  );
  assertStringFormat(latestFailure.properties.checkId, 'uuid', 'LatestFailureResult.checkId');
  assertStringEnum(
    latestFailure.properties.resultKind,
    ['dns_error', 'policy_rejected', 'timeout', 'connect_error', 'tls_error', 'protocol_error', 'internal_error'],
    'LatestFailureResult.resultKind',
  );
  assertIntegerRange(latestFailure.properties.durationMs, 0, 20000, 'LatestFailureResult.durationMs');
  assertStringFormat(latestFailure.properties.completedAt, 'date-time', 'LatestFailureResult.completedAt');

  const latestWorkerTimeout = document.components.schemas.LatestWorkerTimeoutResult;
  assertObjectShape(
    latestWorkerTimeout,
    ['checkId', 'resultKind', 'completedAt'],
    ['checkId', 'resultKind', 'completedAt'],
    'LatestWorkerTimeoutResult',
  );
  assertStringFormat(latestWorkerTimeout.properties.checkId, 'uuid', 'LatestWorkerTimeoutResult.checkId');
  assertStringConst(
    latestWorkerTimeout.properties.resultKind,
    'worker_timeout',
    'LatestWorkerTimeoutResult.resultKind',
  );
  assertStringFormat(
    latestWorkerTimeout.properties.completedAt,
    'date-time',
    'LatestWorkerTimeoutResult.completedAt',
  );

  const problemSchema = document.components.schemas.Problem;
  assert(isObject(problemSchema), 'Problem must be an object');
  assert(problemSchema.type === 'object', 'Problem.type must be object');
  assert(problemSchema.additionalProperties === false, 'Problem.additionalProperties must be false');
  assertExactKeys(
    problemSchema.properties,
    ['type', 'title', 'status', 'detail', 'instance'],
    'Problem.properties',
  );
  assertStringFormat(problemSchema.properties.type, 'uri-reference', 'Problem.type');
  assert(problemSchema.properties.title?.type === 'string', 'Problem.title.type must be string');
  assert(problemSchema.properties.status?.type === 'integer', 'Problem.status.type must be integer');
  assert(problemSchema.properties.status?.minimum === 100, 'Problem.status.minimum must be 100');
  assert(problemSchema.properties.status?.maximum === 599, 'Problem.status.maximum must be 599');
  assert(problemSchema.properties.detail?.type === 'string', 'Problem.detail.type must be string');
  assertStringFormat(problemSchema.properties.instance, 'uri-reference', 'Problem.instance');

  validateLatestResultFixtures(repositoryRoot);
  validateAvailability(document, repositoryRoot);
}

const availabilityFixtureCases = [
  ['available', 'available', 'successful_response'], ['unavailable-http', 'unavailable', 'unexpected_http_status'],
  ['unavailable-probe', 'unavailable', 'probe_failure'], ['unknown-no-result', 'unknown', 'no_result'],
  ['unknown-stale', 'unknown', 'stale_result'], ['unknown-future', 'unknown', 'future_result'],
  ['unknown-policy', 'unknown', 'policy_rejected'], ['unknown-execution', 'unknown', 'execution_failure'],
];

function assertUTCTimestamp(value, label) {
  assert(typeof value === 'string' && /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z$/.test(value)
    && Number.isFinite(Date.parse(value)), `${label} must be a UTC timestamp`);
}

function validateAvailability(document, repositoryRoot) {
  const pathItem = document.paths['/monitors/{monitorId}/availability'];
  assert(isObject(pathItem), 'availability path item must be an object');
  assert(!own(pathItem, 'servers'), 'availability path item must not define servers');
  assertExactSet(operationMethods(pathItem), ['get'], 'availability operations');
  const operation = pathItem.get;
  assert(operation.operationId === 'getMonitorAvailability', 'operationId must be getMonitorAvailability');
  assertNoOperationSecurityOrServers(operation, 'availability');
  assert(Array.isArray(operation.parameters) && operation.parameters.length === 1, 'availability requires one parameter');
  const parameter = operation.parameters[0];
  assert(parameter.name === 'monitorId' && parameter.in === 'path' && parameter.required === true, 'availability monitorId must be a required path parameter');
  assertStringFormat(parameter.schema, 'uuid', 'availability monitorId');
  assertExactKeys(operation.responses, ['200', '400', '404', '500'], 'availability responses');
  assertResponseContent(document, operation.responses['200'], 'application/json', 'MonitorAvailability', 'availability 200');
  assertProblemResponses(document, operation, ['400', '404', '500'], 'availability');
  for (const [code, response] of Object.entries(operation.responses)) {
    assert(isObject(response.headers?.['Cache-Control']), `availability ${code} must define Cache-Control`);
    assertStringConst(response.headers['Cache-Control'].schema, 'no-store', `availability ${code} Cache-Control`);
  }
  const schemas = document.components.schemas;
  const variants = [
    ['AvailableMonitorAvailability', 'available', ['successful_response']],
    ['UnavailableMonitorAvailability', 'unavailable', ['unexpected_http_status', 'probe_failure']],
    ['UnknownMonitorAvailability', 'unknown', ['future_result', 'stale_result', 'policy_rejected', 'execution_failure']],
    ['NoResultMonitorAvailability', 'unknown', ['no_result']],
  ];
  assertOneOfRefs(schemas.MonitorAvailability, variants.map(([name]) => `#/components/schemas/${name}`), 'MonitorAvailability');
  for (const [name, status, reasons] of variants) {
    const schema = schemas[name];
    const keys = ['status', 'reason', 'evaluatedAt', ...(name === 'NoResultMonitorAvailability' ? [] : ['evidence'])];
    assertObjectShape(schema, keys, keys, name);
    assertStringConst(schema.properties.status, status, `${name}.status`);
    if (reasons.length === 1) assertStringConst(schema.properties.reason, reasons[0], `${name}.reason`);
    else assertStringEnum(schema.properties.reason, reasons, `${name}.reason`);
    assertStringFormat(schema.properties.evaluatedAt, 'date-time', `${name}.evaluatedAt`);
    if (name !== 'NoResultMonitorAvailability') assertSchemaTarget(document, schema.properties.evidence, 'MonitorAvailabilityEvidence', `${name}.evidence`);
  }
  const evidence = schemas.MonitorAvailabilityEvidence;
  assertObjectShape(evidence, ['checkId', 'completedAt'], ['checkId', 'completedAt'], 'MonitorAvailabilityEvidence');
  assertStringFormat(evidence.properties.checkId, 'uuid', 'MonitorAvailabilityEvidence.checkId');
  assertStringFormat(evidence.properties.completedAt, 'date-time', 'MonitorAvailabilityEvidence.completedAt');
  for (const [name, status, reason] of availabilityFixtureCases) {
    const payload = readJSONFixture(repositoryRoot, `availability-${name}.json`);
    assertExactKeys(payload, ['status', 'reason', 'evaluatedAt', ...(reason === 'no_result' ? [] : ['evidence'])], `availability-${name} fixture`);
    assert(payload.status === status && payload.reason === reason, `availability-${name} status/reason must match its variant`);
    assertUTCTimestamp(payload.evaluatedAt, `availability-${name}.evaluatedAt`);
    if (reason !== 'no_result') {
      assertExactKeys(payload.evidence, ['checkId', 'completedAt'], `availability-${name}.evidence`);
      assert(typeof payload.evidence.checkId === 'string' && /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(payload.evidence.checkId), `availability-${name}.checkId must be a UUID`);
      assertUTCTimestamp(payload.evidence.completedAt, `availability-${name}.completedAt`);
    }
  }
}

function main() {
  const [bundlePath, repositoryRoot = '.'] = process.argv.slice(2);
  if (!bundlePath) {
    console.error('Usage: check-public-contract.mjs <bundled-json> [repository-root]');
    process.exit(2);
  }

  try {
    const raw = fs.readFileSync(bundlePath, 'utf8');
    const document = JSON.parse(raw);
    validatePublicContract(document, repositoryRoot);
    console.log('Public contract semantic check: PASS');
  } catch (error) {
    if (error instanceof ContractInvariantError) {
      console.error(`Public contract invariant failed: ${error.message}`);
      process.exit(1);
    }
    console.error(`Public contract check failed: ${error.message}`);
    process.exit(1);
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href) {
  main();
}
