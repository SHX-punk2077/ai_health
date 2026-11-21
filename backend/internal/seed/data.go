// Package seed 生成可复现的开发演示健康数据。
// 它只用于开发/演示数据库，绝不能连接生产库或测试库。
package seed

import (
	"math/rand"
	"time"

	"github.com/shopspring/decimal"

	"ai_health/internal/model/entity"
)

const (
	// DefaultAnchor 是默认的演示日期锚点，记录范围截止锚点前一天。
	DefaultAnchor = "2026-11-21"
	// DefaultRandomSeed 固定默认随机种子，保证同一锚点下可复现。
	DefaultRandomSeed int64 = 20261121
	// DemoPassword 仅用于本地开发演示账号，文档中会明确标注。
	DemoPassword = "Demo#2026"
)

// DemoUser 是一个演示用户的完整数据，UserID 在写入时按真实用户回填。
type DemoUser struct {
	Username string
	Nickname string
	Profile  *entity.HealthProfile
	Records  []entity.HealthRecord
}

// Plan 是一次 seed 的确定性数据计划。
type Plan struct {
	Anchor     time.Time
	RandomSeed int64
	Users      []DemoUser
}

// BuildPlan 生成三个演示用户的数据；同一 anchor + randomSeed 输出完全一致。
func BuildPlan(anchor time.Time, randomSeed int64) Plan {
	a := dateOnly(anchor)
	rng := rand.New(rand.NewSource(randomSeed))
	return Plan{
		Anchor:     a,
		RandomSeed: randomSeed,
		Users: []DemoUser{
			buildDemoA(a, rng),
			buildDemoB(a, rng),
			buildDemoC(a, rng),
		},
	}
}

func buildDemoA(anchor time.Time, rng *rand.Rand) DemoUser {
	profile := &entity.HealthProfile{
		RealName:             text("演示用户A"),
		Age:                  uint8Ptr(34),
		BirthDate:            birthDateForAge(anchor, 34),
		Gender:               uint8Ptr(1),
		HeightCm:             cents(17500),
		WeightKg:             cents(7050),
		BloodSugarMmol:       cents(520),
		SystolicPressure:     uint16Ptr(118),
		DiastolicPressure:    uint16Ptr(78),
		TotalCholesterolMmol: cents(460),
		HeartRate:            uint16Ptr(68),
		VisionLeft:           cents(100),
		VisionRight:          cents(110),
		SleepDurationHours:   cents(750),
		SleepQuality:         text("良好"),
		SmokingStatus:        uint8Ptr(0),
		DrinkingStatus:       uint8Ptr(0),
		ExerciseHabit:        uint8Ptr(2),
		FoodPreferences:      text("均衡饮食"),
		DailyWaterMl:         uint32Ptr(1800),
		MedicalHistory:       text("演示数据，无真实病史"),
		AllergyHistory:       text("演示数据，无真实过敏史"),
		CreatedAt:            anchor.AddDate(0, 0, -120),
		UpdatedAt:            anchor.AddDate(0, 0, -1),
	}

	records := make([]entity.HealthRecord, 0, 96)
	for i := 0; i < 90; i++ {
		// 90 天窗口内留出少量空档，模拟真实使用中的漏记。
		if i%19 == 7 {
			continue
		}
		dayOffset := i - 90 // -90 .. -1，确保默认锚点不会生成未来测量时间
		at := dayAt(anchor, dayOffset, 7, 30)
		rec := entity.HealthRecord{
			WeightKg:           cents(int64(7050 + rng.Intn(160) - 80 - i*2)),
			BloodSugarMmol:     cents(int64(520 + rng.Intn(41) - 20)),
			SystolicPressure:   uint16Ptr(uint16(116 + rng.Intn(9))),
			DiastolicPressure:  uint16Ptr(uint16(76 + rng.Intn(7))),
			HeartRate:          uint16Ptr(uint16(66 + rng.Intn(9))),
			SleepDurationHours: cents(int64(720 + rng.Intn(61) - 30)),
			DailyWaterMl:       uint32Ptr(uint32(1600 + rng.Intn(401))),
			MeasuredAt:         at,
			Source:             sourceFor(i),
			CreatedAt:          at,
			UpdatedAt:          at,
		}
		if i%30 == 0 {
			rec.HeightCm = cents(17500)
		}
		if i%6 == 0 {
			rec.BloodSugarMmol = nil
		}
		if i%7 == 0 {
			rec.SystolicPressure = nil
			rec.DiastolicPressure = nil
		}
		if i%5 == 0 {
			rec.SleepDurationHours = nil
		}
		if i%9 == 0 {
			rec.HeartRate = nil
		}
		records = append(records, rec)

		if i == 20 || i == 55 {
			secondAt := dayAt(anchor, dayOffset, 19, 40)
			records = append(records, entity.HealthRecord{
				WeightKg:          cents(int64(7090 + rng.Intn(120) - 60)),
				SystolicPressure:  uint16Ptr(uint16(118 + rng.Intn(7))),
				DiastolicPressure: uint16Ptr(uint16(77 + rng.Intn(5))),
				HeartRate:         uint16Ptr(uint16(70 + rng.Intn(7))),
				MeasuredAt:        secondAt,
				Source:            entity.SourceDevice,
				Remark:            text("晚间复测（演示数据）"),
				CreatedAt:         secondAt,
				UpdatedAt:         secondAt,
			})
		}
	}

	// 同一天多次测量。
	sameDay := dayAt(anchor, -10, 21, 0)
	records = append(records, entity.HealthRecord{
		WeightKg:           cents(7180),
		BloodSugarMmol:     cents(840),
		SystolicPressure:   uint16Ptr(146),
		DiastolicPressure:  uint16Ptr(92),
		HeartRate:          uint16Ptr(102),
		SleepDurationHours: cents(520),
		MeasuredAt:         sameDay,
		Source:             entity.SourceDevice,
		Remark:             text("演示：需要关注的测量值，不构成诊断"),
		CreatedAt:          sameDay,
		UpdatedAt:          sameDay,
	})

	// 同一测量时间的两条记录，用于验证 measured_at DESC、id DESC 的稳定排序。
	tieAt := dayAt(anchor, -31, 8, 0)
	for _, w := range []int64{7010, 7040} {
		records = append(records, entity.HealthRecord{
			WeightKg:   cents(w),
			MeasuredAt: tieAt,
			Source:     entity.SourceDevice,
			Remark:     text("同一时间重复测量（演示数据）"),
			CreatedAt:  tieAt,
			UpdatedAt:  tieAt,
		})
	}

	// 历史补录：测量时间早于 90 天窗口，录入时间晚于测量时间。
	backfillAt := dayAt(anchor, -120, 8, 0)
	records = append(records, entity.HealthRecord{
		HeightCm:          cents(17500),
		WeightKg:          cents(7120),
		BloodSugarMmol:    cents(530),
		SystolicPressure:  uint16Ptr(120),
		DiastolicPressure: uint16Ptr(80),
		MeasuredAt:        backfillAt,
		Source:            entity.SourceImport,
		Remark:            text("历史补录（演示数据）"),
		CreatedAt:         anchor.AddDate(0, 0, -2),
		UpdatedAt:         anchor.AddDate(0, 0, -2),
	})

	// 最新记录只包含体重，用于验证汇总接口不会因为最新记录缺指标而丢失更早的血压等数据。
	latestAt := dayAt(anchor, -1, 8, 15)
	records = append(records, entity.HealthRecord{
		WeightKg:   cents(7040),
		MeasuredAt: latestAt,
		Source:     entity.SourceManual,
		Remark:     text("最新体重（演示数据）"),
		CreatedAt:  latestAt,
		UpdatedAt:  latestAt,
	})

	return DemoUser{Username: "demo_user_a", Nickname: "演示用户A（正常）", Profile: profile, Records: records}
}

func buildDemoB(anchor time.Time, rng *rand.Rand) DemoUser {
	profile := &entity.HealthProfile{
		RealName:           text("演示用户B"),
		Age:                uint8Ptr(42),
		BirthDate:          birthDateForAge(anchor, 42),
		Gender:             uint8Ptr(2),
		HeightCm:           cents(16800),
		WeightKg:           cents(8200),
		SleepDurationHours: cents(600),
		DailyWaterMl:       uint32Ptr(1200),
		CreatedAt:          anchor.AddDate(0, 0, -90),
		UpdatedAt:          anchor.AddDate(0, 0, -1),
	}

	records := make([]entity.HealthRecord, 0, 12)
	for d := 30; d >= 1; d -= 3 {
		at := dayAt(anchor, -d, 9, 0)
		rec := entity.HealthRecord{
			WeightKg:   cents(int64(8150 + rng.Intn(121) - 60)),
			MeasuredAt: at,
			Source:     sourceFor(d),
			CreatedAt:  at,
			UpdatedAt:  at,
		}
		if d == 15 {
			rec.HeightCm = cents(16800)
			rec.SystolicPressure = uint16Ptr(126)
			rec.DiastolicPressure = uint16Ptr(82)
		}
		if d == 9 {
			rec.BloodSugarMmol = cents(820) // 偏高但属于合法数据，用于演示异常提醒场景
			rec.HeartRate = uint16Ptr(88)
		}
		records = append(records, rec)
	}
	latestAt := dayAt(anchor, -1, 9, 0)
	records = append(records, entity.HealthRecord{
		WeightKg:   cents(8100),
		MeasuredAt: latestAt,
		Source:     entity.SourceManual,
		Remark:     text("最新体重（演示数据）"),
		CreatedAt:  latestAt,
		UpdatedAt:  latestAt,
	})

	return DemoUser{Username: "demo_user_b", Nickname: "演示用户B（数据不完整）", Profile: profile, Records: records}
}

func buildDemoC(anchor time.Time, rng *rand.Rand) DemoUser {
	records := make([]entity.HealthRecord, 0, 28)
	for d := 45; d >= 1; d -= 2 {
		at := dayAt(anchor, -d, 18, 30)
		rec := entity.HealthRecord{
			WeightKg:          cents(int64(6200 + rng.Intn(121) - 60)),
			SystolicPressure:  uint16Ptr(uint16(112 + rng.Intn(9))),
			DiastolicPressure: uint16Ptr(uint16(72 + rng.Intn(7))),
			HeartRate:         uint16Ptr(uint16(64 + rng.Intn(9))),
			MeasuredAt:        at,
			Source:            sourceFor(d),
			CreatedAt:         at,
			UpdatedAt:         at,
		}
		if d == 21 {
			// 偏慢心率，用于演示合法但需要关注的提醒场景。
			rec.HeartRate = uint16Ptr(55)
			rec.Remark = text("演示：心率偏慢，不构成诊断")
		}
		records = append(records, rec)
	}
	latestAt := dayAt(anchor, -1, 18, 45)
	records = append(records, entity.HealthRecord{
		WeightKg:   cents(6200),
		MeasuredAt: latestAt,
		Source:     entity.SourceManual,
		Remark:     text("无档案用户的最新体重（演示数据）"),
		CreatedAt:  latestAt,
		UpdatedAt:  latestAt,
	})
	return DemoUser{Username: "demo_user_c", Nickname: "演示用户C（无档案）", Records: records}
}

// birthDateForAge 以锚点为基准生成与年龄一致的生日，避免出现 34 岁但生日只有 120 天的矛盾数据。
func birthDateForAge(anchor time.Time, age int) *time.Time {
	return dayPtr(dateOnly(anchor).AddDate(-age, 0, 0))
}

func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func dayAt(anchor time.Time, dayOffset, hour, minute int) time.Time {
	return anchor.AddDate(0, 0, dayOffset).Add(time.Duration(hour)*time.Hour + time.Duration(minute)*time.Minute)
}

func sourceFor(i int) string {
	switch i % 13 {
	case 0:
		return entity.SourceDevice
	case 5:
		return entity.SourceImport
	case 9:
		return entity.SourceAI
	default:
		return entity.SourceManual
	}
}

func cents(v int64) *decimal.Decimal {
	d := decimal.New(v, -2)
	return &d
}

func text(s string) *string { return &s }

func uint8Ptr(v uint8) *uint8 { return &v }

func uint16Ptr(v uint16) *uint16 { return &v }

func uint32Ptr(v uint32) *uint32 { return &v }

func dayPtr(t time.Time) *time.Time {
	d := dateOnly(t)
	return &d
}
