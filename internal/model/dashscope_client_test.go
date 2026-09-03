package model

import (
	"net/http"
	"testing"
)

func TestFriendlyDashScopeError(t *testing.T) {
	tests := []struct {
		name string
		code string
		want string
	}{
		{name: "arrearage", code: "Arrearage", want: "模型账户余额或可用额度不足，请充值或确认免费额度已生效后重试"},
		{name: "invalid key", code: "InvalidApiKey", want: "模型 API Key 无效，请在设置中重新配置"},
		{name: "access denied", code: "AccessDenied", want: "当前模型账户没有调用权限，请检查模型开通状态"},
		{name: "throttling", code: "Throttling", want: "模型请求过于频繁，请稍后重试"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := friendlyDashScopeError(http.StatusBadRequest, []byte(`{"code":"`+tt.code+`","message":"provider detail"}`))
			if err.Error() != tt.want {
				t.Fatalf("got %q, want %q", err, tt.want)
			}
		})
	}
}

func TestFriendlyDashScopeErrorDoesNotExposeProviderBody(t *testing.T) {
	err := friendlyDashScopeError(http.StatusBadRequest, []byte(`{"code":"Unknown","message":"sensitive provider detail"}`))
	if err.Error() != "模型请求失败，请检查模型配置后重试" {
		t.Fatalf("unexpected error: %q", err)
	}
}
