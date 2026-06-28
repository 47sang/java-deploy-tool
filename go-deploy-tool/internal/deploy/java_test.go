package deploy

import "testing"

// TestContains 覆盖 deploy 包内用于模块过滤的 contains 辅助函数。
func TestContains(t *testing.T) {
	t.Parallel()
	if !contains([]string{"admin", "client", "websocket"}, "client") {
		t.Error("应包含 client")
	}
	if contains([]string{"admin"}, "client") {
		t.Error("不应包含 client")
	}
	if contains([]string{}, "x") {
		t.Error("空切片不应包含任何元素")
	}
}
