package entity_test

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"ai_health/internal/model/entity"
)

func TestModelCreateAndQuery(t *testing.T) {
	db := openTestDB(t)

	user := createUser(t, db, "alice")
	oauth := &entity.SysOauthIdentity{UserID: user.ID, Provider: "wechat", OpenID: "openid-alice"}
	mustCreate(t, db, oauth)

	// provider + open_id 唯一：同一第三方身份不能重复绑定。
	dupOauth := &entity.SysOauthIdentity{UserID: user.ID, Provider: "wechat", OpenID: "openid-alice"}
	if err := db.Create(dupOauth).Error; mysqlErrCode(err) != 1062 {
		t.Fatalf("重复 OAuth 身份错误码 = %d, want 1062 (err=%v)", mysqlErrCode(err), err)
	}

	var userWithOauth entity.SysUser
	if err := db.Preload("OauthIdentities").First(&userWithOauth, user.ID).Error; err != nil {
		t.Fatalf("预加载 OAuth 身份失败: %v", err)
	}
	if len(userWithOauth.OauthIdentities) != 1 || userWithOauth.OauthIdentities[0].OpenID != "openid-alice" {
		t.Fatalf("SysUser.OauthIdentities = %+v", userWithOauth.OauthIdentities)
	}

	role := createRole(t, db, "admin")
	root := createMenu(t, db, "system", nil, entity.MenuTypeDir)
	child := createMenu(t, db, "system:user", &root.ID, entity.MenuTypeMenu)

	var menuTree entity.SysMenu
	if err := db.Preload("Children").First(&menuTree, root.ID).Error; err != nil {
		t.Fatalf("预加载菜单子节点失败: %v", err)
	}
	if len(menuTree.Children) != 1 || menuTree.Children[0].ID != child.ID {
		t.Fatalf("SysMenu.Children = %+v", menuTree.Children)
	}

	mustCreate(t, db, &entity.SysUserRole{UserID: user.ID, RoleID: role.ID})
	mustCreate(t, db, &entity.SysRoleMenu{RoleID: role.ID, MenuID: child.ID})

	profile := &entity.HealthProfile{
		UserID:             user.ID,
		RealName:           ptr("张三"),
		Age:                ptr(uint8(30)),
		Gender:             ptr(uint8(1)),
		HeightCm:           dec("175.50"),
		WeightKg:           dec("70.10"),
		BloodSugarMmol:     dec("5.60"),
		SystolicPressure:   ptr(uint16(120)),
		DiastolicPressure:  ptr(uint16(80)),
		HeartRate:          ptr(uint16(72)),
		SleepDurationHours: dec("7.25"),
		SleepQuality:       ptr("良好"),
		DailyWaterMl:       ptr(uint32(2000)),
	}
	mustCreate(t, db, profile)

	measuredAt := time.Date(2026, 11, 20, 8, 30, 15, 123_000_000, time.UTC)
	record := &entity.HealthRecord{
		UserID:         user.ID,
		HeightCm:       dec("175.00"),
		WeightKg:       dec("70.05"),
		BloodSugarMmol: dec("5.55"),
		MeasuredAt:     measuredAt,
		Source:         entity.SourceDevice,
		Remark:         ptr("晨起空腹"),
	}
	mustCreate(t, db, record)

	sportType := createSportType(t, db, "running")
	sportInfo := &entity.SportInfo{
		SportTypeID:       sportType.ID,
		SuitableTime:      ptr("早晨"),
		SuitableHeartRate: ptr("120-140"),
		DurationMinutes:   ptr(uint16(30)),
		Intensity:         ptr("moderate"),
		Status:            1,
	}
	mustCreate(t, db, sportInfo)
	disease := &entity.SportDiseaseDetail{
		SportTypeID:       sportType.ID,
		DiseaseName:       "高血压",
		RecommendedMethod: ptr("快走"),
		Precautions:       ptr("避免憋气"),
		RiskLevel:         ptr(uint8(2)),
	}
	mustCreate(t, db, disease)

	conv := createConversation(t, db, user.ID)
	msg := &entity.AIMessage{
		ConversationID: conv.ID,
		MessageRole:    entity.RoleUser,
		Content:        "我最近血压有点高",
		SequenceNo:     1,
	}
	mustCreate(t, db, msg)

	counts := []struct {
		name  string
		model any
	}{
		{"sys_user", &entity.SysUser{}},
		{"sys_oauth_identity", &entity.SysOauthIdentity{}},
		{"sys_role", &entity.SysRole{}},
		// createMenu 建了根菜单和子菜单两行。
		{"sys_menu", &entity.SysMenu{}},
		{"sys_user_role", &entity.SysUserRole{}},
		{"sys_role_menu", &entity.SysRoleMenu{}},
		{"health_profile", &entity.HealthProfile{}},
		{"health_record", &entity.HealthRecord{}},
		{"sport_type", &entity.SportType{}},
		{"sport_info", &entity.SportInfo{}},
		{"sport_disease_detail", &entity.SportDiseaseDetail{}},
		{"ai_conversation", &entity.AIConversation{}},
		{"ai_message", &entity.AIMessage{}},
	}
	for _, tc := range counts {
		t.Run(tc.name, func(t *testing.T) {
			want := int64(1)
			if tc.name == "sys_menu" {
				want = 2
			}
			assertCount(t, db, tc.model, want)
		})
	}

	var gotSport entity.SportInfo
	if err := db.Preload("SportType").First(&gotSport, sportInfo.ID).Error; err != nil {
		t.Fatalf("预加载运动类型失败: %v", err)
	}
	if gotSport.SportType == nil || gotSport.SportType.ID != sportType.ID {
		t.Fatalf("SportInfo.SportType = %+v", gotSport.SportType)
	}

	var gotMsg entity.AIMessage
	if err := db.Preload("Conversation").First(&gotMsg, msg.ID).Error; err != nil {
		t.Fatalf("预加载会话失败: %v", err)
	}
	if gotMsg.Conversation == nil || gotMsg.Conversation.ID != conv.ID {
		t.Fatalf("AIMessage.Conversation = %+v", gotMsg.Conversation)
	}
	if msg.CreatedAt.IsZero() || record.CreatedAt.IsZero() || profile.CreatedAt.IsZero() {
		t.Fatal("autoCreateTime 未生效")
	}
}

func TestPasswordHashNotJSON(t *testing.T) {
	hash := "secret-bcrypt-hash"
	raw, err := json.Marshal(&entity.SysUser{ID: 1, Username: "json-user", PasswordHash: &hash})
	if err != nil {
		t.Fatalf("序列化用户失败: %v", err)
	}
	if strings.Contains(string(raw), hash) || strings.Contains(string(raw), "password") {
		t.Fatalf("密码哈希不应出现在 JSON: %s", raw)
	}
}

func TestUserSoftDelete(t *testing.T) {
	db := openTestDB(t)

	user := createUser(t, db, "bob")
	profile := createProfile(t, db, user.ID)
	record := &entity.HealthRecord{UserID: user.ID, MeasuredAt: time.Now().UTC(), Source: entity.SourceManual}
	mustCreate(t, db, record)
	oauth := &entity.SysOauthIdentity{UserID: user.ID, Provider: "wechat", OpenID: "openid-bob"}
	mustCreate(t, db, oauth)
	conv := createConversation(t, db, user.ID)
	msg := &entity.AIMessage{ConversationID: conv.ID, MessageRole: entity.RoleUser, Content: "hi", SequenceNo: 1}
	mustCreate(t, db, msg)

	if err := db.Delete(user).Error; err != nil {
		t.Fatalf("软删除用户失败: %v", err)
	}

	if err := db.First(&entity.SysUser{}, user.ID).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("软删除后普通查询应返回 ErrRecordNotFound, got %v", err)
	}
	if got := countRows(t, db, &entity.SysUser{}); got != 0 {
		t.Fatalf("软删除后用户数 = %d, want 0", got)
	}

	var deleted entity.SysUser
	if err := db.Unscoped().First(&deleted, user.ID).Error; err != nil {
		t.Fatalf("Unscoped 查询软删除用户失败: %v", err)
	}
	if !deleted.DeletedAt.Valid {
		t.Fatal("DeletedAt 未标记")
	}

	// 软删除不会触发数据库级联：关联数据仍保留。
	assertCount(t, db, &entity.HealthProfile{}, 1)
	assertCount(t, db, &entity.HealthRecord{}, 1)
	assertCount(t, db, &entity.SysOauthIdentity{}, 1)
	assertCount(t, db, &entity.AIConversation{}, 1)

	// 物理删除用户，由 SQL 外键级联清理关联数据。
	if err := db.Unscoped().Delete(user).Error; err != nil {
		t.Fatalf("物理删除用户失败: %v", err)
	}
	assertCount(t, db.Unscoped(), &entity.SysUser{}, 0)
	assertCount(t, db.Unscoped(), &entity.HealthProfile{}, 0)
	assertCount(t, db.Unscoped(), &entity.HealthRecord{}, 0)
	assertCount(t, db.Unscoped(), &entity.SysOauthIdentity{}, 0)
	assertCount(t, db.Unscoped(), &entity.AIConversation{}, 0)
	assertCount(t, db.Unscoped(), &entity.AIMessage{}, 0)

	_ = profile
}

func TestHealthProfileOneToOne(t *testing.T) {
	db := openTestDB(t)

	user := createUser(t, db, "carol")
	profile := createProfile(t, db, user.ID)
	profile.HeightCm = dec("165.20")
	profile.WeightKg = dec("55.40")
	if err := db.Save(profile).Error; err != nil {
		t.Fatalf("更新健康档案失败: %v", err)
	}

	var loadedUser entity.SysUser
	if err := db.Preload("Profile").First(&loadedUser, user.ID).Error; err != nil {
		t.Fatalf("预加载 Profile 失败: %v", err)
	}
	if loadedUser.Profile == nil || loadedUser.Profile.ID != profile.ID {
		t.Fatalf("SysUser.Profile = %+v", loadedUser.Profile)
	}

	var loadedProfile entity.HealthProfile
	if err := db.Preload("User").First(&loadedProfile, profile.ID).Error; err != nil {
		t.Fatalf("预加载 User 失败: %v", err)
	}
	if loadedProfile.User == nil || loadedProfile.User.ID != user.ID {
		t.Fatalf("HealthProfile.User = %+v", loadedProfile.User)
	}

	// health_profile.user_id 唯一，禁止一人多档。
	dup := &entity.HealthProfile{UserID: user.ID}
	if err := db.Create(dup).Error; mysqlErrCode(err) != 1062 {
		t.Fatalf("重复健康档案错误码 = %d, want 1062 (err=%v)", mysqlErrCode(err), err)
	}

	// HealthProfile.Records 通过非主键 user_id 关联历史记录。
	for i := 0; i < 2; i++ {
		rec := &entity.HealthRecord{
			UserID:     user.ID,
			MeasuredAt: time.Date(2026, 11, 20, 8+i, 0, 0, 0, time.UTC),
			Source:     entity.SourceManual,
		}
		mustCreate(t, db, rec)
	}
	var withRecords entity.HealthProfile
	if err := db.Preload("Records").First(&withRecords, profile.ID).Error; err != nil {
		t.Fatalf("预加载 Records 失败: %v", err)
	}
	if len(withRecords.Records) != 2 {
		t.Fatalf("HealthProfile.Records 数量 = %d, want 2", len(withRecords.Records))
	}
}

func TestHealthRecordOneToMany(t *testing.T) {
	db := openTestDB(t)

	user := createUser(t, db, "dave")
	profile := createProfile(t, db, user.ID)
	base := time.Date(2026, 11, 18, 7, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		rec := &entity.HealthRecord{
			UserID:       user.ID,
			WeightKg:     dec("70.00"),
			MeasuredAt:   base.Add(time.Duration(i) * 24 * time.Hour).Add(123 * time.Millisecond),
			Source:       entity.SourceManual,
			DailyWaterMl: ptr(uint32(1500 + i*100)),
		}
		mustCreate(t, db, rec)
	}

	var records []entity.HealthRecord
	if err := db.Preload("User").Where("user_id = ?", user.ID).Order("measured_at DESC, id DESC").Find(&records).Error; err != nil {
		t.Fatalf("查询健康记录失败: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("健康记录数量 = %d, want 3", len(records))
	}
	for i := 1; i < len(records); i++ {
		if records[i].MeasuredAt.After(records[i-1].MeasuredAt) {
			t.Fatalf("记录未按 measured_at DESC 排序: %v", records)
		}
	}
	if records[0].User == nil || records[0].User.ID != user.ID {
		t.Fatalf("记录未预加载 User: %+v", records[0].User)
	}
	if records[0].MeasuredAt.Location() != time.UTC {
		t.Fatalf("measured_at 时区 = %v, want UTC", records[0].MeasuredAt.Location())
	}
	if delta := records[0].MeasuredAt.Nanosecond() % int(time.Second); delta != 123_000_000 {
		t.Fatalf("DATETIME(3) 毫秒精度丢失: %v", records[0].MeasuredAt)
	}

	count := db.Model(user).Association("HealthRecords").Count()
	if count != 3 {
		t.Fatalf("Association HealthRecords = %d, want 3", count)
	}
	count = db.Model(profile).Association("Records").Count()
	if count != 3 {
		t.Fatalf("Association Profile.Records = %d, want 3", count)
	}
}

func TestDecimalPrecision(t *testing.T) {
	db := openTestDB(t)

	user := createUser(t, db, "erin")
	profile := &entity.HealthProfile{
		UserID:               user.ID,
		HeightCm:             dec("175.50"),
		WeightKg:             dec("70.10"),
		BloodSugarMmol:       dec("5.60"),
		TotalCholesterolMmol: dec("4.35"),
		VisionLeft:           dec("1.50"),
		SleepDurationHours:   dec("7.25"),
	}
	mustCreate(t, db, profile)

	var h, w, b, c, v, s string
	row := db.Raw("SELECT CAST(height_cm AS CHAR), CAST(weight_kg AS CHAR), CAST(blood_sugar_mmol AS CHAR), CAST(total_cholesterol_mmol AS CHAR), CAST(vision_left AS CHAR), CAST(sleep_duration_hours AS CHAR) FROM health_profile WHERE id = ?", profile.ID).Row()
	if err := row.Scan(&h, &w, &b, &c, &v, &s); err != nil {
		t.Fatalf("读取 DECIMAL 原始值失败: %v", err)
	}
	if h != "175.50" || w != "70.10" || b != "5.60" || c != "4.35" || v != "1.50" || s != "7.25" {
		t.Fatalf("DECIMAL 精度不匹配: h=%s w=%s b=%s c=%s v=%s s=%s", h, w, b, c, v, s)
	}

	var got entity.HealthProfile
	if err := db.First(&got, profile.ID).Error; err != nil {
		t.Fatalf("读取健康档案失败: %v", err)
	}
	if got.WeightKg == nil || got.WeightKg.StringFixed(2) != "70.10" {
		t.Fatalf("WeightKg = %v, want 70.10", got.WeightKg)
	}
	if got.VisionLeft == nil || got.VisionLeft.StringFixed(2) != "1.50" {
		t.Fatalf("VisionLeft = %v, want 1.50", got.VisionLeft)
	}

	if err := db.Model(&got).Update("weight_kg", dec("70.15")).Error; err != nil {
		t.Fatalf("更新 DECIMAL 失败: %v", err)
	}
	var updated string
	if err := db.Raw("SELECT CAST(weight_kg AS CHAR) FROM health_profile WHERE id = ?", profile.ID).Scan(&updated).Error; err != nil {
		t.Fatalf("读取更新后 DECIMAL 失败: %v", err)
	}
	if updated != "70.15" {
		t.Fatalf("更新后 weight_kg = %q, want 70.15", updated)
	}

	if err := db.Model(&got).Update("height_cm", nil).Error; err != nil {
		t.Fatalf("置空 DECIMAL 失败: %v", err)
	}
	var isNull bool
	if err := db.Raw("SELECT height_cm IS NULL FROM health_profile WHERE id = ?", profile.ID).Scan(&isNull).Error; err != nil {
		t.Fatalf("检查 DECIMAL NULL 失败: %v", err)
	}
	if !isNull {
		t.Fatal("height_cm 置空失败")
	}
}

func TestNullableFields(t *testing.T) {
	db := openTestDB(t)

	user := createUser(t, db, "frank")
	var email, phone, nickname sql.NullString
	row := db.Raw("SELECT email, phone, nickname FROM sys_user WHERE id = ?", user.ID).Row()
	if err := row.Scan(&email, &phone, &nickname); err != nil {
		t.Fatalf("读取 NULL 字段失败: %v", err)
	}
	if email.Valid || phone.Valid || nickname.Valid {
		t.Fatalf("可空字段应为 NULL: email=%v phone=%v nickname=%v", email, phone, nickname)
	}

	if err := db.Model(user).Updates(map[string]any{
		"email":    "frank@example.com",
		"phone":    "13800000000",
		"nickname": "Frank",
	}).Error; err != nil {
		t.Fatalf("写入可空字段失败: %v", err)
	}
	var loaded entity.SysUser
	if err := db.First(&loaded, user.ID).Error; err != nil {
		t.Fatalf("重新读取用户失败: %v", err)
	}
	if loaded.Email == nil || *loaded.Email != "frank@example.com" {
		t.Fatalf("Email = %v", loaded.Email)
	}
	if loaded.Phone == nil || *loaded.Phone != "13800000000" {
		t.Fatalf("Phone = %v", loaded.Phone)
	}

	if err := db.Model(user).Update("email", nil).Error; err != nil {
		t.Fatalf("置空 email 失败: %v", err)
	}
	if err := db.First(&loaded, user.ID).Error; err != nil {
		t.Fatalf("重新读取用户失败: %v", err)
	}
	if loaded.Email != nil {
		t.Fatalf("Email 应为 nil, got %v", loaded.Email)
	}

	rec := &entity.HealthRecord{
		UserID:     user.ID,
		MeasuredAt: time.Date(2026, 11, 21, 9, 0, 0, 0, time.UTC),
		Source:     entity.SourceImport,
	}
	mustCreate(t, db, rec)
	var weight sql.NullString
	var systolic sql.NullInt64
	var remark sql.NullString
	row = db.Raw("SELECT weight_kg, systolic_pressure, remark FROM health_record WHERE id = ?", rec.ID).Row()
	if err := row.Scan(&weight, &systolic, &remark); err != nil {
		t.Fatalf("读取记录 NULL 字段失败: %v", err)
	}
	if weight.Valid || systolic.Valid || remark.Valid {
		t.Fatalf("记录的数值/备注应为 NULL: weight=%v systolic=%v remark=%v", weight, systolic, remark)
	}

	if err := db.Model(rec).Updates(map[string]any{
		"weight_kg":         dec("68.20"),
		"systolic_pressure": uint16(118),
		"remark":            "复测",
	}).Error; err != nil {
		t.Fatalf("写入记录可空字段失败: %v", err)
	}
	var loadedRec entity.HealthRecord
	if err := db.First(&loadedRec, rec.ID).Error; err != nil {
		t.Fatalf("重新读取记录失败: %v", err)
	}
	if loadedRec.WeightKg == nil || loadedRec.WeightKg.StringFixed(2) != "68.20" {
		t.Fatalf("WeightKg = %v", loadedRec.WeightKg)
	}
	if loadedRec.SystolicPressure == nil || *loadedRec.SystolicPressure != 118 {
		t.Fatalf("SystolicPressure = %v", loadedRec.SystolicPressure)
	}
	if loadedRec.Remark == nil || *loadedRec.Remark != "复测" {
		t.Fatalf("Remark = %v", loadedRec.Remark)
	}

	if err := db.Model(rec).Updates(map[string]any{
		"weight_kg":         nil,
		"systolic_pressure": nil,
		"remark":            nil,
	}).Error; err != nil {
		t.Fatalf("置空记录字段失败: %v", err)
	}
	// GORM 的 *decimal.Decimal 指针扫描器在数据库值为 NULL 且目标字段已有非 nil 值时
	// 会提前返回、保留旧值；查询必须使用新的零值目标，避免脏读。
	var cleared entity.HealthRecord
	if err := db.First(&cleared, rec.ID).Error; err != nil {
		t.Fatalf("重新读取记录失败: %v", err)
	}
	if cleared.WeightKg != nil || cleared.SystolicPressure != nil || cleared.Remark != nil {
		t.Fatalf("记录字段应为 nil: %+v", cleared)
	}

	profile := createProfile(t, db, user.ID)
	birth := time.Date(1990, 5, 20, 0, 0, 0, 0, time.UTC)
	if err := db.Model(profile).Update("birth_date", birth).Error; err != nil {
		t.Fatalf("写入 birth_date 失败: %v", err)
	}
	var loadedProfile entity.HealthProfile
	if err := db.First(&loadedProfile, profile.ID).Error; err != nil {
		t.Fatalf("重新读取档案失败: %v", err)
	}
	if loadedProfile.BirthDate == nil || loadedProfile.BirthDate.Format("2006-01-02") != "1990-05-20" {
		t.Fatalf("BirthDate = %v", loadedProfile.BirthDate)
	}

	longText := strings.Repeat("a", 5000)
	if err := db.Model(profile).Update("food_preferences", longText).Error; err != nil {
		t.Fatalf("写入 TEXT 字段失败: %v", err)
	}
	if err := db.First(&loadedProfile, profile.ID).Error; err != nil {
		t.Fatalf("重新读取档案失败: %v", err)
	}
	if loadedProfile.FoodPreferences == nil || len(*loadedProfile.FoodPreferences) != len(longText) {
		t.Fatalf("TEXT 字段长度不匹配: %v", loadedProfile.FoodPreferences)
	}
}

func TestAIConversationMessages(t *testing.T) {
	db := openTestDB(t)

	user := createUser(t, db, "grace")
	conv := createConversation(t, db, user.ID)
	mustCreate(t, db, &entity.AIMessage{
		ConversationID: conv.ID,
		MessageRole:    entity.RoleUser,
		Content:        "血压高怎么办",
		SequenceNo:     1,
	})
	mustCreate(t, db, &entity.AIMessage{
		ConversationID: conv.ID,
		MessageRole:    entity.RoleAssistant,
		Content:        "建议监测并咨询医生",
		ModelName:      ptr("deepseek-chat"),
		TokenCount:     ptr(uint32(128)),
		SequenceNo:     2,
	})
	mustCreate(t, db, &entity.AIMessage{
		ConversationID: conv.ID,
		MessageRole:    entity.RoleTool,
		Content:        "{\"ok\":true}",
		SequenceNo:     3,
	})

	longContent := strings.Repeat("x", 65535)
	mustCreate(t, db, &entity.AIMessage{
		ConversationID: conv.ID,
		MessageRole:    entity.RoleAssistant,
		Content:        longContent,
		SequenceNo:     4,
	})

	var loaded entity.AIConversation
	if err := db.Preload("Messages", func(db *gorm.DB) *gorm.DB {
		return db.Order("sequence_no ASC")
	}).First(&loaded, conv.ID).Error; err != nil {
		t.Fatalf("预加载消息失败: %v", err)
	}
	if len(loaded.Messages) != 4 {
		t.Fatalf("消息数量 = %d, want 4", len(loaded.Messages))
	}
	for i, msg := range loaded.Messages {
		if msg.SequenceNo != uint32(i+1) {
			t.Fatalf("消息顺序错误: %+v", loaded.Messages)
		}
	}
	if len(loaded.Messages[3].Content) != len(longContent) {
		t.Fatalf("MEDIUMTEXT 内容长度 = %d, want %d", len(loaded.Messages[3].Content), len(longContent))
	}
	if loaded.Messages[1].ModelName == nil || *loaded.Messages[1].ModelName != "deepseek-chat" {
		t.Fatalf("ModelName = %v", loaded.Messages[1].ModelName)
	}
	if loaded.Messages[1].TokenCount == nil || *loaded.Messages[1].TokenCount != 128 {
		t.Fatalf("TokenCount = %v", loaded.Messages[1].TokenCount)
	}

	dup := &entity.AIMessage{ConversationID: conv.ID, MessageRole: entity.RoleUser, Content: "dup", SequenceNo: 1}
	if err := db.Create(dup).Error; mysqlErrCode(err) != 1062 {
		t.Fatalf("重复 sequence_no 错误码 = %d, want 1062 (err=%v)", mysqlErrCode(err), err)
	}

	var loadedMsg entity.AIMessage
	if err := db.Preload("Conversation").Where("conversation_id = ?", conv.ID).Order("sequence_no ASC").First(&loadedMsg).Error; err != nil {
		t.Fatalf("查询消息失败: %v", err)
	}
	if loadedMsg.Conversation == nil || loadedMsg.Conversation.ID != conv.ID {
		t.Fatalf("AIMessage.Conversation = %+v", loadedMsg.Conversation)
	}

	count := db.Model(conv).Association("Messages").Count()
	if count != 4 {
		t.Fatalf("Association Messages = %d, want 4", count)
	}

	if err := db.Delete(conv).Error; err != nil {
		t.Fatalf("软删除会话失败: %v", err)
	}
	if err := db.First(&entity.AIConversation{}, conv.ID).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("软删除后会话应不可见, got %v", err)
	}
	if got := countRows(t, db, &entity.AIMessage{}); got != 4 {
		t.Fatalf("软删除会话后消息数 = %d, want 4", got)
	}

	if err := db.Unscoped().Delete(conv).Error; err != nil {
		t.Fatalf("物理删除会话失败: %v", err)
	}
	assertCount(t, db.Unscoped(), &entity.AIMessage{}, 0)
}
