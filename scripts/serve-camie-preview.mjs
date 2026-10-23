import { createReadStream } from 'node:fs';
import { stat } from 'node:fs/promises';
import http from 'node:http';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const projectRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const defaultRoot = path.join(projectRoot, 'dist', 'camie-portal');
const contentTypes = {
  '.css': 'text/css; charset=utf-8',
  '.html': 'text/html; charset=utf-8',
  '.ico': 'image/x-icon',
  '.jpeg': 'image/jpeg',
  '.jpg': 'image/jpeg',
  '.js': 'text/javascript; charset=utf-8',
  '.json': 'application/json; charset=utf-8',
  '.png': 'image/png',
  '.svg': 'image/svg+xml',
  '.webp': 'image/webp',
  '.woff': 'font/woff',
  '.woff2': 'font/woff2',
};

export function createPreviewServer({
  root = defaultRoot,
  memberFrontend = 'http://127.0.0.1:3001',
  memberBackend = 'http://127.0.0.1:8093',
  portalFrontend = 'http://127.0.0.1:3000',
  portalBackend = 'http://127.0.0.1:8092',
} = {}) {
  const siteRoot = path.resolve(root);
  const targets = {
    memberFrontend: new URL(memberFrontend),
    memberBackend: new URL(memberBackend),
    portalFrontend: new URL(portalFrontend),
    portalBackend: new URL(portalBackend),
  };

  return http.createServer(async (request, response) => {
    const requestURL = new URL(request.url, 'http://localhost');
    const pathname = requestURL.pathname;
    let target;
    if (pathname.startsWith('/business_member/api/') || pathname.startsWith('/business_member/uploads/')) {
      target = targets.memberBackend;
    } else if (pathname.startsWith('/business_portal/api/') || pathname.startsWith('/business_portal/uploads/')) {
      target = targets.portalBackend;
    } else if (pathname === '/business_member' || pathname.startsWith('/business_member/')) {
      target = targets.memberFrontend;
    } else if (pathname === '/business_portal' || pathname.startsWith('/business_portal/')) {
      target = targets.portalFrontend;
    }

    if (target) {
      const upstreamURL = new URL(pathname + requestURL.search, target);
      const upstream = http.request(upstreamURL, {
        method: request.method,
        headers: {
          ...request.headers,
          host: target.host,
          'x-forwarded-host': request.headers.host || '',
          'x-forwarded-proto': 'http',
        },
      }, (upstreamResponse) => {
        response.writeHead(upstreamResponse.statusCode, upstreamResponse.headers);
        upstreamResponse.pipe(response);
      });
      upstream.on('error', () => {
        if (!response.headersSent) {
          response.writeHead(502, { 'content-type': 'text/plain; charset=utf-8' });
          response.end(`本地服务未启动：${target.origin}\n`);
        } else {
          response.destroy();
        }
      });
      request.pipe(upstream);
      return;
    }

    if (request.method !== 'GET' && request.method !== 'HEAD') {
      response.writeHead(405, { Allow: 'GET, HEAD' });
      response.end();
      return;
    }

    let relativePath;
    try {
      relativePath = decodeURIComponent(pathname).replace(/^\/+/, '');
    } catch {
      response.writeHead(400);
      response.end();
      return;
    }
    if (relativePath.includes('\\') || relativePath.includes('\0')) {
      response.writeHead(400);
      response.end();
      return;
    }
    let filePath = path.resolve(siteRoot, relativePath);
    if (filePath !== siteRoot && !filePath.startsWith(siteRoot + path.sep)) {
      response.writeHead(403);
      response.end();
      return;
    }
    try {
      let info = await stat(filePath);
      if (info.isDirectory()) {
        filePath = path.join(filePath, 'index.html');
        info = await stat(filePath);
      }
      if (!info.isFile()) throw new Error('not a file');
      response.writeHead(200, {
        'content-type': contentTypes[path.extname(filePath).toLowerCase()] || 'application/octet-stream',
        'content-length': info.size,
        'cache-control': 'no-store',
      });
      if (request.method === 'HEAD') response.end();
      else createReadStream(filePath).pipe(response);
    } catch {
      response.writeHead(404, { 'content-type': 'text/plain; charset=utf-8' });
      response.end('页面不存在，请先在后台生成全站。\n');
    }
  });
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const port = Number(process.env.CAMIE_PREVIEW_PORT || 8090);
  const server = createPreviewServer();
  server.listen(port, '127.0.0.1', () => {
    console.log(`CAMIE 本地预览：http://127.0.0.1:${port}/`);
    console.log(`会员数据中心：http://127.0.0.1:${port}/pages/member.html?column=数据中心`);
  });
}
