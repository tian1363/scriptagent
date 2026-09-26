import test from 'node:test';
import assert from 'node:assert/strict';
import {visibleAssistantContent} from './assistantContent.js';
test('unwraps final answers including fenced JSON', () => {
  assert.equal(visibleAssistantContent('```json\n{"type":"final","reason":"internal","answer":"第一行\\n第二行"}\n```'), '第一行\n第二行');
});
test('recovers model output with unescaped quotes and raw newlines', () => {
  assert.equal(visibleAssistantContent('{"type":"final","reason":"internal","answer":"首帧展示"摇一摇"。\\n第二行\n第三行"}'), '首帧展示"摇一摇"。\n第二行\n第三行');
});
test('streaming metadata stays hidden and ordinary JSON is preserved', () => {
  assert.equal(visibleAssistantContent('{"type":"final","reason":"internal'), '');
  assert.equal(visibleAssistantContent('{"type":"final","answer":"正在写\\n下一行'), '正在写\n下一行');
  assert.equal(visibleAssistantContent('{"duration":3}'), '{"duration":3}');
});
