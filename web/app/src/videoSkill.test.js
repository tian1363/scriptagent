import test from 'node:test';
import assert from 'node:assert/strict';
import {parseVideoSkillRequest} from './videoSkill.js';
import {prepareVideoDraft} from './videoSkill.js';
test('extracts only prompt body and fills all requested parameters',()=>{
 const result = prepareVideoDraft('好的，以下是提示词。\n## 视频生成提示词\n```text\n海边日落，模特拿起产品。\n镜头缓慢推进。\n```\n## 视频参数\n时长：7秒\n画幅：16:9\n分辨率：1080P\n声音：静音\n## 说明\n你可以复制使用。');
 assert.deepEqual(result,{prompt:'海边日落，模特拿起产品。\n镜头缓慢推进。',duration:7,ratio:'16:9',resolution:'1080P',soundEnabled:false});
});
test('preserves plain scene copy and provides valid defaults',()=>{
 assert.deepEqual(prepareVideoDraft('人物走进厨房，拿起咖啡。'),{prompt:'人物走进厨房，拿起咖啡。',duration:5,ratio:'9:16',resolution:'720P',soundEnabled:true});
 assert.equal(prepareVideoDraft('时长：60秒\n画幅：横屏\n一个产品特写').duration,30);
});
test('video skill triggers explicit commands and conversational video requests',()=>{
 for(const text of ['生成完整视频','生成全部视频','请帮我生成完整视频']) assert.deepEqual(parseVideoSkillRequest(text),{prompt:''});
 for(const text of ['/生成视频','/generate-video','调用 generate-video skill','把上面的脚本生成视频','按上文生成视频','生成视频']) assert.deepEqual(parseVideoSkillRequest(text),{prompt:''});
 assert.deepEqual(parseVideoSkillRequest('/生成视频 日落海边，5秒'),{prompt:'日落海边，5秒'});
 assert.deepEqual(parseVideoSkillRequest('帮我生成一个北美模特的视频'),{prompt:'帮我生成一个北美模特的视频'});
});
test('questions, negation and prompt writing do not submit video requests',()=>{
 for(const text of ['怎么生成视频','不要生成视频','生成一个视频提示词','帮我写视频方案','这个技能有什么用','/generate-video-prompt']) assert.equal(parseVideoSkillRequest(text),null,text);
});
