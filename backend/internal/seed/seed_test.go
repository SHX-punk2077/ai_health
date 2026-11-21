package seed

import (
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"ai_health/internal/model/entity"
)

func TestBuildPlanDeterministicAndSafe(t *testing.T) {
	anchor := time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC)
	p1 := BuildPlan(anchor, 42)
	p2 := BuildPlan(anchor, 42)
	if !reflect.DeepEqual(p1, p2) {
		t.Fatal("同一 anchor 和随机种子应生成完全一致的计划")
	}
	p3 := BuildPlan(anchor, 43)
	if reflect.DeepEqual(p1.Users, p3.Users) {
		t.Fatal("不同随机种子不应生成完全相同的数据")
	}
	if len(p1.Users) != 3 {
		t.Fatalf("演示用户数量 = %d，期望 3", len(p1.Users))
	}

	byName := make(map[string]DemoUser, 3)
	for _, u := range p1.Users {
		byName[u.Username] = u
	}
	if byName["demo_user_a"].Profile == nil {
		t.Fatal("demo_user_a 应有健康档案")
	}
	if byName["demo_user_b"].Profile == nil {
		t.Fatal("demo_user_b 应有健康档案")
	}
	if byName["demo_user_c"].Profile != nil {
		t.Fatal("demo_user_c 不应有健康档案")
	}

	for _, u := range p1.Users {
		if len(u.Records) == 0 {
			t.Fatalf("%s 没有记录", u.Username)
		}
		for i, rec := range u.Records {
			if rec.MeasuredAt.IsZero() {
				t.Fatalf("%s 第 %d 条记录测量时间为空", u.Username, i)
			}
			if rec.MeasuredAt.After(anchor) {
				t.Fatalf("%s 第 %d 条记录测量时间晚于锚点", u.Username, i)
			}
			if !validSource(rec.Source) {
				t.Fatalf("%s 第 %d 条记录来源非法: %q", u.Username, i, rec.Source)
			}
			if rec.SystolicPressure != nil && rec.DiastolicPressure != nil && *rec.SystolicPressure <= *rec.DiastolicPressure {
				t.Fatalf("%s 第 %d 条记录收缩压不大于舒张压", u.Username, i)
			}
		}
	}
	if err := validatePlan(p1); err != nil {
		t.Fatalf("BuildPlan 结果未通过安全校验: %v", err)
	}
}

func TestInspectDSNAllowsOnlyDevOrDemo(t *testing.T) {
	tests := []struct {
		name    string
		dsn     string
		wantDB  string
		wantErr bool
	}{
		{name: "dev", dsn: "root:pass@tcp(127.0.0.1:3306)/ai_health_dev?charset=utf8mb4", wantDB: "ai_health_dev"},
		{name: "demo", dsn: "root:pass@tcp(127.0.0.1:3306)/ai_health_demo", wantDB: "ai_health_demo"},
		{name: "empty", wantErr: true},
		{name: "normal database", dsn: "root:pass@tcp(127.0.0.1:3306)/ai_health", wantErr: true},
		{name: "test database", dsn: "root:pass@tcp(127.0.0.1:3306)/ai_health_test", wantErr: true},
		{name: "prod database", dsn: "root:pass@tcp(127.0.0.1:3306)/ai_health_prod", wantErr: true},
		{name: "dev test database", dsn: "root:pass@tcp(127.0.0.1:3306)/ai_health_dev_test", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			normalized, db, err := inspectDSN(tc.dsn)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("期望拒绝 DSN，实际通过（db=%s）", db)
				}
				return
			}
			if err != nil {
				t.Fatalf("inspectDSN() error = %v", err)
			}
			if db != tc.wantDB {
				t.Fatalf("db = %q，期望 %q", db, tc.wantDB)
			}
			if !strings.Contains(normalized, "parseTime=true") {
				t.Fatalf("归一化 DSN 必须强制 parseTime=true: %s", normalized)
			}
		})
	}
}

func TestInspectDSNRejectsRemoteOrUnsafeHosts(t *testing.T) {
	tests := []struct {
		name    string
		dsn     string
		wantErr bool
	}{
		{name: "loopback ipv4", dsn: "root:pass@tcp(127.0.0.1:3306)/ai_health_dev"},
		{name: "localhost", dsn: "root:pass@tcp(localhost:3306)/ai_health_dev"},
		{name: "loopback ipv6", dsn: "root:pass@tcp([::1]:3306)/ai_health_dev"},
		{name: "local unix socket", dsn: "root:pass@unix(/var/run/mysqld/mysqld.sock)/ai_health_dev"},
		{name: "remote host with dev name", dsn: "root:pass@tcp(prod-host:3306)/ai_health_dev", wantErr: true},
		{name: "public ip with dev name", dsn: "root:pass@tcp(203.0.113.10:3306)/ai_health_dev", wantErr: true},
		{name: "docker host alias", dsn: "root:pass@tcp(host.docker.internal:3306)/ai_health_dev", wantErr: true},
		{name: "all interfaces", dsn: "root:pass@tcp(0.0.0.0:3306)/ai_health_dev", wantErr: true},
		{name: "remote unix socket", dsn: "root:pass@unix(/etc/passwd)/ai_health_dev", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := inspectDSN(tc.dsn)
			if tc.wantErr && err == nil {
				t.Fatal("应拒绝远程或非本机数据库")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("应允许本机开发库: %v", err)
			}
		})
	}
}

func TestInspectDSNAllowsTestOnlyForInternalApply(t *testing.T) {
	const dsn = "root:pass@tcp(127.0.0.1:3306)/ai_health_test"
	if _, _, err := inspectDSN(dsn); err == nil {
		t.Fatal("公开入口必须拒绝测试库")
	}
	if _, db, err := inspectDSNWithTest(dsn, true); err != nil || db != "ai_health_test" {
		t.Fatalf("内部 Apply 测试模式应允许 _test 库: db=%q err=%v", db, err)
	}
}

func TestDemoBirthDateMatchesAge(t *testing.T) {
	anchor := time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC)
	plan := BuildPlan(anchor, DefaultRandomSeed)
	for _, u := range plan.Users {
		if u.Profile == nil {
			continue
		}
		if u.Profile.Age == nil || u.Profile.BirthDate == nil {
			t.Fatalf("%s 缺少年龄或生日", u.Username)
		}
		age := int(*u.Profile.Age)
		want := anchor.AddDate(-age, 0, 0)
		if !u.Profile.BirthDate.Equal(want) {
			t.Fatalf("%s 年龄 %d 与生日 %s 不一致，期望 %s", u.Username, age, u.Profile.BirthDate.Format("2006-01-02"), want.Format("2006-01-02"))
		}
	}
}

func TestRunDryRunDoesNotConnect(t *testing.T) {
	var out bytes.Buffer
	err := Run(context.Background(), Options{
		DSN:        "root:pass@tcp(127.0.0.1:1)/ai_health_dev",
		Anchor:     DefaultAnchor,
		RandomSeed: 42,
		Out:        &out,
	})
	if err != nil {
		t.Fatalf("dry-run 不应尝试连接数据库: %v", err)
	}
	if !strings.Contains(out.String(), "dry-run") {
		t.Fatalf("dry-run 输出缺少提示: %s", out.String())
	}
	if strings.Contains(out.String(), "已写入") {
		t.Fatalf("dry-run 不应报告写入: %s", out.String())
	}
}

func TestRunRejectsUnsafeInputs(t *testing.T) {
	tests := []struct {
		name string
		opts Options
		want string
	}{
		{name: "empty dsn", opts: Options{Anchor: DefaultAnchor}, want: "AIH_SEED_MYSQL_DSN"},
		{name: "test dsn", opts: Options{DSN: "root:pass@tcp(127.0.0.1:3306)/ai_health_test", Anchor: DefaultAnchor}, want: "测试数据库"},
		{name: "prod dsn", opts: Options{DSN: "root:pass@tcp(127.0.0.1:3306)/ai_health_prod", Anchor: DefaultAnchor}, want: "生产数据库"},
		{name: "bad anchor", opts: Options{DSN: "root:pass@tcp(127.0.0.1:3306)/ai_health_dev", Anchor: "2026/11/21"}, want: "YYYY-MM-DD"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.opts.Out = &bytes.Buffer{}
			err := Run(context.Background(), tc.opts)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Run() error = %v，期望包含 %q", err, tc.want)
			}
		})
	}
}

func TestRunRejectsFutureAnchor(t *testing.T) {
	future := time.Now().UTC().AddDate(0, 0, 1).Format("2006-01-02")
	err := Run(context.Background(), Options{
		DSN:    "root:pass@tcp(127.0.0.1:3306)/ai_health_dev",
		Anchor: future,
		Out:    &bytes.Buffer{},
	})
	if err == nil || !strings.Contains(err.Error(), "不能晚于今天") {
		t.Fatalf("Run() error = %v，期望拒绝未来锚点", err)
	}
}

func TestApplyRejectsNonDemoPlanBeforeDatabase(t *testing.T) {
	plan := BuildPlan(time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC), 42)
	plan.Users[0].Username = "some_other_user"
	_, err := Apply(context.Background(), &gorm.DB{}, plan)
	if err == nil || !strings.Contains(err.Error(), "非指定演示用户") {
		t.Fatalf("Apply() error = %v，期望在访问数据库前拒绝非 demo 用户", err)
	}
}

func TestValidatePlanRequiresThreeExpectedUsers(t *testing.T) {
	plan := BuildPlan(time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC), 42)
	if err := validatePlan(plan); err != nil {
		t.Fatalf("有效计划被拒绝: %v", err)
	}

	missing := plan
	missing.Users = append([]DemoUser(nil), plan.Users[:2]...)
	if err := validatePlan(missing); err == nil {
		t.Fatal("缺少用户时应拒绝")
	}

	badProfile := plan
	badProfile.Users = append([]DemoUser(nil), plan.Users...)
	badProfile.Users[2].Profile = badProfile.Users[0].Profile
	if err := validatePlan(badProfile); err == nil {
		t.Fatal("demo_user_c 带档案时应拒绝")
	}

	badSource := plan
	badSource.Users = append([]DemoUser(nil), plan.Users...)
	badSource.Users[0].Records = append([]entity.HealthRecord(nil), plan.Users[0].Records...)
	badSource.Users[0].Records[0].Source = "unknown"
	if err := validatePlan(badSource); err == nil {
		t.Fatal("非法来源时应拒绝")
	}
}
