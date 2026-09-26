// Keep generated artifacts at their creation time, independent of polling updates.
export function chatTimeline(messages, videos) {
  const time = value => {
    const parsed = Date.parse(value);
    return Number.isFinite(parsed) ? parsed : 0;
  };
  return [
    ...messages.map((message, index) => ({ type: 'message', item: message, messageIndex: index })),
    ...videos.map(video => ({ type: 'video', item: video })),
  ].sort((a, b) => time(a.item.created_at) - time(b.item.created_at));
}
