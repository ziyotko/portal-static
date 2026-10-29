import assert from 'node:assert/strict';
import { mkdtemp, rm, writeFile } from 'node:fs/promises';
import http from 'node:http';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { after, test } from 'node:test';
import { createPreviewServer } from './serve-camie-preview.mjs';

const servers = [];
const root = await mkdtemp(path.join(tmpdir(), 'camie-preview-'));
await writeFile(path.join(root, 'index.html'), '<h1>CAMIE</h1>');

async function listen(server) {
  servers.push(server);
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  return `http://127.0.0.1:${server.address().port}`;
}

const memberFrontend = await listen(http.createServer((request, response) => {
  response.end(`member frontend: ${request.url}`);
}));
const memberBackend = await listen(http.createServer((request, response) => {
  response.end(`member backend: ${request.url}`);
}));
const portalFrontend = await listen(http.createServer((request, response) => {
  response.end(`portal frontend: ${request.url}`);
}));
const portalBackend = await listen(http.createServer((request, response) => {
  response.end(`portal backend: ${request.url}`);
}));
const preview = await listen(createPreviewServer({ root, memberFrontend, memberBackend, portalFrontend, portalBackend }));

after(async () => {
  await Promise.all(servers.map((server) => new Promise((resolve) => server.close(resolve))));
  await rm(root, { recursive: true, force: true });
});

test('serves generated static pages and keeps missing paths as 404', async () => {
  const page = await fetch(preview);
  assert.equal(page.status, 200);
  assert.match(await page.text(), /CAMIE/);
  assert.equal((await fetch(`${preview}/missing.html`)).status, 404);
});

test('routes login and APIs to their separate local services', async () => {
  const routes = [
    ['/business_member/login?returnUrl=%2Fpages%2Fmember.html', 'member frontend'],
    ['/business_member/api/member/profile', 'member backend'],
    ['/business_portal/api/member-zone/statistics/dashboard', 'portal backend'],
    ['/business_portal/', 'portal frontend'],
  ];
  for (const [route, service] of routes) {
    const response = await fetch(preview + route);
    assert.equal(response.status, 200);
    assert.equal(await response.text(), `${service}: ${route}`);
  }
});
