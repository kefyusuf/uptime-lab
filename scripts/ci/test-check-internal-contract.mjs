#!/usr/bin/env node

import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const checkerPath = fileURLToPath(new URL('./check-internal-contract.mjs', import.meta.url));
const tempRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'uptime-lab-internal-contract-'));
process.on('exit', () => fs.rmSync(tempRoot, { recursive: true, force: true }));

let passed = 0;
let failed = 0;

function problemResponse() {
  return {
    description: 'Problem.',
    content: {
      'application/problem+json': {
        schema: { $ref: '#/components/schemas/Problem' },
      },
    },
  };
}

function validDocument() {
  return {
    openapi: '3.1.2',
    info: {
      title: 'uptime-lab Internal Checker API',
      version: '0.1.0',
    },
    paths: {
      '/internal/checks/claim': {
        post: {
          responses: {
            '200': {
              description: 'One due check.',
              content: {
                'application/json': {
                  schema: { $ref: '#/components/schemas/CheckWork' },
                },
              },
            },
            '204': { description: 'No due work.' },
            '405': problemResponse(),
            '500': problemResponse(),
          },
        },
      },
      '/internal/checks/{checkId}/result': {
        put: {
          parameters: [
            {
              name: 'checkId',
              in: 'path',
              required: true,
              schema: { type: 'string', format: 'uuid' },
            },
          ],
          requestBody: {
            required: true,
            content: {
              'application/json': {
                schema: { $ref: '#/components/schemas/CheckResult' },
              },
            },
          },
          responses: {
            '204': { description: 'Accepted or exact duplicate.' },
            '400': problemResponse(),
            '404': problemResponse(),
            '409': problemResponse(),
            '415': problemResponse(),
            '422': problemResponse(),
            '405': problemResponse(),
            '500': problemResponse(),
          },
        },
      },
    },
    components: {
      schemas: {
        CheckWork: {
          type: 'object',
          additionalProperties: false,
          required: ['checkId', 'monitorId', 'targetUrl', 'timeoutMs', 'maxRedirects'],
          properties: {
            checkId: { type: 'string', format: 'uuid' },
            monitorId: { type: 'string', format: 'uuid' },
            targetUrl: { type: 'string' },
            timeoutMs: { type: 'integer', const: 10000 },
            maxRedirects: { type: 'integer', const: 3 },
          },
        },
        CheckResult: {
          oneOf: [
            { $ref: '#/components/schemas/HttpResponseResult' },
            { $ref: '#/components/schemas/FailureResult' },
          ],
        },
        HttpResponseResult: {
          type: 'object',
          additionalProperties: false,
          required: ['kind', 'durationMs', 'httpStatus'],
          properties: {
            kind: { type: 'string', const: 'http_response' },
            durationMs: { type: 'integer', minimum: 0, maximum: 20000 },
            httpStatus: { type: 'integer', minimum: 100, maximum: 599 },
          },
        },
        FailureResult: {
          type: 'object',
          additionalProperties: false,
          required: ['kind', 'durationMs'],
          properties: {
            kind: {
              type: 'string',
              enum: [
                'dns_error',
                'policy_rejected',
                'timeout',
                'connect_error',
                'tls_error',
                'protocol_error',
                'internal_error',
              ],
            },
            durationMs: { type: 'integer', minimum: 0, maximum: 20000 },
          },
        },
        Problem: {
          type: 'object',
          additionalProperties: false,
          properties: {
            type: { type: 'string', format: 'uri-reference' },
            title: { type: 'string' },
            status: { type: 'integer', minimum: 100, maximum: 599 },
            detail: { type: 'string' },
            instance: { type: 'string', format: 'uri-reference' },
          },
        },
      },
    },
  };
}

function clone(value) {
  return structuredClone(value);
}

function runChecker(name, document) {
  const bundle = path.join(tempRoot, `${name.replace(/[^a-z0-9]+/gi, '-').toLowerCase()}.json`);
  fs.writeFileSync(bundle, JSON.stringify(document));
  return spawnSync(process.execPath, [checkerPath, bundle], { encoding: 'utf8' });
}

function pass(name) {
  console.log(`PASS: ${name}`);
  passed += 1;
}

function fail(name, detail) {
  console.error(`FAIL: ${name}: ${detail}`);
  failed += 1;
}

function expectPass(name, mutate = () => {}) {
  const document = clone(validDocument());
  mutate(document);
  const result = runChecker(name, document);
  if (result.status === 0) pass(name);
  else fail(name, result.stderr.trim() || result.stdout.trim() || `exit ${result.status}`);
}

function expectReject(name, mutate, expected) {
  const document = clone(validDocument());
  mutate(document);
  const result = runChecker(name, document);
  const output = `${result.stdout}\n${result.stderr}`;
  if (result.status !== 0 && output.includes(expected)) pass(name);
  else fail(name, `expected rejection containing "${expected}", got exit=${result.status}: ${output.trim()}`);
}

expectPass('canonical internal contract fixture passes');

expectReject('OpenAPI version mismatch fails', (d) => { d.openapi = '3.2.0'; }, 'openapi must be exactly 3.1.2');
expectReject('contract version mismatch fails', (d) => { d.info.version = '0.2.0'; }, 'info.version must be exactly 0.1.0');
expectReject('extra internal path fails', (d) => { d.paths['/internal/extra'] = { get: {} }; }, 'internal path set');
expectReject('copied public monitor path fails', (d) => { d.paths['/monitors'] = { post: {} }; }, 'internal path set');
expectReject('GET work acquisition fails', (d) => {
  d.paths['/internal/checks/claim'].get = d.paths['/internal/checks/claim'].post;
  delete d.paths['/internal/checks/claim'].post;
}, '/internal/checks/claim operations');
expectReject('POST result submission fails', (d) => {
  d.paths['/internal/checks/{checkId}/result'].post = d.paths['/internal/checks/{checkId}/result'].put;
  delete d.paths['/internal/checks/{checkId}/result'].put;
}, '/internal/checks/{checkId}/result operations');
expectReject('claim request body fails', (d) => {
  d.paths['/internal/checks/claim'].post.requestBody = {
    required: true,
    content: { 'application/json': { schema: { type: 'object' } } },
  };
}, 'claim requestBody must be absent');
expectReject('wrong claim response set fails', (d) => {
  d.paths['/internal/checks/claim'].post.responses['404'] = problemResponse();
}, 'claim responses');
expectReject('claim 204 content fails', (d) => {
  d.paths['/internal/checks/claim'].post.responses['204'].content = {
    'application/json': { schema: { type: 'object' } },
  };
}, 'claim 204 content must be absent');
expectReject('claim 200 wrong media type fails', (d) => {
  const content = d.paths['/internal/checks/claim'].post.responses['200'].content;
  content['text/plain'] = content['application/json'];
  delete content['application/json'];
}, 'claim 200 content keys');
expectReject('extra CheckWork field fails', (d) => {
  d.components.schemas.CheckWork.properties.workerId = { type: 'string' };
}, 'CheckWork.properties keys');
expectReject('worker identity field fails', (d) => {
  d.components.schemas.CheckWork.properties.workerId = { type: 'string' };
  d.components.schemas.CheckWork.required.push('workerId');
}, 'CheckWork.properties keys');
expectReject('CheckWork additionalProperties relaxation fails', (d) => {
  d.components.schemas.CheckWork.additionalProperties = true;
}, 'CheckWork.additionalProperties must be false');
expectReject('CheckWork required set mismatch fails', (d) => {
  d.components.schemas.CheckWork.required = ['checkId', 'monitorId', 'targetUrl', 'timeoutMs'];
}, 'CheckWork.required');
expectReject('checkId without uuid format fails', (d) => {
  delete d.components.schemas.CheckWork.properties.checkId.format;
}, 'CheckWork.checkId.format must be uuid');
expectReject('monitorId without uuid format fails', (d) => {
  delete d.components.schemas.CheckWork.properties.monitorId.format;
}, 'CheckWork.monitorId.format must be uuid');
expectReject('targetUrl RFC uri format fails', (d) => {
  d.components.schemas.CheckWork.properties.targetUrl.format = 'uri';
}, 'CheckWork.targetUrl.format must be absent');
expectReject('timeoutMs is not fixed fails', (d) => {
  d.components.schemas.CheckWork.properties.timeoutMs.const = 9000;
}, 'CheckWork.timeoutMs.const must be 10000');
expectReject('maxRedirects is not fixed fails', (d) => {
  d.components.schemas.CheckWork.properties.maxRedirects.const = 5;
}, 'CheckWork.maxRedirects.const must be 3');

expectReject('result checkId path format mismatch fails', (d) => {
  d.paths['/internal/checks/{checkId}/result'].put.parameters[0].schema.format = 'string';
}, 'result checkId.format must be uuid');
expectReject('optional result body fails', (d) => {
  d.paths['/internal/checks/{checkId}/result'].put.requestBody.required = false;
}, 'result requestBody.required must be true');
expectReject('wrong result request media fails', (d) => {
  const content = d.paths['/internal/checks/{checkId}/result'].put.requestBody.content;
  content['text/plain'] = content['application/json'];
  delete content['application/json'];
}, 'result requestBody.content keys');
expectReject('wrong result response set fails', (d) => {
  d.paths['/internal/checks/{checkId}/result'].put.responses['201'] = {};
}, 'result responses');
expectReject('result 204 content fails', (d) => {
  d.paths['/internal/checks/{checkId}/result'].put.responses['204'].content = {
    'application/json': { schema: { type: 'object' } },
  };
}, 'result 204 content must be absent');

expectReject('CheckResult arbitrary map fails', (d) => {
  d.components.schemas.CheckResult = { type: 'object', additionalProperties: true };
}, 'CheckResult must use exactly two oneOf branches');
expectReject('HTTP result extra raw error field fails', (d) => {
  d.components.schemas.HttpResponseResult.properties.error = { type: 'string' };
}, 'HttpResponseResult.properties keys');
expectReject('failure result raw message field fails', (d) => {
  d.components.schemas.FailureResult.properties.message = { type: 'string' };
}, 'FailureResult.properties keys');
expectReject('HTTP result additionalProperties relaxation fails', (d) => {
  d.components.schemas.HttpResponseResult.additionalProperties = true;
}, 'HttpResponseResult.additionalProperties must be false');
expectReject('failure result additionalProperties relaxation fails', (d) => {
  d.components.schemas.FailureResult.additionalProperties = true;
}, 'FailureResult.additionalProperties must be false');
expectReject('worker_timeout submitted by Rust fails', (d) => {
  d.components.schemas.FailureResult.properties.kind.enum.push('worker_timeout');
}, 'FailureResult.kind.enum');
expectReject('duration lower bound mismatch fails', (d) => {
  d.components.schemas.FailureResult.properties.durationMs.minimum = -1;
}, 'FailureResult.durationMs.minimum must be 0');
expectReject('duration upper bound mismatch fails', (d) => {
  d.components.schemas.FailureResult.properties.durationMs.maximum = 2147483647;
}, 'FailureResult.durationMs.maximum must be 20000');
expectReject('HTTP duration upper bound mismatch fails', (d) => {
  d.components.schemas.HttpResponseResult.properties.durationMs.maximum = 30000;
}, 'HttpResponseResult.durationMs.maximum must be 20000');
expectReject('HTTP status lower bound mismatch fails', (d) => {
  d.components.schemas.HttpResponseResult.properties.httpStatus.minimum = 0;
}, 'HttpResponseResult.httpStatus.minimum must be 100');
expectReject('HTTP status upper bound mismatch fails', (d) => {
  d.components.schemas.HttpResponseResult.properties.httpStatus.maximum = 999;
}, 'HttpResponseResult.httpStatus.maximum must be 599');
expectReject('failure result gains httpStatus fails', (d) => {
  d.components.schemas.FailureResult.properties.httpStatus = { type: 'integer' };
}, 'FailureResult.properties keys');

expectReject('root security fails', (d) => { d.security = []; }, 'root security must be absent');
expectReject('operation security fails', (d) => { d.paths['/internal/checks/claim'].post.security = []; }, 'claim operation security must be absent');
expectReject('securitySchemes fails', (d) => { d.components.securitySchemes = {}; }, 'components.securitySchemes must be absent');
expectReject('root servers fails', (d) => { d.servers = [{ url: 'http://api:8080' }]; }, 'root servers must be absent');
expectReject('extra component schema fails', (d) => { d.components.schemas.Future = { type: 'object' }; }, 'components.schemas keys');
expectReject('Problem additionalProperties relaxation fails', (d) => {
  d.components.schemas.Problem.additionalProperties = true;
}, 'Problem.additionalProperties must be false');

console.log(`\nInternal contract semantic tests: ${passed} passed, ${failed} failed`);
if (failed !== 0) process.exit(1);
