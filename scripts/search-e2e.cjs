// Isolated live/static acceptance environment. Never edits the checked-out site.
const fs = require('fs');
const os = require('os');
const path = require('path');
const http = require('http');
const { spawn, spawnSync } = require('child_process');
const repo = path.resolve(__dirname, '..');
const root = fs.mkdtempSync(path.join(os.tmpdir(), 'foundry-search-e2e-'));
const livePort = Number(process.env.FOUNDRY_SEARCH_LIVE_PORT || 18753);
const staticPort = Number(process.env.FOUNDRY_SEARCH_STATIC_PORT || 18754);
const write = (name, body) => {
  const target = path.join(root, name);
  fs.mkdirSync(path.dirname(target), { recursive: true });
  fs.writeFileSync(target, body);
};
write(
  'content/config/site.yaml',
  `title: Search Acceptance\nbase_url: http://127.0.0.1:${livePort}\ndefault_lang: en\nlanguages: [en, es]\ntheme: default\nthemes_dir: ${path.join(repo, 'themes')}\nadmin:\n  enabled: false\nserver:\n  addr: "127.0.0.1:${livePort}"\n`
);
for (const [slug, title, lang, body, workflow] of [
  ['alpha', 'Alpha guide', 'en', 'Alpha documentation body.', 'published'],
  ['body', 'Body reference', 'en', 'Alpha appears in this body.', 'published'],
  ['spanish', 'Guía de café', 'es', 'Café español.', 'published'],
  ['draft', 'Secret draft', 'en', 'Alpha secret.', 'draft'],
  ['review', 'Secret review', 'en', 'Alpha secret.', 'in_review'],
  ['future', 'Secret future', 'en', 'Alpha secret.', 'scheduled'],
  ['archived', 'Secret archived', 'en', 'Alpha secret.', 'archived'],
])
  write(
    `content/pages/${lang === 'en' ? '' : `${lang}/`}${slug}.md`,
    `---\ntitle: ${title}\nlang: ${lang}\nworkflow: ${workflow}\n${workflow === 'scheduled' ? 'scheduled_publish_at: "2099-01-01T00:00:00Z"\n' : ''}---\n${body}\n`
  );
write('content/pages/index.md', '---\ntitle: Home\n---\nWelcome to the search acceptance site.\n');
fs.mkdirSync(path.join(root, 'content/posts'), { recursive: true });
const binary = path.join(root, 'foundry');
const build = spawnSync('go', ['build', '-o', binary, './cmd/foundry'], {
  cwd: repo,
  env: process.env,
  stdio: 'inherit',
});
if (build.status !== 0) process.exit(1);
const siteBuild = spawnSync(binary, ['build'], { cwd: root, stdio: 'inherit' });
if (siteBuild.status !== 0) process.exit(1);
const live = spawn(binary, ['serve'], { cwd: root, stdio: 'inherit' });
const server = http.createServer((req, res) => {
  let requested;
  try {
    requested = decodeURIComponent(new URL(req.url, 'http://localhost').pathname);
  } catch (_) {
    res.writeHead(400).end();
    return;
  }
  const publicRoot = path.join(root, 'public');
  let target = path.resolve(publicRoot, `.${requested}`);
  if (!target.startsWith(`${publicRoot}${path.sep}`) && target !== publicRoot) {
    res.writeHead(403).end();
    return;
  }
  if (fs.existsSync(target) && fs.statSync(target).isDirectory())
    target = path.join(target, 'index.html');
  if (!fs.existsSync(target)) {
    res.writeHead(404).end('Not found');
    return;
  }
  const types = {
    '.js': 'text/javascript',
    '.json': 'application/json',
    '.html': 'text/html',
    '.css': 'text/css',
  };
  res.setHeader('Content-Type', types[path.extname(target)] || 'application/octet-stream');
  fs.createReadStream(target).pipe(res);
});
let closing = false;
const close = () => {
  if (closing) return;
  closing = true;
  live.kill('SIGTERM');
  server.close();
  fs.rmSync(root, { recursive: true, force: true });
  process.exit(0);
};
process.on('SIGTERM', close);
process.on('SIGINT', close);
live.on('exit', (code) => {
  if (!closing) process.exit(code || 1);
});
(async () => {
  for (let attempt = 0; attempt < 120; attempt++) {
    try {
      const response = await fetch(`http://127.0.0.1:${livePort}/search/`);
      if (response.ok) {
        server.listen(staticPort, '127.0.0.1');
        return;
      }
    } catch (_) {}
    await new Promise((resolve) => setTimeout(resolve, 250));
  }
  throw new Error('Search live server did not become ready');
})().catch((error) => {
  console.error(error);
  live.kill();
  process.exit(1);
});
