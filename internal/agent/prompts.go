package agent

import (
	"fmt"

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

func scriptGenerationPrompt(job jobs.Job, productMD, analysisMarkdown string) string {
	return fmt.Sprintf(`你是 CreatiBI 分镜脚本生成 Agent。请基于产品 Markdown 和视频理解结果，生成 1 条复刻脚本和 %d 条裂变脚本。

产品 Markdown：
%s

用户补充要求：
%s

视频理解结果：
%s

生成原则：

- 复刻脚本复刻参考视频的结构、节奏、镜头功能，不复制原视频独有台词、品牌、人物肖像或版权音乐。
- 裂变脚本保留复刻结构的核心链路，但分别改变钩子、卖点、场景、人群、节奏、CTA 或表现形式。
- 所有脚本必须适合写入 CreatiBI 分镜结构。
- 所有 storyboards 都必须包含 time_range、visual、action、voiceover、subtitle、shot_size、camera_intent、props_scene、audio、purpose。
- fission_scripts 数量必须严格等于 %d。
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
        "visual": "画面描述",
        "action": "动作描述",
        "voiceover": "旁白",
        "subtitle": "字幕",
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
  },
  "fission_scripts": [
    {
      "title": "裂变脚本标题",
      "script_type": "fission",
      "industry": "game 或 ecommerce",
      "duration_seconds": 25,
      "source_summary": "裂变说明",
      "storyboards": [],
      "metadata": {
        "parent_script_id": "replica",
        "fission_dimension": "钩子裂变",
        "kept_elements": ["保留元素"],
        "changed_elements": ["替换元素"]
      }
    }
  ]
}`, job.FissionCount, productMD, job.Requirement, analysisMarkdown, job.FissionCount)
}
