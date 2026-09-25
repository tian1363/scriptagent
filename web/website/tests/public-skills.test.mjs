import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { execFileSync } from 'node:child_process';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const website = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');

test('publishes source-backed, indexable Skill pages', () => {
  execFileSync('node', ['scripts/generate-public-skills.mjs'], {
    cwd: website,
    env: { ...process.env, SITE_BASE_PATH: '/scriptagent/' },
  });
  const sitemap = readFileSync(path.join(website, 'public/sitemap.xml'), 'utf8');
  const listing = readFileSync(path.join(website, 'public/skills/index.html'), 'utf8');
  assert.match(listing, /href="\/scriptagent\/skills\/ugc-hook-writer\/"/);
  for (const slug of ['ugc-hook-writer', 'product-selling-point-writer', 'fission-strategy', 'script-review']) {
    const page = readFileSync(path.join(website, `public/skills/${slug}/index.html`), 'utf8');
    assert.match(page, new RegExp(`rel="canonical" href="https://tian1363.github.io/scriptagent/skills/${slug}/"`));
    assert.match(page, /完整工作流/);
    assert.match(page, /github.com\/tian1363\/scriptagent\/blob\/main\/internal\/chat\/service.go/);
    assert.match(sitemap, new RegExp(`<loc>https://tian1363.github.io/scriptagent/skills/${slug}/</loc>`));
  }
  assert.match(readFileSync(path.join(website, 'public/skills/ugc-hook-writer/index.html'), 'utf8'), /首帧画面/);
  assert.match(readFileSync(path.join(website, 'public/skills/product-selling-point-writer/index.html'), 'utf8'), /待核实/);
});
