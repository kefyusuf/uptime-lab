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
    ['CreateMonitorRequest', 'Monitor', 'Problem'],
    'components.schemas',
  );

  assert(isObject(document.paths), 'paths must be an object');
  const publicPaths = Object.keys(document.paths).filter((key) => key.startsWith('/'));
  assertExactSet(publicPaths, ['/monitors', '/monitors/{monitorId}'], 'public path set');

  const createPath = document.paths['/monitors'];
  const getPath = document.paths['/monitors/{monitorId}'];
  assert(isObject(createPath), '/monitors path item must be an object');
  assert(isObject(getPath), '/monitors/{monitorId} path item must be an object');
  assert(!own(createPath, 'servers'), '/monitors path item must not define servers');
  assert(!own(getPath, 'servers'), '/monitors/{monitorId} path item must not define servers');
  assertExactSet(operationMethods(createPath), ['post'], '/monitors operations');
  assertExactSet(operationMethods(getPath), ['get'], '/monitors/{monitorId} operations');

  const register = createPath.post;
  const getMonitor = getPath.get;
  assert(isObject(register), 'POST /monitors must exist');
  assert(isObject(getMonitor), 'GET /monitors/{monitorId} must exist');
  assert(register.operationId === 'registerMonitor', 'POST /monitors operationId must be registerMonitor');
  assert(getMonitor.operationId === 'getMonitor', 'GET /monitors/{monitorId} operationId must be getMonitor');
  assertNoOperationSecurityOrServers(register, 'POST /monitors');
  assertNoOperationSecurityOrServers(getMonitor, 'GET /monitors/{monitorId}');

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

  const root = path.resolve(repositoryRoot);
  const internalContract = path.join(root, 'contracts', 'openapi', 'internal.yaml');
  assert(!fs.existsSync(internalContract), 'contracts/openapi/internal.yaml must not exist');
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
