package entity_test

import (
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// 本文件校验类型契约，防止后续把 DECIMAL 改成浮点、或给未设计软删除的表误加 DeletedAt。

func TestDecimalFieldsUseDecimalType(t *testing.T) {
	decimalType := reflect.TypeOf(decimal.Decimal{})
	decimalPtrType := reflect.TypeOf(&decimal.Decimal{})
	checked := 0

	for _, model := range allSchemaModels {
		parsed, err := schema.Parse(model, &sync.Map{}, schema.NamingStrategy{})
		if err != nil {
			t.Fatalf("GORM 解析 %T 失败: %v", model, err)
		}
		for dbName, field := range parsed.FieldsByDBName {
			if !strings.HasPrefix(strings.ToLower(string(field.DataType)), "decimal") {
				continue
			}
			checked++
			if field.FieldType != decimalType && field.FieldType != decimalPtrType {
				t.Errorf("%s.%s 的 Go 类型 = %v, want decimal.Decimal 或 *decimal.Decimal",
					parsed.Table, dbName, field.FieldType)
			}
		}
	}

	if checked == 0 {
		t.Fatal("没有检查到任何 DECIMAL 字段，测试无效")
	}
	t.Logf("检查 DECIMAL 字段 %d 个，全部使用 decimal.Decimal / *decimal.Decimal", checked)
}

func TestSoftDeleteOnlyOnDesignedTables(t *testing.T) {
	deletedAtType := reflect.TypeOf(gorm.DeletedAt{})
	got := make(map[string]bool)

	for _, model := range allSchemaModels {
		parsed, err := schema.Parse(model, &sync.Map{}, schema.NamingStrategy{})
		if err != nil {
			t.Fatalf("GORM 解析 %T 失败: %v", model, err)
		}
		for _, field := range parsed.Fields {
			if field.FieldType == deletedAtType {
				got[parsed.Table] = true
			}
		}
	}

	want := map[string]bool{
		"sys_user":        true,
		"ai_conversation": true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("使用 gorm.DeletedAt 的表 = %v, want %v", got, want)
	}
	t.Logf("软删除表: %v", got)
}
