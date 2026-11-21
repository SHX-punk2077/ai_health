package seed

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	glogger "gorm.io/gorm/logger"

	"ai_health/internal/consts"
	"ai_health/internal/model/entity"
	"ai_health/internal/pkg/hash"
)

// Options 控制一次 seed 执行。
type Options struct {
	// DSN 必须显式提供，只接受开发/演示数据库。
	DSN string
	// Yes 为 false 时只做 dry-run，不建立数据库连接。
	Yes bool
	// Anchor 是演示数据的日期锚点，空值使用 DefaultAnchor。
	Anchor string
	// RandomSeed 固定随机种子，0 使用 DefaultRandomSeed。
	RandomSeed int64
	// Out 输出目标，空值使用 os.Stdout。
	Out io.Writer
}

// ApplyStats 是一次实际写入的统计。
type ApplyStats struct {
	Users    int
	Profiles int
	Records  int
}

// Run 校验环境、生成计划并（在 Yes 时）写入数据库。
func Run(ctx context.Context, opts Options) error {
	out := opts.Out
	if out == nil {
		out = os.Stdout
	}

	dsn, dbName, err := inspectDSN(opts.DSN)
	if err != nil {
		return err
	}
	anchor, err := parseAnchor(opts.Anchor)
	if err != nil {
		return err
	}
	if anchor.After(dateOnly(time.Now().UTC())) {
		return fmt.Errorf("锚点日期 %s 不能晚于今天（UTC）", anchor.Format("2006-01-02"))
	}

	randomSeed := opts.RandomSeed
	if randomSeed == 0 {
		randomSeed = DefaultRandomSeed
	}
	plan := BuildPlan(anchor, randomSeed)
	if err := validatePlan(plan); err != nil {
		return err
	}

	fmt.Fprintf(out, "目标数据库: %s\n", dbName)
	fmt.Fprintf(out, "锚点日期: %s（记录范围截止 %s）\n", anchor.Format("2006-01-02"), anchor.AddDate(0, 0, -1).Format("2006-01-02"))
	fmt.Fprintf(out, "随机种子: %d\n", randomSeed)
	for _, u := range plan.Users {
		profile := "否"
		if u.Profile != nil {
			profile = "是"
		}
		fmt.Fprintf(out, "演示用户: %s（档案: %s，记录: %d 条）\n", u.Username, profile, len(u.Records))
	}

	if !opts.Yes {
		fmt.Fprintln(out, "dry-run: 未写入数据库；确认执行请加 -yes。")
		return nil
	}

	db, err := openSeedDB(dsn)
	if err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("获取数据库连接失败: %w", err)
	}
	defer sqlDB.Close()

	stats, err := Apply(ctx, db, plan)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "已写入: 用户 %d，档案 %d，健康记录 %d 条。\n", stats.Users, stats.Profiles, stats.Records)
	fmt.Fprintln(out, "演示账号密码仅用于开发环境，请勿用于生产。")
	return nil
}

// Apply 在单事务内重建三个演示用户的健康数据。
// 公开入口只允许开发/演示数据库；测试内部使用 apply 并显式允许 _test 库。
func Apply(ctx context.Context, db *gorm.DB, plan Plan) (ApplyStats, error) {
	return apply(ctx, db, plan, false)
}

func apply(ctx context.Context, db *gorm.DB, plan Plan, allowTest bool) (ApplyStats, error) {
	var stats ApplyStats
	if db == nil {
		return stats, errors.New("数据库连接为空")
	}
	if err := validatePlan(plan); err != nil {
		return stats, err
	}
	if _, err := validateSeedDB(ctx, db, allowTest); err != nil {
		return stats, err
	}

	passwordHash, err := hash.Hash(DemoPassword)
	if err != nil {
		return stats, fmt.Errorf("生成演示密码哈希失败: %w", err)
	}

	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		role, err := ensureDemoSeedRole(tx)
		if err != nil {
			return err
		}
		users := make([]*entity.SysUser, 0, len(plan.Users))
		ids := make([]uint64, 0, len(plan.Users))
		for _, spec := range plan.Users {
			u, err := upsertDemoUser(tx, spec, passwordHash, plan.Anchor, role)
			if err != nil {
				return err
			}
			users = append(users, u)
			ids = append(ids, u.ID)
		}

		// 只清理这三个演示用户的健康数据，绝不 TRUNCATE 其他表或影响普通用户。
		if err := tx.Where("user_id IN ?", ids).Delete(&entity.HealthRecord{}).Error; err != nil {
			return fmt.Errorf("清理演示健康记录失败: %w", err)
		}
		if err := tx.Where("user_id IN ?", ids).Delete(&entity.HealthProfile{}).Error; err != nil {
			return fmt.Errorf("清理演示健康档案失败: %w", err)
		}

		for i, spec := range plan.Users {
			uid := users[i].ID
			if spec.Profile != nil {
				p := *spec.Profile
				p.UserID = uid
				if err := tx.Create(&p).Error; err != nil {
					return fmt.Errorf("写入演示健康档案失败（%s）: %w", spec.Username, err)
				}
				stats.Profiles++
			}
			for _, rec := range spec.Records {
				r := rec
				r.UserID = uid
				if err := tx.Create(&r).Error; err != nil {
					return fmt.Errorf("写入演示健康记录失败（%s）: %w", spec.Username, err)
				}
				stats.Records++
			}
		}
		stats.Users = len(plan.Users)
		return nil
	})
	if err != nil {
		return ApplyStats{}, err
	}
	return stats, nil
}

// demoSeedRoleCode 是 seed 工具独占的演示用户所有权标记。
// 该角色禁用且不绑定任何菜单，普通注册和管理端角色选择都不会使用它。
const (
	demoSeedRoleCode        = "demo_seed"
	demoSeedRoleName        = "演示数据专用角色"
	demoSeedRoleDescription = "仅由演示数据 seed 工具创建和维护；禁用且无菜单，禁止分配给普通用户"
)

// ensureDemoSeedRole 创建或验证 seed 独占角色。
// 已存在但被启用、带有菜单或元数据不匹配时直接拒绝，不静默修改可能被他人使用的角色。
func ensureDemoSeedRole(tx *gorm.DB) (*entity.SysRole, error) {
	var role entity.SysRole
	err := tx.Where("role_code = ?", demoSeedRoleCode).First(&role).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		desc := demoSeedRoleDescription
		role = entity.SysRole{
			RoleCode:    demoSeedRoleCode,
			RoleName:    demoSeedRoleName,
			Description: &desc,
			Status:      consts.StatusDisabled,
		}
		if err := tx.Create(&role).Error; err != nil {
			return nil, fmt.Errorf("创建演示数据专用角色失败: %w", err)
		}
		// status 列有 default:1，GORM 可能省略零值，因此创建后显式写回禁用状态。
		if err := tx.Model(&entity.SysRole{}).Where("id = ?", role.ID).
			Update("status", consts.StatusDisabled).Error; err != nil {
			return nil, fmt.Errorf("禁用演示数据专用角色失败: %w", err)
		}
		role.Status = consts.StatusDisabled
	} else if err != nil {
		return nil, fmt.Errorf("查询演示数据专用角色失败: %w", err)
	} else {
		if role.Status != consts.StatusDisabled {
			return nil, fmt.Errorf("角色 %s 已存在但未禁用，拒绝使用；请人工确认后清理", demoSeedRoleCode)
		}
		if role.RoleName != demoSeedRoleName || role.Description == nil || *role.Description != demoSeedRoleDescription {
			return nil, fmt.Errorf("角色 %s 的元数据与 seed 所有权标记不一致，拒绝使用", demoSeedRoleCode)
		}
	}

	var menuCount int64
	if err := tx.Table("sys_role_menu").Where("role_id = ?", role.ID).Count(&menuCount).Error; err != nil {
		return nil, fmt.Errorf("检查演示数据专用角色菜单失败: %w", err)
	}
	if menuCount != 0 {
		return nil, fmt.Errorf("角色 %s 绑定了 %d 个菜单，拒绝作为无权限演示角色使用", demoSeedRoleCode, menuCount)
	}
	return &role, nil
}

// upsertDemoUser 只接管“用户名 + 恰好一个 seed 专用角色”都匹配的用户。
// 同名普通用户或带其它角色的用户一律拒绝，且在拒绝前不会修改密码、状态或健康数据。
func upsertDemoUser(tx *gorm.DB, spec DemoUser, passwordHash string, anchor time.Time, role *entity.SysRole) (*entity.SysUser, error) {
	var u entity.SysUser
	err := tx.Unscoped().Where("username = ?", spec.Username).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		nickname := spec.Nickname
		u = entity.SysUser{
			Username:     spec.Username,
			PasswordHash: &passwordHash,
			Nickname:     &nickname,
			Status:       consts.StatusEnabled,
			CreatedAt:    anchor.AddDate(0, 0, -120),
			UpdatedAt:    anchor,
		}
		if err := tx.Create(&u).Error; err != nil {
			return nil, fmt.Errorf("创建演示用户失败（%s）: %w", spec.Username, err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("查询演示用户失败（%s）: %w", spec.Username, err)
	} else {
		roles, err := rolesOfUser(tx, u.ID)
		if err != nil {
			return nil, fmt.Errorf("检查演示用户角色失败（%s）: %w", spec.Username, err)
		}
		if len(roles) != 1 || roles[0].ID != role.ID {
			return nil, fmt.Errorf("用户名 %s 已被非演示用户占用或带有额外角色，拒绝修改；请人工确认后清理", spec.Username)
		}
		// 只有确认过 seed 所有权标记的用户才允许重置演示密码并恢复为启用状态。
		updates := map[string]any{
			"password_hash": passwordHash,
			"nickname":      spec.Nickname,
			"status":        consts.StatusEnabled,
			"deleted_at":    nil,
			"updated_at":    anchor,
		}
		if err := tx.Unscoped().Model(&entity.SysUser{}).Where("id = ?", u.ID).Updates(updates).Error; err != nil {
			return nil, fmt.Errorf("更新演示用户失败（%s）: %w", spec.Username, err)
		}
		u.PasswordHash = &passwordHash
		u.Nickname = &spec.Nickname
		u.Status = consts.StatusEnabled
		u.DeletedAt = gorm.DeletedAt{}
	}

	if err := tx.Exec("INSERT IGNORE INTO sys_user_role (user_id, role_id) VALUES (?, ?)", u.ID, role.ID).Error; err != nil {
		return nil, fmt.Errorf("绑定演示用户角色失败（%s）: %w", spec.Username, err)
	}
	return &u, nil
}

// rolesOfUser 返回用户当前的全部角色（包括禁用角色），用于严格验证演示身份。
func rolesOfUser(tx *gorm.DB, uid uint64) ([]entity.SysRole, error) {
	var roles []entity.SysRole
	err := tx.Table("sys_role").
		Select("sys_role.*").
		Joins("JOIN sys_user_role ON sys_user_role.role_id = sys_role.id").
		Where("sys_user_role.user_id = ?", uid).
		Order("sys_role.id ASC").
		Find(&roles).Error
	return roles, err
}

func validatePlan(plan Plan) error {
	if plan.Anchor.IsZero() {
		return errors.New("演示数据锚点不能为空")
	}
	if len(plan.Users) != 3 {
		return fmt.Errorf("演示用户必须恰好为 3 个，实际 %d 个", len(plan.Users))
	}
	expected := map[string]bool{
		"demo_user_a": true,
		"demo_user_b": true,
		"demo_user_c": true,
	}
	seen := make(map[string]bool, len(plan.Users))
	for _, u := range plan.Users {
		if !expected[u.Username] {
			return fmt.Errorf("不允许写入非指定演示用户 %q", u.Username)
		}
		if seen[u.Username] {
			return fmt.Errorf("演示用户重复: %s", u.Username)
		}
		seen[u.Username] = true
		if strings.TrimSpace(u.Nickname) == "" {
			return fmt.Errorf("演示用户昵称不能为空: %s", u.Username)
		}
		if u.Username == "demo_user_c" && u.Profile != nil {
			return errors.New("demo_user_c 不允许包含健康档案")
		}
		if u.Username != "demo_user_c" && u.Profile == nil {
			return fmt.Errorf("演示用户 %s 缺少健康档案", u.Username)
		}
		if len(u.Records) == 0 {
			return fmt.Errorf("演示用户 %s 缺少健康记录", u.Username)
		}
		for i := range u.Records {
			rec := &u.Records[i]
			if rec.MeasuredAt.IsZero() {
				return fmt.Errorf("演示用户 %s 存在空测量时间", u.Username)
			}
			if rec.MeasuredAt.After(plan.Anchor) {
				return fmt.Errorf("演示用户 %s 存在晚于锚点的测量时间", u.Username)
			}
			if !validSource(rec.Source) {
				return fmt.Errorf("演示用户 %s 存在非法来源 %q", u.Username, rec.Source)
			}
			if rec.SystolicPressure != nil && rec.DiastolicPressure != nil && *rec.SystolicPressure <= *rec.DiastolicPressure {
				return fmt.Errorf("演示用户 %s 存在收缩压不大于舒张压的记录", u.Username)
			}
		}
	}
	for name := range expected {
		if !seen[name] {
			return fmt.Errorf("缺少演示用户 %s", name)
		}
	}
	return nil
}

func validSource(source string) bool {
	switch source {
	case entity.SourceManual, entity.SourceDevice, entity.SourceImport, entity.SourceAI:
		return true
	default:
		return false
	}
}

func inspectDSN(raw string) (string, string, error) {
	return inspectDSNWithTest(raw, false)
}

func inspectDSNWithTest(raw string, allowTest bool) (string, string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", "", errors.New("必须通过 AIH_SEED_MYSQL_DSN 显式提供开发数据库 DSN")
	}
	cfg, err := mysqlDriver.ParseDSN(raw)
	if err != nil {
		return "", "", fmt.Errorf("DSN 格式非法: %w", err)
	}
	name := strings.ToLower(strings.TrimSpace(cfg.DBName))
	if name == "" {
		return "", "", errors.New("DSN 必须包含数据库名")
	}
	if strings.Contains(name, "prod") || strings.Contains(name, "production") {
		return "", "", fmt.Errorf("禁止连接生产数据库 %q", cfg.DBName)
	}
	if !allowTest && strings.Contains(name, "test") {
		return "", "", fmt.Errorf("禁止连接测试数据库 %q", cfg.DBName)
	}
	if allowTest {
		if !strings.HasSuffix(name, "_test") && !strings.HasSuffix(name, "_dev") && !strings.HasSuffix(name, "_demo") {
			return "", "", fmt.Errorf("数据库名 %q 必须以 _test、_dev 或 _demo 结尾", cfg.DBName)
		}
	} else if !strings.HasSuffix(name, "_dev") && !strings.HasSuffix(name, "_demo") {
		return "", "", fmt.Errorf("数据库名 %q 必须以 _dev 或 _demo 结尾", cfg.DBName)
	}
	if err := validateSeedHost(cfg); err != nil {
		return "", "", err
	}
	// 保证时间字段可被 GORM 扫描，且与项目 API 的 UTC 约定一致。
	cfg.ParseTime = true
	cfg.Loc = time.UTC
	return cfg.FormatDSN(), cfg.DBName, nil
}

// validateSeedHost 只允许本机回环地址或本机 Unix socket。
// 仅校验库名不足以阻止 tcp(prod-host)/ai_health_dev 这类“名字合法但主机远程”的 DSN。
func validateSeedHost(cfg *mysqlDriver.Config) error {
	switch strings.ToLower(strings.TrimSpace(cfg.Net)) {
	case "tcp", "tcp4", "tcp6", "":
		addr := strings.TrimSpace(cfg.Addr)
		if addr == "" {
			return errors.New("DSN 缺少数据库主机地址")
		}
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			host = strings.Trim(addr, "[]")
		}
		if strings.EqualFold(host, "localhost") {
			return nil
		}
		ip := net.ParseIP(host)
		if ip != nil && ip.IsLoopback() {
			return nil
		}
		return fmt.Errorf("数据库主机 %q 不是本机回环地址；seed 只允许连接本机开发库", host)
	case "unix":
		addr := strings.TrimSpace(cfg.Addr)
		for _, prefix := range []string{"/var/run/mysqld/", "/run/mysqld/", "/tmp/"} {
			if strings.HasPrefix(addr, prefix) {
				return nil
			}
		}
		return fmt.Errorf("Unix socket 路径 %q 不在允许的本机目录内", addr)
	default:
		return fmt.Errorf("不支持的数据库连接方式 %q；seed 只允许 tcp 回环或本机 Unix socket", cfg.Net)
	}
}

// validateSeedDB 在 Apply 内部再次校验实际连接，不能只依赖 Run 的前置检查。
// 这样即使测试或其它代码直接调用 Apply，也无法把演示数据写入测试/生产库或远程库。
func validateSeedDB(ctx context.Context, db *gorm.DB, allowTest bool) (string, error) {
	if db == nil || db.Dialector == nil {
		return "", errors.New("数据库连接为空")
	}
	var dsn string
	switch d := db.Dialector.(type) {
	case mysql.Dialector:
		dsn = d.DSN
		if d.DSNConfig != nil {
			dsn = d.DSNConfig.FormatDSN()
		}
	case *mysql.Dialector:
		dsn = d.DSN
		if d.DSNConfig != nil {
			dsn = d.DSNConfig.FormatDSN()
		}
	default:
		return "", errors.New("seed 只支持 MySQL 数据库")
	}
	if strings.TrimSpace(dsn) == "" {
		return "", errors.New("无法读取 MySQL 连接配置")
	}
	_, wantName, err := inspectDSNWithTest(dsn, allowTest)
	if err != nil {
		return "", err
	}
	var actualName string
	if err := db.WithContext(ctx).Raw("SELECT DATABASE()").Scan(&actualName).Error; err != nil {
		return "", fmt.Errorf("校验实际数据库失败: %w", err)
	}
	if actualName == "" || actualName != wantName {
		return "", fmt.Errorf("实际连接的数据库 %q 与 DSN 数据库 %q 不一致，拒绝写入", actualName, wantName)
	}
	return actualName, nil
}

func parseAnchor(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = DefaultAnchor
	}
	anchor, err := time.ParseInLocation("2006-01-02", raw, time.UTC)
	if err != nil {
		return time.Time{}, fmt.Errorf("锚点日期格式必须为 YYYY-MM-DD: %w", err)
	}
	return anchor, nil
}

func openSeedDB(dsn string) (*gorm.DB, error) {
	return gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger:  glogger.Default.LogMode(glogger.Silent),
		NowFunc: func() time.Time { return time.Now().UTC() },
	})
}
