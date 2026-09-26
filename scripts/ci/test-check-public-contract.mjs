#!/usr/bin/env node

import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const checkerPath = fileURLToPath(new URL('./check-public-contract.mjs', import.meta.url));
const tempRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'uptime-lab-contract-'));
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
      title: 'uptime-lab Public API',
      version: '0.1.0',
    },
    paths: {
      '/monitors': {
        post: {
          operationId: 'registerMonitor',
          requestBody: {
            required: true,
            content: {
              'application/json': {
                schema: { $ref: '#/components/schemas/CreateMonitorRequest' },
              },
            },
          },
          responses: {
            '201': {
              description: 'Created.',
              headers: {
                Location: {
                  schema: { type: 'string', format: 'uri-reference' },
                },
              },
              content: {
                'application/json': {
                  schema: { $ref: '#/components/schemas/Monitor' },
                },
              },
            },
            '400': problemResponse(),
            '415': problemResponse(),
            '422': problemResponse(),
            '500': problemResponse(),
          },
        },
      },
      '/monitors/{monitorId}': {
        get: {
          operationId: 'getMonitor',
          parameters: [
            {
              name: 'monitorId',
              in: 'path',
              required: true,
              schema: { type: 'string', format: 'uuid' },
            },
          ],
          responses: {
            '200': {
              description: 'Found.',
              content: {
                'application/json': {
                  schema: { $ref: '#/components/schemas/Monitor' },
                },
              },
            },
            '400': problemResponse(),
            '404': problemResponse(),
            '500': problemResponse(),
          },
        },
      },
    },
    components: {
      schemas: {
        CreateMonitorRequest: {
          type: 'object',
          additionalProperties: false,
          required: ['targetUrl'],
          properties: {
            targetUrl: { type: 'string' },
          },
        },
        Monitor: {
          type: 'object',
          additionalProperties: false,
          required: ['id', 'targetUrl', 'createdAt'],
          properties: {
            id: {
              type: 'string',
              format: 'uuid',
              description: 'Stable monitor UUID.',
            },
            targetUrl: { type: 'string' },
            createdAt: { type: 'string', format: 'date-time' },
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

function fixtureRoot(name) {
  const root = path.join(tempRoot, name.replace(/[^a-z0-9]+/gi, '-').toLowerCase());
  fs.mkdirSync(path.join(root, 'contracts', 'openapi'), { recursive: true });
  fs.writeFileSync(path.join(root, 'contracts', 'openapi', 'public.yaml'), 'fixture\n');
  return root;
}

function runChecker(name, document) {
  const root = fixtureRoot(name);
  const bundle = path.join(root, 'bundle.json');
  fs.writeFileSync(bundle, JSON.stringify(document));
  return spawnSync(process.execPath, [checkerPath, bundle, root], {
    encoding: 'utf8',
  });
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
  else fail(name, result.stderr.trim() || `exit ${result.status}`);
}

function expectReject(name, mutate, expected, options = {}) {
  const document = clone(validDocument());
  mutate(document);
  const result = runChecker(name, document, options);
  const output = `${result.stdout}\n${result.stderr}`;
  if (result.status !== 0 && output.includes(expected)) pass(name);
  else fail(name, `expected rejection containing "${expected}", got exit=${result.status}: ${output.trim()}`);
}

expectPass('canonical fixture passes');

expectReject('OpenAPI version mismatch fails', (d) => { d.openapi = '3.2.1'; }, 'openapi must be exactly 3.1.2');
expectReject('contract version mismatch fails', (d) => { d.info.version = '0.2.0'; }, 'info.version must be exactly 0.1.0');
expectReject('extra public path fails', (d) => { d.paths['/extra'] = { get: {} }; }, 'public path set');
expectReject('GET /monitors fails', (d) => { d.paths['/monitors'].get = {}; }, '/monitors operations');
expectReject('PUT lifecycle mutation fails', (d) => { d.paths['/monitors/{monitorId}'].put = {}; }, '/monitors/{monitorId} operations');
expectReject('PATCH lifecycle mutation fails', (d) => { d.paths['/monitors/{monitorId}'].patch = {}; }, '/monitors/{monitorId} operations');
expectReject('DELETE lifecycle mutation fails', (d) => { d.paths['/monitors/{monitorId}'].delete = {}; }, '/monitors/{monitorId} operations');
expectReject('missing POST fails', (d) => { delete d.paths['/monitors'].post; }, '/monitors operations');
expectReject('missing GET by ID fails', (d) => { delete d.paths['/monitors/{monitorId}'].get; }, '/monitors/{monitorId} operations');
expectReject('wrong register operationId fails', (d) => { d.paths['/monitors'].post.operationId = 'createMonitor'; }, 'operationId must be registerMonitor');
expectReject('wrong get operationId fails', (d) => { d.paths['/monitors/{monitorId}'].get.operationId = 'findMonitor'; }, 'operationId must be getMonitor');
expectReject('wrong POST response set fails', (d) => { d.paths['/monitors'].post.responses['202'] = problemResponse(); }, 'POST /monitors responses');
expectReject('wrong GET response set fails', (d) => { d.paths['/monitors/{monitorId}'].get.responses['204'] = {}; }, 'GET /monitors/{monitorId} responses');
expectReject('missing Location header fails', (d) => { delete d.paths['/monitors'].post.responses['201'].headers.Location; }, 'must define Location header');
expectReject('non URI-reference Location fails', (d) => { d.paths['/monitors'].post.responses['201'].headers.Location.schema.format = 'uri'; }, 'Location schema.format must be uri-reference');
expectReject('optional POST request body fails', (d) => { d.paths['/monitors'].post.requestBody.required = false; }, 'requestBody.required must be true');
expectReject('wrong POST request media fails', (d) => {
  const content = d.paths['/monitors'].post.requestBody.content;
  content['text/plain'] = content['application/json'];
  delete content['application/json'];
}, 'requestBody.content keys');
expectReject('targetUrl RFC uri format fails', (d) => { d.components.schemas.CreateMonitorRequest.properties.targetUrl.format = 'uri'; }, 'CreateMonitorRequest.targetUrl.format must be absent');
expectReject('Monitor id without uuid format fails', (d) => { delete d.components.schemas.Monitor.properties.id.format; }, 'Monitor.id.format must be uuid');
expectReject('Monitor targetUrl RFC uri format fails', (d) => { d.components.schemas.Monitor.properties.targetUrl.format = 'uri'; }, 'Monitor.targetUrl.format must be absent');
expectReject('Monitor createdAt without date-time format fails', (d) => { delete d.components.schemas.Monitor.properties.createdAt.format; }, 'Monitor.createdAt.format must be date-time');
expectReject('UUID v7 public promise fails', (d) => { d.components.schemas.Monitor.properties.id.description = 'UUID v7 identity.'; }, 'must not promise UUID v7');
expectReject('wrong POST success media fails', (d) => {
  const content = d.paths['/monitors'].post.responses['201'].content;
  content['application/problem+json'] = content['application/json'];
  delete content['application/json'];
}, 'POST /monitors 201 content keys');
expectReject('wrong GET success media fails', (d) => {
  const content = d.paths['/monitors/{monitorId}'].get.responses['200'].content;
  content['application/problem+json'] = content['application/json'];
  delete content['application/json'];
}, 'GET /monitors/{monitorId} 200 content keys');
expectReject('missing problem media fails', (d) => {
  const content = d.paths['/monitors'].post.responses['422'].content;
  content['application/json'] = content['application/problem+json'];
  delete content['application/problem+json'];
}, 'POST /monitors 422 content keys');
expectReject('extra request property fails', (d) => { d.components.schemas.CreateMonitorRequest.properties.name = { type: 'string' }; }, 'CreateMonitorRequest.properties keys');
expectReject('request additionalProperties relaxation fails', (d) => { d.components.schemas.CreateMonitorRequest.additionalProperties = true; }, 'CreateMonitorRequest.additionalProperties must be false');
expectReject('Monitor additionalProperties relaxation fails', (d) => { d.components.schemas.Monitor.additionalProperties = true; }, 'Monitor.additionalProperties must be false');
expectReject('extra Monitor field fails', (d) => { d.components.schemas.Monitor.properties.enabled = { type: 'boolean' }; }, 'Monitor.properties keys');
expectReject('missing required Monitor field fails', (d) => { d.components.schemas.Monitor.required = ['id', 'targetUrl']; }, 'Monitor.required');
expectReject('root security fails', (d) => { d.security = []; }, 'root security must be absent');
expectReject('operation security fails', (d) => { d.paths['/monitors'].post.security = []; }, 'POST /monitors must not define security');
expectReject('securitySchemes fails', (d) => { d.components.securitySchemes = {}; }, 'components.securitySchemes must be absent');
expectReject('root servers fails', (d) => { d.servers = [{ url: 'https://example.test' }]; }, 'root servers must be absent');
expectReject('operation servers fails', (d) => { d.paths['/monitors'].post.servers = [{ url: 'https://example.test' }]; }, 'POST /monitors must not define servers');
expectReject('extra component schema fails', (d) => { d.components.schemas.Future = { type: 'object' }; }, 'components.schemas keys');
expectReject('monitorId format mismatch fails', (d) => { d.paths['/monitors/{monitorId}'].get.parameters[0].schema.format = 'string'; }, 'monitorId parameter schema.format must be uuid');
expectReject('Problem type format mismatch fails', (d) => { d.components.schemas.Problem.properties.type.format = 'uri'; }, 'Problem.type.format must be uri-reference');
expectReject('Problem instance format mismatch fails', (d) => { d.components.schemas.Problem.properties.instance.format = 'uri'; }, 'Problem.instance.format must be uri-reference');
expectReject('Problem status range mismatch fails', (d) => { d.components.schemas.Problem.properties.status.maximum = 999; }, 'Problem.status.maximum must be 599');
console.log(`\nPublic contract semantic tests: ${passed} passed, ${failed} failed`);
if (failed !== 0) process.exit(1);
