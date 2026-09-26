import test from 'node:test';
import assert from 'node:assert/strict';
import { chatTimeline } from './chatTimeline.js';

test('completed videos remain before later messages and appear once', () => {
  const messages = [{id:'before',created_at:'2026-09-06T10:00:00Z'}, {id:'after',created_at:'2026-09-06T11:00:00Z'}];
  const video = {id:'video',created_at:'2026-09-06T10:30:00Z',updated_at:'2026-09-06T12:00:00Z',status:'completed'};
  const result = chatTimeline(messages, [video]);
  assert.deepEqual(result.map(e => e.item.id), ['before','video','after']);
  assert.equal(result[2].messageIndex, 1);
  assert.deepEqual(chatTimeline(messages, [{...video,status:'running'}]).map(e => e.item.id), ['before','video','after']);
});
test('videos sort chronologically even when API returns newest first', () => {
  assert.deepEqual(chatTimeline([], [{id:'new',created_at:'2026-09-06'}, {id:'old',created_at:'2026-09-05'}]).map(e => e.item.id), ['old','new']);
});
