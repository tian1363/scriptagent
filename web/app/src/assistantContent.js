// Only unwrap the agent's final-answer envelope; ordinary JSON remains content.
export function visibleAssistantContent(content = '') {
  let value = String(content).trim();
  const fenced = value.match(/^```(?:json)?\s*\n([\s\S]*?)\n```$/i);
  const candidate = fenced ? fenced[1].trim() : value;
  if (!candidate.startsWith('{')) return value;
  try {
    const parsed = JSON.parse(candidate);
    if (typeof parsed.answer === 'string') return parsed.answer;
  } catch { /* Models sometimes emit raw quotes or newlines inside answer. */ }
  if (!/^\{\s*"type"\s*:\s*"final"/.test(candidate)) return value;
  const answer = candidate.match(/"answer"\s*:\s*"([\s\S]*)/);
  if (!answer) return ''; // Never show internal metadata during streaming.
  value = answer[1].replace(/"\s*}\s*$/, '');
  return value.replace(/\\(u[0-9a-fA-F]{4}|["\\/nrtbf])/g, (_, escape) => {
    if (escape.startsWith('u')) return String.fromCharCode(parseInt(escape.slice(1), 16));
    return ({n:'\n',r:'\r',t:'\t',b:'\b',f:'\f','"':'"','\\':'\\','/':'/'})[escape];
  }).replace(/\\$/, '').trim();
}
