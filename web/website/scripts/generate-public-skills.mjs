#!/usr/bin/env node
import { readFileSync, writeFileSync, mkdirSync, rmSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const website = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const root = path.resolve(website, '../..');
const source = readFileSync(path.join(root, 'internal/chat/service.go'), 'utf8');
const output = path.join(website, 'public/skills');
const base = 'https://tian1363.github.io/scriptagent/';
const repository = 'https://github.com/tian1363/scriptagent';
const sitePath = `/${(process.env.SITE_BASE_PATH || '/').split('/').filter(Boolean).join('/')}${process.env.SITE_BASE_PATH && process.env.SITE_BASE_PATH !== '/' ? '/' : ''}`;
const internal = (path = '') => `${sitePath}${path}`;

const skills = [
  {
    slug: 'ugc-hook-writer', name: 'ugc_hook_writer', title: 'UGC 开头', en: 'UGC Hooks',
    summary: '从真实使用瞬间出发，写出能拍摄的首帧、口播、字幕与产品承接。',
    english: 'Design a shootable opening from a real moment of use, with a first frame, spoken line, caption and product transition.',
    category: '短视频创意', prompt: '调用 ugc_hook_writer skill，基于当前产品，设计 3 个不同角度的 UGC 视频开头。',
    example: '一款旅行收纳袋，目标是赴美旅行的人。已有装箱视频和尺寸数据；请写 3 个前 3 秒开头。',
  },
  {
    slug: 'product-selling-point-writer', name: 'product_selling_point_writer', title: '场景化卖点表达', en: 'Product Benefits in Context',
    summary: '把产品事实、使用场景和可见收益连起来，写出准确、简洁、可验证的表达。',
    english: 'Turn documented product features into a realistic use scene, observable benefit and concise claim.',
    category: '商品表达', prompt: '调用 product_selling_point_writer skill，帮我把当前产品的卖点写成真实使用场景。',
    example: '这是产品资料和实拍素材。请找最值得先讲的卖点，写一条商品页文案和三个短视频镜头。',
  },
  {
    slug: 'fission-strategy', name: 'fission_strategy', title: '创意裂变策略', en: 'Creative Variations',
    summary: '围绕一个明确变量变化开头、结构或视听元素，让不同版本更容易比较。',
    english: 'Create short-video variations by changing one creative variable at a time.',
    category: '创意测试', prompt: '调用 fission_strategy skill，基于当前产品和素材，给我 3 个单变量裂变方向。',
    example: '这条视频的中段演示已经拍好。只改开头，给我三个可以比较的版本。',
  },
  {
    slug: 'script-review', name: 'script_review', title: '脚本优化检查', en: 'Script Review',
    summary: '检查开头、卖点、镜头和行动指引，找到表达不清或拍摄困难的地方。',
    english: 'Review the opening, benefit, shots and call to action, then suggest concrete fixes.',
    category: '脚本优化', prompt: '调用 script_review skill，检查这条脚本的钩子、卖点、可拍摄性和 CTA。',
    example: '这是现有 15 秒脚本。请指出最影响理解的一处问题，并给出可替换的口播与镜头。',
  },
];

function escapeHTML(value) {
  return value.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;').replaceAll('"', '&quot;');
}

function skillContent(name) {
  const at = source.indexOf(`Name:             "${name}"`) >= 0
    ? source.indexOf(`Name:             "${name}"`)
    : source.indexOf(`Name: "${name}"`);
  if (at < 0) throw new Error(`Built-in skill missing: ${name}`);
  const start = source.indexOf('Content: strings.TrimSpace(`', at);
  const end = source.indexOf('`)', start);
  if (start < 0 || end < 0 || end > source.indexOf('\n\t},', at)) throw new Error(`Cannot read prompt for ${name}`);
  return source.slice(start + 'Content: strings.TrimSpace(`'.length, end).trim();
}

function layout({ title, description, canonical, body, listing = false }) {
  const schema = listing
    ? { '@context': 'https://schema.org', '@type': 'CollectionPage', name: title, description, url: canonical }
    : { '@context': 'https://schema.org', '@type': 'WebPage', name: title, description, url: canonical, isPartOf: { '@type': 'WebSite', name: 'ScriptAgent', url: base } };
  return `<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>${escapeHTML(title)} · ScriptAgent</title><meta name="description" content="${escapeHTML(description)}">
<meta name="robots" content="index,follow"><link rel="canonical" href="${canonical}">
<meta property="og:type" content="article"><meta property="og:title" content="${escapeHTML(title)} · ScriptAgent"><meta property="og:description" content="${escapeHTML(description)}"><meta property="og:url" content="${canonical}"><meta property="og:image" content="${base}og-cover.jpg">
<link rel="icon" href="${internal('assets/scriptagent-mark.png')}"><link rel="stylesheet" href="${internal('skills/style.css')}">
<script type="application/ld+json">${JSON.stringify(schema).replaceAll('<', '\\u003c')}</script></head><body>
<header class="site-header"><a class="brand" href="${internal()}"><img src="${internal('assets/scriptagent-mark.png')}" alt=""><strong>ScriptAgent</strong><span>OPEN SKILLS</span></a><nav><a href="${internal()}">官网</a><a href="${internal('skills/')}">公开 Skill</a><a href="${repository}" target="_blank" rel="noopener noreferrer">GitHub ↗</a></nav></header>
${body}
<footer><span>ScriptAgent · Open-source Beta</span><a href="${repository}">查看项目源码 ↗</a></footer></body></html>`;
}

rmSync(output, { recursive: true, force: true });
mkdirSync(output, { recursive: true });
const cards = skills.map((skill, index) => `<a class="skill-card" href="${internal(`skills/${skill.slug}/`)}"><span class="card-index">0${index + 1} / ${escapeHTML(skill.category)}</span><h2>${escapeHTML(skill.title)}</h2><p>${escapeHTML(skill.summary)}</p><span class="card-link">查看方法与完整提示词 <b>↗</b></span></a>`).join('');
writeFileSync(path.join(output, 'index.html'), layout({
  title: '公开创作 Skill', description: 'ScriptAgent 开源创作 Skill：UGC 开头、场景化卖点表达、创意裂变与脚本检查。可查看完整提示词和源码。', canonical: `${base}skills/`, listing: true,
  body: `<main class="library"><p class="eyebrow">OPEN CREATIVE METHODS / 01—04</p><h1>创作方法，<em>公开分享。</em></h1><p class="lead">从真实产品资料出发，把卖点变成能拍、能说、能验证的 UGC 创意。这里公开部分内置 Skill 的完整工作流，你可以阅读、引用，也可以在开源项目里改进它们。</p><p class="english">Open creative workflows for product storytelling and UGC. Read the prompts, adapt them and contribute on GitHub.</p><div class="skill-grid">${cards}</div><aside class="note"><span>WHY OPEN?</span><p>好的创作方法值得被讨论。Skill 是工作流和判断标准，不是效果保证；实际结果仍取决于产品资料、素材和人的选择。</p></aside></main>`,
}));

for (const skill of skills) {
  const dir = path.join(output, skill.slug);
  mkdirSync(dir, { recursive: true });
  const content = skillContent(skill.name);
  const canonical = `${base}skills/${skill.slug}/`;
  const sourceURL = `${repository}/blob/main/internal/chat/service.go`;
  writeFileSync(path.join(dir, 'index.html'), layout({
    title: skill.title, description: skill.summary, canonical,
    body: `<main class="detail"><a class="back" href="${internal('skills/')}">← 所有公开 Skill</a><div class="detail-heading"><p class="eyebrow">${escapeHTML(skill.category)} / OPEN SKILL</p><h1>${escapeHTML(skill.title)}<span>${escapeHTML(skill.en)}</span></h1><p class="lead">${escapeHTML(skill.summary)}</p><p class="english">${escapeHTML(skill.english)}</p></div><div class="detail-grid"><section class="method-copy"><h2>完整工作流</h2><p>以下内容从当前开源仓库的内置 Skill 定义生成；产品更新后会随官网重新构建。</p><pre>${escapeHTML(content)}</pre></section><aside class="use-panel"><span class="panel-label">TRY IT</span><h2>从一个真实产品开始</h2><p class="panel-title">输入示例</p><blockquote>${escapeHTML(skill.example)}</blockquote><p class="panel-title">在 ScriptAgent 中调用</p><div class="invocation">${escapeHTML(skill.prompt)}</div><a class="source-link" href="${sourceURL}" target="_blank" rel="noopener noreferrer">查看 GitHub 源码 ↗</a><p class="source-note">仅公开产品内置方法；你的私有产品资料、自定义 Skill 和对话不会发布到官网。</p></aside></div><a class="next" href="${internal('skills/')}">探索其他公开 Skill <span>↗</span></a></main>`,
  }));
}

writeFileSync(path.join(output, 'style.css'), `@import url('https://fonts.googleapis.com/css2?family=DM+Sans:wght@400;500;600;700&family=Noto+Sans+SC:wght@400;500;600;700&family=Noto+Serif+SC:wght@500;600;700&display=swap');
:root{font-family:'DM Sans','Noto Sans SC',sans-serif;color:#171513;background:#fbf7f0}*{box-sizing:border-box}html{scroll-behavior:smooth}body{margin:0;min-width:320px}a{color:inherit;text-decoration:none}a:focus-visible{outline:2px solid #f0523d;outline-offset:4px}.site-header{height:78px;padding:0 clamp(24px,7vw,110px);display:flex;align-items:center;justify-content:space-between;border-bottom:1px solid #e8e0d6}.brand{display:flex;align-items:center;gap:9px}.brand img{width:25px;height:25px}.brand strong{font-size:19px;letter-spacing:-.04em}.brand span{font-size:9px;border:1px solid #bdaea1;border-radius:20px;padding:3px 7px;letter-spacing:.08em}.site-header nav{display:flex;gap:26px;font-size:12px}.site-header nav a:hover,.back:hover,.source-link:hover{color:#d8422f}.library,.detail{max-width:1360px;margin:auto;padding:clamp(74px,8vw,126px) clamp(24px,7vw,110px) 120px}.eyebrow{font-size:11px;font-weight:700;letter-spacing:.18em;color:#b15841}.library h1,.detail h1{font-family:'Noto Serif SC',serif;font-size:clamp(45px,6vw,84px);letter-spacing:-.06em;line-height:1.2;margin:22px 0 28px}.library h1 em{font-style:normal;color:#f0523d}.lead{font-size:clamp(16px,1.35vw,19px);line-height:1.8;max-width:760px;margin:0}.english{font-size:13px;color:#80766d;line-height:1.7;max-width:730px;margin:13px 0 0}.skill-grid{display:grid;grid-template-columns:repeat(2,1fr);gap:14px;margin-top:70px}.skill-card{background:#fffdf9;padding:32px;min-height:265px;display:flex;flex-direction:column;border:1px solid #e8e0d6;transition:transform .25s,border-color .25s,box-shadow .25s}.skill-card:hover{transform:translateY(-5px);border-color:#f0523d;box-shadow:0 16px 38px #7c52421c}.card-index{font-size:10px;color:#b15841;font-weight:700;letter-spacing:.15em}.skill-card h2{font-family:'Noto Serif SC',serif;font-size:30px;margin:30px 0 10px;letter-spacing:-.04em}.skill-card p{font-size:13px;line-height:1.8;color:#6b625a;margin:0;max-width:490px}.card-link{font-size:12px;font-weight:700;margin-top:auto;padding-top:30px;display:flex;justify-content:space-between;color:#d8422f}.card-link b{font-size:18px}.note{border-top:1px solid #ded3c6;margin-top:76px;padding-top:22px;display:grid;grid-template-columns:180px 1fr;gap:20px}.note span{font-size:11px;color:#b15841;font-weight:700;letter-spacing:.15em}.note p{font-size:13px;color:#6b625a;line-height:1.8;max-width:700px;margin:0}.back{font-size:12px;color:#6b625a}.detail-heading{margin:55px 0 75px}.detail h1 span{display:block;font:500 15px 'DM Sans',sans-serif;letter-spacing:.04em;color:#b15841;margin-top:12px}.detail-grid{display:grid;grid-template-columns:minmax(0,1.35fr) minmax(280px,.65fr);gap:55px;align-items:start}.method-copy h2,.use-panel h2{font-size:22px;letter-spacing:-.04em;margin:0 0 12px}.method-copy>p{font-size:13px;color:#80766d;line-height:1.7}.method-copy pre{white-space:pre-wrap;overflow-wrap:anywhere;font:13px/1.9 'Noto Sans SC',sans-serif;background:#fffdf9;padding:32px;border-left:3px solid #f0523d;margin:25px 0 0;color:#322d29}.use-panel{background:#1c1a18;color:#fbf7f0;padding:34px;position:sticky;top:24px}.panel-label{font-size:10px;color:#ff9a84;letter-spacing:.18em;font-weight:700}.use-panel h2{margin:20px 0 32px}.panel-title{font-size:10px;font-weight:700;letter-spacing:.12em;color:#b9aa9f;text-transform:uppercase;margin:25px 0 10px}.use-panel blockquote,.invocation{font-size:13px;line-height:1.8;margin:0;padding:16px 18px;background:#ffffff10}.invocation{color:#ffbaaa}.source-link{display:inline-block;background:#f0523d;padding:12px 16px;margin-top:30px;font-size:12px;font-weight:700}.source-link:hover{color:white;background:#d8422f}.source-note{color:#ab9f96;font-size:11px;line-height:1.7;margin:20px 0 0}.next{display:flex;justify-content:space-between;border-top:1px solid #ded3c6;margin-top:90px;padding:22px 0;color:#b15841;font-weight:700;font-size:13px}footer{background:#171513;color:#f6efe8;padding:32px clamp(24px,7vw,110px);display:flex;justify-content:space-between;font-size:11px}footer a{color:#ff9a84}@media(max-width:760px){.site-header{height:68px;padding:0 24px}.site-header nav{gap:12px}.site-header nav a:first-child{display:none}.brand span{display:none}.library,.detail{padding:75px 24px 90px}.skill-grid{grid-template-columns:1fr;margin-top:48px}.skill-card{min-height:235px;padding:25px}.note{grid-template-columns:1fr}.detail-heading{margin:43px 0 55px}.detail-grid{grid-template-columns:1fr;gap:30px}.use-panel{position:static}.method-copy pre{padding:23px;font-size:12px}footer{gap:25px}}@media(prefers-reduced-motion:reduce){html{scroll-behavior:auto}*,*:before,*:after{transition:none!important}}`);
writeFileSync(path.join(website, 'public/sitemap.xml'), `<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">\n${[base, `${base}skills/`, ...skills.map(skill => `${base}skills/${skill.slug}/`)].map(url => `  <url><loc>${url}</loc></url>`).join('\n')}\n</urlset>\n`);
writeFileSync(path.join(website, 'public/robots.txt'), `User-agent: *\nAllow: /\nSitemap: ${base}sitemap.xml\n`);
console.log(`Generated ${skills.length} public Skill pages from built-in definitions.`);
