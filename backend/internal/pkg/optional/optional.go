package optional

import "encoding/json"

// Optional 是三态 JSON 字段：未出现、显式 null、具体值。
// Set=false 表示未出现；Set=true 且 Null=true 表示显式 null；
// 其余情况 Value 保存具体值。空字符串是具体值，不是 null。
type Optional[T any] struct {
	Set   bool
	Null  bool
	Value T
}

// Ptr 在字段出现且不是显式 null 时返回具体值的指针；否则返回 nil。
// 该指针是副本，调用方可以安全保存。
func Ptr[T any](o Optional[T]) *T {
	if !o.Set || o.Null {
		return nil
	}
	v := o.Value
	return &v
}

// UnmarshalJSON 记录字段是否出现以及是否为显式 null。
func (o *Optional[T]) UnmarshalJSON(data []byte) error {
	o.Set = true
	if string(data) == "null" {
		o.Null = true
		var zero T
		o.Value = zero
		return nil
	}
	o.Null = false
	return json.Unmarshal(data, &o.Value)
}
