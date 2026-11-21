package entity_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"

	"gorm.io/gorm/schema"

	"ai_health/internal/model/entity"
)

// 本文件是纯离线测试：解析 migrations/000001_init.up.sql，与 GORM 解析出的
// 表名、列顺序、列类型、NULL、默认值、索引和外键逐项比对，不需要数据库。

type sqlColumn struct {
	name                     string
	typeName                 string
	notNull                  bool
	defaultValue             string
	autoIncrement            bool
	onUpdateCurrentTimestamp bool
}

type sqlIndexColumn struct {
	name string
	desc bool
}

type sqlIndex struct {
	name    string
	unique  bool
	columns []sqlIndexColumn
}

type sqlForeignKey struct {
	name       string
	columns    []string
	refTable   string
	refColumns []string
	onDelete   string
	onUpdate   string
}

type sqlTable struct {
	name        string
	primaryKey  []string
	columns     []sqlColumn
	indexes     map[string]sqlIndex
	foreignKeys map[string]sqlForeignKey
}

var allSchemaModels = []any{
	&entity.SysUser{},
	&entity.SysOauthIdentity{},
	&entity.SysRole{},
	&entity.SysMenu{},
	&entity.SysUserRole{},
	&entity.SysRoleMenu{},
	&entity.HealthProfile{},
	&entity.HealthRecord{},
	&entity.SportType{},
	&entity.SportInfo{},
	&entity.SportDiseaseDetail{},
	&entity.AIConversation{},
	&entity.AIMessage{},
}

// TestDatabaseHasOnlyModelTables 确认迁移 + GORM 注册后数据库里只有 13 张模型表，
// 没有因为 many2many 配置错误而偷偷生成额外的中间表。
func TestDatabaseHasOnlyModelTables(t *testing.T) {
	db := openTestDB(t)

	var tables []string
	if err := db.Raw("SHOW TABLES").Scan(&tables).Error; err != nil {
		t.Fatalf("读取表清单失败: %v", err)
	}
	sort.Strings(tables)
	want := append([]string(nil), testTables...)
	sort.Strings(want)
	t.Logf("数据库表 (%d): %v", len(tables), tables)

	if strings.Join(tables, ",") != strings.Join(want, ",") {
		t.Fatalf("数据库表清单 = %v, want %v", tables, want)
	}
}

// TestGORMEntitySchemaMatchesMigration 校验 13 个 Model 与迁移 SQL 完全一致。
func TestGORMEntitySchemaMatchesMigration(t *testing.T) {
	sqlTables, err := loadSQLSchema(filepath.Join("..", "..", "..", "migrations", "000001_init.up.sql"))
	if err != nil {
		t.Fatalf("解析迁移 SQL 失败: %v", err)
	}
	if len(sqlTables) != 13 {
		t.Fatalf("迁移 SQL 表数量 = %d, want 13", len(sqlTables))
	}
	if len(allSchemaModels) != 13 {
		t.Fatalf("Model 数量 = %d, want 13", len(allSchemaModels))
	}

	seen := make(map[string]bool, len(sqlTables))
	for _, model := range allSchemaModels {
		parsed, err := schema.Parse(model, &sync.Map{}, schema.NamingStrategy{})
		if err != nil {
			t.Fatalf("GORM 解析 %T 失败: %v", model, err)
		}
		want, ok := sqlTables[parsed.Table]
		if !ok {
			t.Fatalf("Model %T 映射到未知表 %q", model, parsed.Table)
		}
		seen[parsed.Table] = true

		t.Run(parsed.Table, func(t *testing.T) {
			compareColumns(t, parsed, want)
			compareIndexes(t, parsed, want)
			compareForeignKeys(t, parsed, want)
		})
	}

	for name := range sqlTables {
		if !seen[name] {
			t.Errorf("SQL 表 %q 没有对应 Model", name)
		}
	}
}

func compareColumns(t *testing.T, parsed *schema.Schema, want sqlTable) {
	t.Helper()

	if len(parsed.DBNames) != len(want.columns) {
		t.Fatalf("列数量 = %d, want %d (%v)", len(parsed.DBNames), len(want.columns), parsed.DBNames)
	}
	for i, wc := range want.columns {
		gotName := parsed.DBNames[i]
		if gotName != wc.name {
			t.Errorf("第 %d 列 = %q, want %q（顺序必须一致）", i+1, gotName, wc.name)
			continue
		}
		field := parsed.FieldsByDBName[wc.name]
		if field == nil {
			t.Errorf("列 %q 没有对应 GORM Field", wc.name)
			continue
		}

		gotType := normalizeType(string(field.DataType))
		wantType := normalizeType(wc.typeName)
		if gotType != wantType {
			t.Errorf("%s 类型 = %q, want %q", wc.name, gotType, wantType)
		}

		gotNotNull := field.NotNull || field.PrimaryKey
		if gotNotNull != wc.notNull {
			t.Errorf("%s NotNull = %v, want %v", wc.name, gotNotNull, wc.notNull)
		}
		if field.AutoIncrement != wc.autoIncrement {
			t.Errorf("%s AutoIncrement = %v, want %v", wc.name, field.AutoIncrement, wc.autoIncrement)
		}

		switch {
		case wc.defaultValue == "" || strings.EqualFold(wc.defaultValue, "NULL"):
			// 可空列 / 无默认值：GORM 不应声明字面量默认值。
			if field.HasDefaultValue && field.DefaultValue != "" {
				t.Errorf("%s 不应有默认值，GORM default=%q", wc.name, field.DefaultValue)
			}
		case isCurrentTimestamp(wc.defaultValue):
			if field.AutoCreateTime == 0 && field.AutoUpdateTime == 0 {
				t.Errorf("%s 默认值为 %s，但缺少 autoCreateTime/autoUpdateTime", wc.name, wc.defaultValue)
			}
		default:
			if !field.HasDefaultValue {
				t.Errorf("%s 缺少 GORM default 标签，SQL 默认值为 %q", wc.name, wc.defaultValue)
			} else if strings.Trim(field.DefaultValue, "'") != wc.defaultValue {
				t.Errorf("%s GORM default = %q, want %q", wc.name, field.DefaultValue, wc.defaultValue)
			}
		}

		if wc.onUpdateCurrentTimestamp && field.AutoUpdateTime == 0 {
			t.Errorf("%s 有 ON UPDATE CURRENT_TIMESTAMP，但缺少 autoUpdateTime", wc.name)
		}
	}

	if strings.Join(parsed.PrimaryFieldDBNames, ",") != strings.Join(want.primaryKey, ",") {
		t.Errorf("主键 = %v, want %v", parsed.PrimaryFieldDBNames, want.primaryKey)
	}
}

func compareIndexes(t *testing.T, parsed *schema.Schema, want sqlTable) {
	t.Helper()

	gotIndexes := parsed.ParseIndexes()
	if len(gotIndexes) != len(want.indexes) {
		t.Errorf("索引数量 = %d, want %d (got=%v)", len(gotIndexes), len(want.indexes), sortedIndexNames(gotIndexes))
	}
	for name, wi := range want.indexes {
		gi, ok := gotIndexes[name]
		if !ok {
			t.Errorf("缺少索引 %q", name)
			continue
		}
		gotUnique := gi.Class == "UNIQUE"
		if gotUnique != wi.unique {
			t.Errorf("索引 %q unique = %v, want %v", name, gotUnique, wi.unique)
		}
		if len(gi.Fields) != len(wi.columns) {
			t.Errorf("索引 %q 列数 = %d, want %d", name, len(gi.Fields), len(wi.columns))
			continue
		}
		for i, wc := range wi.columns {
			if gi.Fields[i].DBName != wc.name {
				t.Errorf("索引 %q 第 %d 列 = %q, want %q", name, i+1, gi.Fields[i].DBName, wc.name)
			}
			gotDesc := strings.EqualFold(gi.Fields[i].Sort, "DESC")
			if gotDesc != wc.desc {
				t.Errorf("索引 %q 列 %q 排序 = %q, want desc=%v", name, wc.name, gi.Fields[i].Sort, wc.desc)
			}
		}
	}
}

func compareForeignKeys(t *testing.T, parsed *schema.Schema, want sqlTable) {
	t.Helper()

	got := make(map[string]*schema.Constraint)
	for _, rel := range parsed.Relationships.Relations {
		c := rel.ParseConstraint()
		if c == nil || c.Name == "" || c.Schema == nil {
			continue
		}
		// has-many 反向关系会把子表的外键带回父 Model；这里只校验本表拥有的外键。
		if c.Schema.Table != parsed.Table {
			continue
		}
		got[c.Name] = c
	}
	if len(got) != len(want.foreignKeys) {
		t.Errorf("外键数量 = %d, want %d (got=%v)", len(got), len(want.foreignKeys), sortedConstraintNames(got))
	}
	for name, wf := range want.foreignKeys {
		gc, ok := got[name]
		if !ok {
			t.Errorf("缺少外键 %q", name)
			continue
		}
		if gc.Schema == nil || gc.ReferenceSchema == nil {
			t.Errorf("外键 %q 的 Schema/ReferenceSchema 未解析", name)
			continue
		}
		if gc.Schema.Table != parsed.Table {
			t.Errorf("外键 %q 所属表 = %q, want %q", name, gc.Schema.Table, parsed.Table)
		}
		gotCols := constraintColumnNames(gc)
		if strings.Join(gotCols, ",") != strings.Join(wf.columns, ",") {
			t.Errorf("外键 %q 列 = %v, want %v", name, gotCols, wf.columns)
		}
		if gc.ReferenceSchema.Table != wf.refTable {
			t.Errorf("外键 %q 引用表 = %q, want %q", name, gc.ReferenceSchema.Table, wf.refTable)
		}
		gotRefs := constraintReferenceNames(gc)
		if strings.Join(gotRefs, ",") != strings.Join(wf.refColumns, ",") {
			t.Errorf("外键 %q 引用列 = %v, want %v", name, gotRefs, wf.refColumns)
		}
		if normalizeAction(gc.OnDelete) != normalizeAction(wf.onDelete) {
			t.Errorf("外键 %q OnDelete = %q, want %q", name, gc.OnDelete, wf.onDelete)
		}
		if normalizeAction(gc.OnUpdate) != normalizeAction(wf.onUpdate) {
			t.Errorf("外键 %q OnUpdate = %q, want %q", name, gc.OnUpdate, wf.onUpdate)
		}
	}
}

func constraintColumnNames(c *schema.Constraint) []string {
	out := make([]string, 0, len(c.ForeignKeys))
	for _, f := range c.ForeignKeys {
		out = append(out, f.DBName)
	}
	return out
}

func constraintReferenceNames(c *schema.Constraint) []string {
	out := make([]string, 0, len(c.References))
	for _, f := range c.References {
		out = append(out, f.DBName)
	}
	return out
}

func normalizeType(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(s))), " ")
}

func normalizeAction(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "NO ACTION"
	}
	return strings.Join(strings.Fields(strings.ToUpper(s)), " ")
}

func isCurrentTimestamp(s string) bool {
	return strings.HasPrefix(strings.ToUpper(strings.TrimSpace(s)), "CURRENT_TIMESTAMP")
}

func sortedIndexNames(indexes map[string]schema.Index) []string {
	names := make([]string, 0, len(indexes))
	for name := range indexes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedConstraintNames(constraints map[string]*schema.Constraint) []string {
	names := make([]string, 0, len(constraints))
	for name := range constraints {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ---------- SQL 解析 ----------

var (
	createTableRE = regexp.MustCompile("(?is)^CREATE\\s+TABLE\\s+\x60?([a-z0-9_]+)\x60?\\s*\\(")
	columnRE      = regexp.MustCompile("^\x60?([a-z0-9_]+)\x60?\\s+(.+)$")
	columnTypeRE  = regexp.MustCompile("(?i)^([a-z]+(?:\\s+unsigned)?(?:\\([0-9,]+\\))?)")
	defaultRE     = regexp.MustCompile("(?is)\\bDEFAULT\\s+('(?:[^']|'')*'|CURRENT_TIMESTAMP(?:\\([0-9]+\\))?|NULL|[A-Za-z0-9_.+-]+)")
	fkRE          = regexp.MustCompile("(?is)^CONSTRAINT\\s+\x60?([a-z0-9_]+)\x60?\\s+FOREIGN\\s+KEY\\s*\\(([^)]*)\\)\\s+REFERENCES\\s+\x60?([a-z0-9_]+)\x60?\\s*\\(([^)]*)\\)(.*)$")
	actionRE      = regexp.MustCompile("(?i)ON\\s+(DELETE|UPDATE)\\s+(CASCADE|RESTRICT|SET\\s+NULL|NO\\s+ACTION)")
	onUpdateTSRE  = regexp.MustCompile("(?i)ON\\s+UPDATE\\s+CURRENT_TIMESTAMP(?:\\([0-9]+\\))?")
	notNullRE     = regexp.MustCompile("(?i)\\bNOT\\s+NULL\\b")
)

func loadSQLSchema(path string) (map[string]sqlTable, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var clean strings.Builder
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		clean.WriteString(line)
		clean.WriteByte('\n')
	}

	tables := make(map[string]sqlTable)
	for _, stmt := range strings.Split(clean.String(), ";") {
		stmt = strings.TrimSpace(stmt)
		if !strings.HasPrefix(strings.ToUpper(stmt), "CREATE TABLE") {
			continue
		}
		table, err := parseCreateTable(stmt)
		if err != nil {
			return nil, err
		}
		tables[table.name] = table
	}
	return tables, nil
}

func parseCreateTable(stmt string) (sqlTable, error) {
	m := createTableRE.FindStringSubmatch(stmt)
	if m == nil {
		return sqlTable{}, fmt.Errorf("无法解析 CREATE TABLE: %.80s", stmt)
	}
	table := sqlTable{
		name:        m[1],
		indexes:     make(map[string]sqlIndex),
		foreignKeys: make(map[string]sqlForeignKey),
	}

	open := strings.Index(stmt, "(")
	close := strings.LastIndex(stmt, ")")
	if open < 0 || close <= open {
		return sqlTable{}, fmt.Errorf("表 %s 缺少列定义括号", table.name)
	}
	for _, part := range splitTopLevel(stmt[open+1 : close]) {
		upper := strings.ToUpper(part)
		switch {
		case strings.HasPrefix(upper, "PRIMARY KEY"):
			table.primaryKey = parseParenColumns(part)
		case strings.HasPrefix(upper, "UNIQUE KEY"), strings.HasPrefix(upper, "UNIQUE INDEX"):
			offset := len("UNIQUE KEY")
			if strings.HasPrefix(upper, "UNIQUE INDEX") {
				offset = len("UNIQUE INDEX")
			}
			name, cols := parseIndex(part[offset:])
			table.indexes[name] = sqlIndex{name: name, unique: true, columns: cols}
		case strings.HasPrefix(upper, "KEY"), strings.HasPrefix(upper, "INDEX"):
			offset := len("KEY")
			if strings.HasPrefix(upper, "INDEX") {
				offset = len("INDEX")
			}
			name, cols := parseIndex(part[offset:])
			table.indexes[name] = sqlIndex{name: name, unique: false, columns: cols}
		case strings.HasPrefix(upper, "CONSTRAINT"):
			fk, err := parseForeignKey(part)
			if err != nil {
				return sqlTable{}, fmt.Errorf("表 %s: %w", table.name, err)
			}
			table.foreignKeys[fk.name] = fk
		default:
			col, err := parseColumn(part)
			if err != nil {
				return sqlTable{}, fmt.Errorf("表 %s: %w", table.name, err)
			}
			table.columns = append(table.columns, col)
		}
	}
	return table, nil
}

func parseColumn(part string) (sqlColumn, error) {
	m := columnRE.FindStringSubmatch(strings.TrimSpace(part))
	if m == nil {
		return sqlColumn{}, fmt.Errorf("无法解析列定义: %.80s", part)
	}
	col := sqlColumn{
		name:                     m[1],
		notNull:                  notNullRE.MatchString(m[2]),
		autoIncrement:            strings.Contains(strings.ToUpper(m[2]), "AUTO_INCREMENT"),
		onUpdateCurrentTimestamp: onUpdateTSRE.MatchString(m[2]),
	}
	tm := columnTypeRE.FindStringSubmatch(strings.TrimSpace(m[2]))
	if tm == nil {
		return sqlColumn{}, fmt.Errorf("列 %s 无法解析类型: %.80s", col.name, m[2])
	}
	col.typeName = normalizeType(tm[1])
	if dm := defaultRE.FindStringSubmatch(m[2]); dm != nil {
		v := dm[1]
		if strings.HasPrefix(v, "'") {
			v = strings.Trim(v, "'")
			v = strings.ReplaceAll(v, "''", "'")
		}
		col.defaultValue = v
	}
	return col, nil
}

func parseIndex(rest string) (string, []sqlIndexColumn) {
	rest = strings.TrimSpace(rest)
	open := strings.Index(rest, "(")
	if open < 0 {
		return "", nil
	}
	name := strings.TrimSpace(strings.Trim(rest[:open], "\x60"))
	var cols []sqlIndexColumn
	for _, item := range splitTopLevel(rest[open+1 : strings.LastIndex(rest, ")")]) {
		fields := strings.Fields(item)
		if len(fields) == 0 {
			continue
		}
		col := sqlIndexColumn{name: strings.Trim(fields[0], "\x60")}
		if len(fields) > 1 && strings.EqualFold(fields[1], "DESC") {
			col.desc = true
		}
		cols = append(cols, col)
	}
	return name, cols
}

func parseParenColumns(part string) []string {
	open := strings.Index(part, "(")
	close := strings.LastIndex(part, ")")
	if open < 0 || close <= open {
		return nil
	}
	var cols []string
	for _, item := range splitTopLevel(part[open+1 : close]) {
		fields := strings.Fields(item)
		if len(fields) > 0 {
			cols = append(cols, strings.Trim(fields[0], "\x60"))
		}
	}
	return cols
}

func parseForeignKey(part string) (sqlForeignKey, error) {
	m := fkRE.FindStringSubmatch(strings.TrimSpace(part))
	if m == nil {
		return sqlForeignKey{}, fmt.Errorf("无法解析外键: %.120s", part)
	}
	fk := sqlForeignKey{
		name:       m[1],
		columns:    splitNameList(m[2]),
		refTable:   m[3],
		refColumns: splitNameList(m[4]),
	}
	for _, am := range actionRE.FindAllStringSubmatch(m[5], -1) {
		switch strings.ToUpper(am[1]) {
		case "DELETE":
			fk.onDelete = normalizeAction(am[2])
		case "UPDATE":
			fk.onUpdate = normalizeAction(am[2])
		}
	}
	return fk, nil
}

func splitNameList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		part = strings.Trim(strings.TrimSpace(part), "\x60")
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// splitTopLevel 按顶层逗号切分，忽略括号内与引号内的逗号。
func splitTopLevel(s string) []string {
	var (
		parts []string
		depth int
		quote byte
		start int
	)
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if quote != 0 {
			if ch == quote {
				// 处理 SQL 的 '' 转义。
				if i+1 < len(s) && s[i+1] == quote {
					i++
					continue
				}
				quote = 0
			}
			continue
		}
		switch ch {
		case '\'', '"', '\x60':
			quote = ch
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	parts = append(parts, strings.TrimSpace(s[start:]))
	return parts
}
