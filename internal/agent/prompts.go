package agent

import (
	"fmt"
	"strings"

	"github.com/tian1363/scriptagent/internal/jobs"
)

func videoAnalysisPrompt(job jobs.Job, productMD string) string {
	return fmt.Sprintf(`你是一个专业短视频分镜分析 Agent。请逐帧分析用户上传的视频，输出非常详细的分镜表。

用户设置行业：%s
裂变脚本数量：%d
用户补充要求：%s

产品信息 Markdown：
%s

## 1. 行业判断

首先判断视频行业类型，只能在以下两类中选择：

- 游戏
- 电商

如果用户设置不是 auto，应优先参考用户设置，但仍需要说明判断依据。

## 2. 分析原则

请只填写视频中能直接观察到的内容。

- 无法判断的内容统一填写 "-"
- 产品卖点允许基于画面和产品 Markdown 进行合理推断
- 画面描述必须非常详细，接近逐帧分析
- 分镜时间粒度建议为 "00:00-00:01"，如画面变化密集，可进一步拆细
- 同一道具、场景、角色在全表中必须统一命名
- 首次出现某个角色、道具、场景时，确定唯一名称
- 后续所有分镜必须严格沿用该名称，不得更换说法
- 画面描述需要包含物理层词和时间层次
- 描述时注意画面元素的布局、方位、动作、互动关系和镜头运用

画面描述句式可参考：

参考方位/镜头 + 主体 + 动作 + 道具 + 互动关系

## 3. 通用分镜表字段

请以 Markdown 表格输出分镜表。

每个分镜至少包含以下列：

| 时间段 | 行业类型 | 产品卖点 | 画面描述（逐帧） | 动作描述 | 视频信息 | 人物角色 | 道具场景 | 旁白/对话 | 景别 | 镜头动机描述 | 叙事节奏描述 | 首帧画面描述 | 音效 | BGM |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|

## 4. 游戏类追加字段

如果判断为游戏类视频，请在表格中追加以下列：

| 游戏素材类型 | 游戏 UI 描述 |
|---|---|

字段说明：

- 游戏素材类型：实录 / 合成 / 实录+合成 / 无法判断
- 游戏 UI 描述：描述 UI 布局、元素、色彩、按钮、血条、金币、等级、技能图标、弹窗、特效文字等

## 5. 电商类追加字段

如果判断为电商类视频，请在表格中追加以下列：

| 产品素材类型 | 产品展示描述 |
|---|---|

字段说明：

- 产品素材类型：实拍 / 3D / 图文贴片 / 使用演示 / 混合 / 无法判断
- 产品展示描述：描述产品外观、卖点展示方式、贴片样式、使用方式、前后对比、包装、细节特写等

## 6. 输出要求

请按以下顺序输出：

### A. 行业判断

简要说明该视频属于游戏还是电商，以及判断依据。

### B. 命名统一表

列出全片中需要统一称呼的角色、道具、场景、产品、UI 元素。

格式：

| 类型 | 统一名称 | 首次出现时间 | 描述 |
|---|---|---|---|

### C. 详细分镜表

以 Markdown 表格输出完整分镜分析。

重要约束：

- 分镜表必须是 Markdown 表格
- 每一行对应一个明确时间段
- 不要省略短时间内的重要画面变化
- 不要把多个明显不同镜头合并成一行
- 所有无法判断的字段填写 "-"

### D. 核心亮点总结

最后给出视频作品中：

- 最能抓住观众眼球的画面亮点
- 最抓耳的声音/旁白/BGM亮点
- 最令人难忘的创意点
- 最适合复刻的脚本结构
- 复刻时必须保留的关键元素
- 可裂变替换的元素

请重点服务于后续生成复刻脚本。`, job.Industry, job.FissionCount, job.Requirement, productMD)
}

func replicaScriptPrompt(job jobs.Job, productMD, analysisMarkdown string) string {
	return fmt.Sprintf(`你是 CreatiBI 复刻分镜脚本生成 Agent。请基于产品 Markdown 和视频理解结果，生成 1 条复刻脚本。

产品 Markdown：
%s

用户补充要求：
%s

视频理解结果：
%s

复刻生成原则：

- 复刻脚本复刻参考视频的结构、节奏、镜头功能，不复制原视频独有台词、品牌、人物肖像或版权音乐。
- 每个分镜必须继承视频理解中的时间段、镜头动机、叙事节奏和核心功能。
- 如果需要改写台词，应保留原镜头承担的功能：hook、twist、selling_point、proof、cta 等。
- 所有脚本必须适合写入 CreatiBI 分镜结构。
- 所有 storyboards 都必须包含 scene_index、time_range、visual、action、voiceover、subtitle、shot_size、camera_intent、props_scene、audio、purpose。
- 输出必须是严格 JSON，不要 Markdown，不要代码块，不要解释。

JSON Schema：

{
  "replica_script": {
    "title": "脚本标题",
    "script_type": "replica",
    "industry": "game 或 ecommerce",
    "duration_seconds": 25,
    "source_summary": "复刻结构说明",
    "storyboards": [
      {
        "scene_index": 1,
        "time_range": "00:00-00:03",
        "visual": "画面描述，写清主体、方位、构图、光线、UI/贴片/环境细节",
        "action": "动作描述，写清人物/产品/镜头运动/互动关系",
        "voiceover": "旁白/对话文案",
        "subtitle": "屏幕字幕或贴片文字",
        "shot_size": "近景",
        "camera_intent": "镜头动机",
        "props_scene": "道具场景",
        "audio": "音效/BGM",
        "purpose": "hook"
      }
    ],
    "metadata": {
      "parent_script_id": "",
      "fission_dimension": "",
      "kept_elements": ["保留元素"],
      "changed_elements": ["替换元素"]
    }
  }
}`, productMD, job.Requirement, analysisMarkdown)
}

func fissionScriptPrompt(job jobs.Job, productMD, analysisMarkdown, replicaScriptJSON string) string {
	return fmt.Sprintf(`你是 CreatiBI 裂变分镜脚本生成 Agent。请基于“复刻脚本”生成 %d 条裂变脚本。

产品 Markdown：
%s

用户补充要求：
%s

视频理解结果：
%s

复刻脚本 JSON：
%s

用户选择的裂变方向：
%s

裂变生成原则：

- 裂变脚本必须以复刻脚本为母版，先继承每个分镜的 scene_index、time_range、镜头功能、叙事节奏和转场顺序。
- 每条裂变只选择一个清晰裂变维度；如果用户选择了裂变方向，必须只从“用户选择的裂变方向”中选择。
- metadata.fission_dimension 必须使用“层级-维度”格式，例如“视听层-换BGM”。
- 如果需要生成多条裂变，应优先覆盖不同层级；不要连续多条都只改同一种元素。
- 每条裂变的 storyboards 数量和复刻脚本保持一致；每个分镜的 time_range 尽量保持一致。
- 每个裂变分镜必须保留原分镜的 purpose，但改写 visual、action、voiceover、subtitle、props_scene、audio 中与裂变维度相关的内容。
- 裂变应适合直接写入 CreatiBI 分镜结构，不要输出抽象策略，要输出可拍/可剪/可生成的视频分镜。
- 所有 storyboards 都必须包含 scene_index、time_range、visual、action、voiceover、subtitle、shot_size、camera_intent、props_scene、audio、purpose。
- fission_scripts 数量必须严格等于 %d。
- 输出必须是严格 JSON，不要 Markdown，不要代码块，不要解释。

允许裂变维度：

1. 视听层
   - 换BGM：保留画面结构，替换音乐风格、情绪走向或卡点方式，写入 audio。
   - 换音效：保留镜头和台词，替换关键动作、UI、转场、点击、爆点音效，写入 audio。
   - 换色调/滤镜：保留主体与动作，改变 visual 中的色彩、光影、滤镜、氛围描述。
   - 换字幕&花字：保留旁白核心，改写 subtitle 的花字风格、强调词、屏幕文字层级。
   - 换画幅：保留内容结构，改写 visual 中的构图、主体位置、留白、贴片适配方式。
   - 换配音(语速/声线)：保留语义，改写 voiceover 的语速、声线、情绪、口吻。

2. 结构层
   - 换开头钩子：重点改写前 1-2 个分镜的 visual/action/voiceover/subtitle，但保留后续链路。
   - 换CTA：重点改写最后 1-2 个分镜的 voiceover/subtitle/action，强化不同转化动作。
   - 时长压缩/拉伸：调整 time_range 和 duration_seconds，压缩或拉伸节奏，同时保留分镜顺序。
   - 变速·节奏调整：保留时长大体不变，改变 action 和 camera_intent 中的速度、卡点、停顿。
   - 换首帧/封面：重点改写第 1 个分镜的 visual、action、subtitle，让首帧更强。
   - 同素材高光重剪：不换主体素材，重排高光重点和镜头强调，改写 camera_intent/action。

3. 元素层
   - 换局部角色/群演：保留场景与叙事，替换 visual/action 中的局部角色、群演或人物关系。
   - 换局部场景贴片：保留主体动作，替换 visual/props_scene 中的局部背景、贴片、环境元素。
   - 换局部道具/UI：保留镜头结构，替换 props_scene/visual 中的局部道具、UI 元素、按钮、弹窗。
   - 字幕语言本地化：保留语义，按目标地区改写 subtitle 和必要的 voiceover 表达。

JSON Schema：

{
  "fission_scripts": [
    {
      "title": "裂变脚本标题",
      "script_type": "fission",
      "industry": "game 或 ecommerce",
      "duration_seconds": 25,
      "source_summary": "说明该裂变相对复刻脚本改了什么、保留了什么",
      "storyboards": [
        {
          "scene_index": 1,
          "time_range": "00:00-00:03",
          "visual": "画面描述，写清主体、方位、构图、光线、UI/贴片/环境细节",
          "action": "动作描述，写清人物/产品/镜头运动/互动关系",
          "voiceover": "旁白/对话文案",
          "subtitle": "屏幕字幕或贴片文字",
          "shot_size": "近景",
          "camera_intent": "镜头动机",
          "props_scene": "道具场景",
          "audio": "音效/BGM",
          "purpose": "hook"
        }
      ],
      "metadata": {
        "parent_script_id": "replica",
        "fission_dimension": "视听层-换BGM",
        "kept_elements": ["保留元素"],
        "changed_elements": ["替换元素"]
      }
    }
  ]
}`, job.FissionCount, productMD, job.Requirement, analysisMarkdown, replicaScriptJSON, selectedFissionDirections(job.FissionDirections), job.FissionCount)
}

func selectedFissionDirections(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "未选择，允许从下方全部裂变维度中自动选择。"
	}
	return raw
}
