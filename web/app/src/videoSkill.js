export function prepareVideoDraft(value = '') {
  const text = value.trim();
  const durationMatch = text.match(/(?:总时长|时长|duration)\s*[:：=]?\s*(\d+)\s*(?:秒|s)?/i) || text.match(/(\d+)\s*(?:秒|seconds?\b|s\b)/i);
  let duration = Math.max(2, Math.min(30, Number(durationMatch?.[1] || 5)));
  const ratio = text.match(/(?:16\s*[:：]\s*9|9\s*[:：]\s*16|1\s*[:：]\s*1)/)?.[0].replace(/\s/g, '').replace('：', ':') || (/横屏/.test(text) ? '16:9' : /方形/.test(text) ? '1:1' : '9:16');
  const resolution = text.match(/\b(480|720|1080)p\b/i)?.[0].toUpperCase() || '720P';
  const soundEnabled = !/(?:^|\n)\s*(?:[-*]\s*)?(?:\*\*)?(?:声音|音频|生成声音|audio|sound_enabled)(?:\*\*)?\s*[:：=]\s*(?:静音|无声|关闭|不生成|false|off|0)(?=\s|$|[，,。；;])/im.test(text)
    && !/(?:^|\n)\s*(?:请)?(?:关闭声音|不生成声音|生成(?:一个|一条|一段)?(?:无声|静音)视频|(?:整条|整个|成片)视频(?:设为|设置为|改为)?(?:静音|无声))(?=$|[\s。，,；;])/u.test(text);
  const section = text.match(/(?:^|\n)\s*(?:#{1,6}\s*)?(?:\*\*)?(?:视频生成提示词|视频提示词|生成提示词|Video Prompt|Prompt)(?:\*\*)?\s*[:：]?[^\S\n]*\n?([\s\S]*)/i);
  let prompt = section ? section[1] : text;
  if (!section) {
    const storyboard = storyboardVideoPrompt(text);
    if (storyboard) {
      prompt = storyboard.prompt;
      duration = storyboard.duration ? Math.max(duration, storyboard.duration) : duration;
    } else if (/(?:分镜|脚本|素材清单|合规声明)/.test(text) && /\|[^\n]+\|/.test(text)) {
      prompt = '';
    }
  }
  prompt = prompt.split(/\n\s*(?:#{1,6}\s*)?(?:\*\*)?(?:说明|解释|注意事项|使用建议|参数设置|生成参数|视频参数|下一步|为什么)(?:\*\*)?\s*[:：\n]/)[0];
  const fenced = prompt.match(/```(?:text|plaintext|prompt)?\s*\n([\s\S]*?)```/i);
  if (fenced) prompt = fenced[1];
  prompt = prompt.split('\n').filter(line => !/^\s*(?:[-*]\s*)?(?:时长|总时长|分辨率|画幅|比例|声音|duration|resolution|ratio|audio)\s*[:：=]/i.test(line.replace(/\*\*/g, '')) && !/^\s*(?:好的[，,！!。]|以下是|下面是|你可以|如果你|如需|希望这|点击.*生成)/.test(line)).join('\n').trim();
  return { prompt, duration, ratio, resolution, soundEnabled };
}

function storyboardVideoPrompt(text) {
  const lines = text.split('\n');
  const headerIndex = lines.findIndex(line => /^\s*\|/.test(line) && /时间/.test(line) && /画面|镜头|分镜/.test(line));
  if (headerIndex < 0) return null;
  const cells = line => line.trim().replace(/^\|/, '').replace(/\|$/, '').split('|').map(cell => cell.trim().replace(/<br\s*\/?\s*>/gi, '，').replace(/\*\*/g, ''));
  const headers = cells(lines[headerIndex]);
  const index = pattern => headers.findIndex(header => pattern.test(header));
  const timeIndex = index(/时间/);
  const visualIndex = index(/画面|镜头|分镜/);
  const voiceIndex = index(/口播|旁白|台词/);
  const subtitleIndex = index(/字幕/);
  const soundIndex = index(/声音|剪辑|音效/);
  if (timeIndex < 0 || visualIndex < 0) return null;
  const scenes = [];
  let lastEnd = 0;
  for (const line of lines.slice(headerIndex + 1)) {
    if (!/^\s*\|/.test(line)) break;
    if (/^\s*\|[\s:|-]+\|?\s*$/.test(line)) continue;
    const row = cells(line);
    const time = row[timeIndex] || '';
    const visual = row[visualIndex] || '';
    if (!visual || !/\d/.test(time)) continue;
    const parts = [`${time}：${visual}`];
    if (voiceIndex >= 0 && row[voiceIndex] && !/^[-—无]$/.test(row[voiceIndex])) parts.push(`口播：${row[voiceIndex]}`);
    if (subtitleIndex >= 0 && row[subtitleIndex] && !/^[-—无]$/.test(row[subtitleIndex])) parts.push(`字幕：${row[subtitleIndex]}`);
    if (soundIndex >= 0 && row[soundIndex] && !/^[-—无]$/.test(row[soundIndex])) parts.push(`声音与剪辑：${row[soundIndex]}`);
    scenes.push(parts.join('；'));
    const lastTime = time.match(/(\d{1,2}:\d{2}|\d+(?:\.\d+)?)(?!.*\d)/)?.[1];
    if (lastTime) {
      const end = lastTime.includes(':') ? Number(lastTime.split(':')[0]) * 60 + Number(lastTime.split(':')[1]) : Number(lastTime);
      lastEnd = Math.max(lastEnd, end);
    }
  }
  if (!scenes.length) return null;
  return {prompt: scenes.join('\n'), duration: lastEnd ? Math.max(2, Math.min(30, Math.ceil(lastEnd))) : null};
}

export function hasVideoScript(value = '') {
  return /(?:^|\n)\s*(?:#{1,6}\s*)?(?:视频生成提示词|视频提示词)\s*[:：\n]/.test(value) || /\|[^\n]*时间[^\n]*\|[^\n]*(?:画面|镜头|分镜)/.test(value);
}

export function videoReferenceIDs(value = '') {
  return [...new Set([...value.matchAll(/\/api\/assets\/([a-zA-Z0-9_-]+)\/file/g)].map(match => match[1]))];
}

export function parseVideoSkillRequest(value = "") {
  const text = value.trim();
  // A request for prompt text belongs in chat, even when it names the video skill.
  if (!/^\/(?:生成视频|generate-video)(?=$|[\s，,:：。])/u.test(text) && /(?:提示词|prompt|教程|方案|用法|说明)/i.test(text)) return null;
  const explicit = text.match(/^(?:\/(?:生成视频|generate-video)|(?:调用|使用)\s*`?(?:generate-video|generate_video|生成视频)`?\s*(?:skill|技能)?)(?=$|[\s，,:：。])[\s，,:：。]*(.*)$/isu);
  if (explicit) return { prompt: explicit[1].trim() };
  if (/不要|别生成|不生成|怎么|如何|能否|能不能|可以吗|能做什么|是什么/.test(text)) return null;
  if (/^(?:请)?(?:帮我|给我)?(?:直接|立即|开始)?生成(?:完整|全部|整段|整条)(?:的)?视频[。！!]?$/u.test(text)) return { prompt: '' };
  const previous = /^(?:请)?(?:把|将|用|按|基于)?(?:上面|上文|刚才|这段|这个)(?:的)?(?:脚本|内容|提示词)?(?:直接)?(?:生成|制作|做成)(?:一个|一段)?视频[。！!]?$/u;
  if (previous.test(text) || /^(?:请)?(?:直接|立即|开始)?(?:给我|帮我)?生成(?:这个|这段|一段|一个)?视频[。！!]?$/u.test(text)) return { prompt: "" };
  if (/^(?:请)?(?:帮我|给我)?(?:生成|制作)(?:一条|一段|一个)?.{0,120}视频/su.test(text) && !/提示词|教程|方案/.test(text)) return { prompt: text };
  return null;
}
