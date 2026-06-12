package videoprompt

import (
	"strings"
	"testing"

	"github.com/tian1363/scriptagent/internal/jobs"
)

func TestGenerateFromJobCreatesSeedancePrompts(t *testing.T) {
	job := jobs.Job{
		Title:         "测试任务",
		ProductMDName: "product.md",
		ReplicaScriptJSON: `{
			"replica_script": {
				"metadata": {"title": "复刻脚本", "industry": "game"},
				"storyboards": [
					{
						"time_range": "00:00-00:02",
						"visual": "手机屏幕中展示游戏主城，中心出现英雄角色",
						"action": "英雄向前挥剑",
						"shot_size": "近景",
						"camera_intent": "快速建立爽感",
						"voiceover": "开局就送强力英雄",
						"subtitle": "上线即领"
					}
				]
			}
		}`,
		FissionScriptsJSON: `{
			"fission_scripts": [
				{
					"metadata": {"title": "换开头钩子", "fission_dimension": "结构层-换开头钩子"},
					"storyboards": [
						{"time_range": "00:00-00:02", "visual": "失败战斗画面突然反转", "action": "玩家点击升级按钮"}
					]
				}
			]
		}`,
	}

	content, err := GenerateFromJob(job, "all")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"Seedance 视频生成提示词", "复刻脚本", "结构层-换开头钩子", "正向提示词", "负向提示词", "上线即领"} {
		if !strings.Contains(content, expected) {
			t.Fatalf("expected generated prompt to contain %q\n%s", expected, content)
		}
	}
}

func TestGenerateFromJobRequiresStoryboards(t *testing.T) {
	_, err := GenerateFromJob(jobs.Job{}, "all")
	if err == nil {
		t.Fatal("expected error for missing scripts")
	}
}
