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

function assertNoSecurityOrServers(operation, label) {
  assert(!own(operation, 'security'), `${label} security must be absent`);
  assert(!own(operation, 'servers'), `${label} servers must be absent`);
}

function assertNoContent(response, label) {
  assert(isObject(response), `${label} response must be an object`);
  assert(!own(response, 'content'), `${label} content must be absent`);
}

function assertResponseContent(document, response, mediaType, schemaName, label) {
  assert(isObject(response), `${label} response must be an object`);
  assertExactKeys(response.content, [mediaType], `${label} content`);
  assertSchemaTarget(document, response.content[mediaType].schema, schemaName, `${label} schema`);
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

function assertInteger(schema, label) {
  assert(isObject(schema), `${label} must be an object`);
  assert(schema.type === 'integer', `${label}.type must be integer`);
}

function assertObjectShape(schema, properties, required, label) {
  assert(isObject(schema), `${label} must be an object`);
  assert(schema.type === 'object', `${label}.type must be object`);
  assert(schema.additionalProperties === false, `${label}.additionalProperties must be false`);
  assertExactKeys(schema.properties, properties, `${label}.properties`);
  assert(Array.isArray(schema.required), `${label}.required must be an array`);
  assertExactSet(schema.required, required, `${label}.required`);
}

export function validateInternalContract(document) {
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
    ['CheckWork', 'CheckResult', 'HttpResponseResult', 'FailureResult', 'Problem'],
    'components.schemas',
  );

  assert(isObject(document.paths), 'paths must be an object');
  const internalPaths = Object.keys(document.paths).filter((key) => key.startsWith('/'));
  assertExactSet(
    internalPaths,
    ['/internal/checks/claim', '/internal/checks/{checkId}/result'],
    'internal path set',
  );

  const claimPath = document.paths['/internal/checks/claim'];
  const resultPath = document.paths['/internal/checks/{checkId}/result'];
  assert(isObject(claimPath), '/internal/checks/claim path item must be an object');
  assert(isObject(resultPath), '/internal/checks/{checkId}/result path item must be an object');
  assertExactSet(operationMethods(claimPath), ['post'], '/internal/checks/claim operations');
  assertExactSet(operationMethods(resultPath), ['put'], '/internal/checks/{checkId}/result operations');

  const claim = claimPath.post;
  const result = resultPath.put;
  assert(isObject(claim), 'POST /internal/checks/claim must exist');
  assert(isObject(result), 'PUT /internal/checks/{checkId}/result must exist');
  assertNoSecurityOrServers(claim, 'claim operation');
  assertNoSecurityOrServers(result, 'result operation');

  assert(!own(claim, 'requestBody'), 'claim requestBody must be absent');
  assertExactKeys(claim.responses, ['200', '204', '405', '500'], 'claim responses');
  assertResponseContent(document, claim.responses['200'], 'application/json', 'CheckWork', 'claim 200');
  assertNoContent(claim.responses['204'], 'claim 204');
  assertProblemResponses(document, claim, ['405', '500'], 'claim');

  assert(Array.isArray(result.parameters), 'result parameters must be an array');
  assert(result.parameters.length === 1, 'result operation must define exactly one parameter');
  const checkId = result.parameters[0];
  assert(isObject(checkId), 'result checkId parameter must be an object');
  assert(checkId.name === 'checkId', 'result path parameter name must be checkId');
  assert(checkId.in === 'path', 'result checkId parameter must be in path');
  assert(checkId.required === true, 'result checkId path parameter must be required');
  assertStringFormat(checkId.schema, 'uuid', 'result checkId');

  assert(isObject(result.requestBody), 'result requestBody must exist');
  assert(result.requestBody.required === true, 'result requestBody.required must be true');
  assertExactKeys(result.requestBody.content, ['application/json'], 'result requestBody.content');
  assertSchemaTarget(
    document,
    result.requestBody.content['application/json'].schema,
    'CheckResult',
    'result request schema',
  );

  assertExactKeys(
    result.responses,
    ['204', '400', '404', '409', '415', '422', '405', '500'],
    'result responses',
  );
  assertNoContent(result.responses['204'], 'result 204');
  assertProblemResponses(document, result, ['400', '404', '409', '415', '422', '405', '500'], 'result');

  const work = document.components.schemas.CheckWork;
  assertObjectShape(
    work,
    ['checkId', 'monitorId', 'targetUrl', 'timeoutMs', 'maxRedirects'],
    ['checkId', 'monitorId', 'targetUrl', 'timeoutMs', 'maxRedirects'],
    'CheckWork',
  );
  assertStringFormat(work.properties.checkId, 'uuid', 'CheckWork.checkId');
  assertStringFormat(work.properties.monitorId, 'uuid', 'CheckWork.monitorId');
  assertStringWithoutFormat(work.properties.targetUrl, 'CheckWork.targetUrl');
  assertInteger(work.properties.timeoutMs, 'CheckWork.timeoutMs');
  assert(work.properties.timeoutMs.const === 10000, 'CheckWork.timeoutMs.const must be 10000');
  assertInteger(work.properties.maxRedirects, 'CheckWork.maxRedirects');
  assert(work.properties.maxRedirects.const === 3, 'CheckWork.maxRedirects.const must be 3');

  const checkResult = document.components.schemas.CheckResult;
  assert(
    isObject(checkResult) && Array.isArray(checkResult.oneOf) && checkResult.oneOf.length === 2,
    'CheckResult must use exactly two oneOf branches',
  );
  assertExactKeys(checkResult, ['oneOf'], 'CheckResult');
  assertSchemaTarget(document, checkResult.oneOf[0], 'HttpResponseResult', 'CheckResult oneOf[0]');
  assertSchemaTarget(document, checkResult.oneOf[1], 'FailureResult', 'CheckResult oneOf[1]');

  const httpResult = document.components.schemas.HttpResponseResult;
  assertObjectShape(
    httpResult,
    ['kind', 'durationMs', 'httpStatus'],
    ['kind', 'durationMs', 'httpStatus'],
    'HttpResponseResult',
  );
  assert(httpResult.properties.kind?.type === 'string', 'HttpResponseResult.kind.type must be string');
  assert(httpResult.properties.kind?.const === 'http_response', 'HttpResponseResult.kind.const must be http_response');
  assertInteger(httpResult.properties.durationMs, 'HttpResponseResult.durationMs');
  assert(httpResult.properties.durationMs.minimum === 0, 'HttpResponseResult.durationMs.minimum must be 0');
  assert(httpResult.properties.durationMs.maximum === 20000, 'HttpResponseResult.durationMs.maximum must be 20000');
  assertInteger(httpResult.properties.httpStatus, 'HttpResponseResult.httpStatus');
  assert(httpResult.properties.httpStatus.minimum === 100, 'HttpResponseResult.httpStatus.minimum must be 100');
  assert(httpResult.properties.httpStatus.maximum === 599, 'HttpResponseResult.httpStatus.maximum must be 599');

  const failureResult = document.components.schemas.FailureResult;
  assertObjectShape(
    failureResult,
    ['kind', 'durationMs'],
    ['kind', 'durationMs'],
    'FailureResult',
  );
  assert(failureResult.properties.kind?.type === 'string', 'FailureResult.kind.type must be string');
  assert(
    Array.isArray(failureResult.properties.kind?.enum),
    'FailureResult.kind.enum must be an array',
  );
  assertExactSet(
    failureResult.properties.kind.enum,
    [
      'dns_error',
      'policy_rejected',
      'timeout',
      'connect_error',
      'tls_error',
      'protocol_error',
      'internal_error',
    ],
    'FailureResult.kind.enum',
  );
  assertInteger(failureResult.properties.durationMs, 'FailureResult.durationMs');
  assert(failureResult.properties.durationMs.minimum === 0, 'FailureResult.durationMs.minimum must be 0');
  assert(failureResult.properties.durationMs.maximum === 20000, 'FailureResult.durationMs.maximum must be 20000');

  const problem = document.components.schemas.Problem;
  assert(isObject(problem), 'Problem must be an object');
  assert(problem.type === 'object', 'Problem.type must be object');
  assert(problem.additionalProperties === false, 'Problem.additionalProperties must be false');
  assertExactKeys(problem.properties, ['type', 'title', 'status', 'detail', 'instance'], 'Problem.properties');
  assertStringFormat(problem.properties.type, 'uri-reference', 'Problem.type');
  assert(problem.properties.title?.type === 'string', 'Problem.title.type must be string');
  assertInteger(problem.properties.status, 'Problem.status');
  assert(problem.properties.status.minimum === 100, 'Problem.status.minimum must be 100');
  assert(problem.properties.status.maximum === 599, 'Problem.status.maximum must be 599');
  assert(problem.properties.detail?.type === 'string', 'Problem.detail.type must be string');
  assertStringFormat(problem.properties.instance, 'uri-reference', 'Problem.instance');
}

function main() {
  const [bundlePath] = process.argv.slice(2);
  if (!bundlePath) {
    console.error('Usage: check-internal-contract.mjs <bundled-json>');
    process.exit(2);
  }

  try {
    const document = JSON.parse(fs.readFileSync(bundlePath, 'utf8'));
    validateInternalContract(document);
    console.log('Internal contract semantic check: PASS');
  } catch (error) {
    if (error instanceof ContractInvariantError) {
      console.error(`Internal contract invariant failed: ${error.message}`);
      process.exit(1);
    }
    console.error(`Internal contract check failed: ${error.message}`);
    process.exit(1);
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href) {
  main();
}
