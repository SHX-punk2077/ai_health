package optional_test

import (
	"encoding/json"
	"testing"

	"ai_health/internal/pkg/optional"
)

func TestOptionalUnmarshalTriState(t *testing.T) {
	type payload struct {
		Name  optional.Optional[string] `json:"name"`
		Count optional.Optional[uint16] `json:"count"`
	}

	var absent payload
	if err := json.Unmarshal([]byte(`{}`), &absent); err != nil {
		t.Fatalf("解析空对象失败: %v", err)
	}
	if absent.Name.Set || absent.Count.Set {
		t.Fatalf("未出现字段应为 Set=false: %+v", absent)
	}

	var null payload
	if err := json.Unmarshal([]byte(`{"name":null,"count":null}`), &null); err != nil {
		t.Fatalf("解析 null 失败: %v", err)
	}
	if !null.Name.Set || !null.Name.Null || !null.Count.Set || !null.Count.Null {
		t.Fatalf("显式 null 应保留 Null 标记: %+v", null)
	}

	var value payload
	if err := json.Unmarshal([]byte(`{"name":"","count":0}`), &value); err != nil {
		t.Fatalf("解析具体值失败: %v", err)
	}
	if !value.Name.Set || value.Name.Null || value.Name.Value != "" {
		t.Fatalf("空串应是具体值而非 null: %+v", value.Name)
	}
	if !value.Count.Set || value.Count.Null || value.Count.Value != 0 {
		t.Fatalf("0 应是具体值而非 null: %+v", value.Count)
	}

	var invalid payload
	if err := json.Unmarshal([]byte(`{"count":-1}`), &invalid); err == nil {
		t.Fatal("负数写入 uint16 应返回错误")
	}
}
