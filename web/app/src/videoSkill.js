export function prepareVideoDraft(value = '') {
  const text = value.trim();
  const durationMatch = text.match(/(?:总时长|时长|duration)\s*[:：=]?\s*(\d+)\s*(?:秒|s)?/i) || text.match(/(\d+)\s*(?:秒|seconds?\b|s\b)/i);
  const duration = Math.max(2, Math.min(30, Number(durationMatch?.[1] || 5)));
  const ratio = text.match(/(?:16\s*[:：]\s*9|9\s*[:：]\s*16|1\s*[:：]\s*1)/)?.[0].replace(/\s/g, '').replace('：', ':') || (/横屏/.test(text) ? '16:9' : /方形/.test(text) ? '1:1' : '9:16');
  const resolution = text.match(/\b(480|720|1080)p\b/i)?.[0].toUpperCase() || '720P';
  const soundEnabled = !/(?:静音|无声|不生成声音|关闭声音|sound_enabled\s*[:=]\s*false|audio\s*[:=]\s*false)/i.test(text);
  const section = text.match(/(?:^|\n)\s*(?:#{1,6}\s*)?(?:\*\*)?(?:视频生成提示词|视频提示词|生成提示词|Video Prompt|Prompt)(?:\*\*)?\s*[:：]?[^\S\n]*\n?([\s\S]*)/i);
  let prompt = section ? section[1] : text;
  prompt = prompt.split(/\n\s*(?:#{1,6}\s*)?(?:\*\*)?(?:说明|解释|注意事项|使用建议|参数设置|生成参数|视频参数|下一步|为什么)(?:\*\*)?\s*[:：\n]/)[0];
  const fenced = prompt.match(/```(?:text|plaintext|prompt)?\s*\n([\s\S]*?)```/i);
  if (fenced) prompt = fenced[1];
  prompt = prompt.split('\n').filter(line => !/^\s*(?:[-*]\s*)?(?:时长|总时长|分辨率|画幅|比例|声音|duration|resolution|ratio|audio)\s*[:：=]/i.test(line.replace(/\*\*/g, '')) && !/^\s*(?:好的[，,！!。]|以下是|下面是|你可以|如果你|如需|希望这|点击.*生成)/.test(line)).join('\n').trim();
  return { prompt, duration, ratio, resolution, soundEnabled };
}

export function parseVideoSkillRequest(value = "") {
  const text = value.trim();
  const explicit = text.match(/^(?:\/(?:生成视频|generate-video)|(?:调用|使用)\s*(?:generate-video|generate_video|生成视频)\s*(?:skill|技能)?)(?=$|[\s，,:：。])[\s，,:：。]*(.*)$/isu);
  if (explicit) return { prompt: explicit[1].trim() };
  if (/不要|别生成|不生成|怎么|如何|能否|能不能|可以吗|能做什么|是什么/.test(text)) return null;
  if (/^(?:请)?(?:帮我|给我)?(?:直接|立即|开始)?生成(?:完整|全部|整段|整条)(?:的)?视频[。！!]?$/u.test(text)) return { prompt: '' };
  const previous = /^(?:请)?(?:把|将|用|按|基于)?(?:上面|上文|刚才|这段|这个)(?:的)?(?:脚本|内容|提示词)?(?:直接)?(?:生成|制作|做成)(?:一个|一段)?视频[。！!]?$/u;
  if (previous.test(text) || /^(?:请)?(?:直接|立即|开始)?(?:给我|帮我)?生成(?:这个|这段|一段|一个)?视频[。！!]?$/u.test(text)) return { prompt: "" };
  if (/^(?:请)?(?:帮我|给我)?(?:生成|制作)(?:一条|一段|一个)?.{0,120}视频/su.test(text) && !/提示词|教程|方案/.test(text)) return { prompt: text };
  return null;
}
