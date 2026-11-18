package services

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"e-absensi-backend/models"
	"e-absensi-backend/utils"
)

func TestGenerateScheduleTemplateProducesValidXLSX(t *testing.T) {
	svc := &ScheduleService{now: time.Now}
	data, err := svc.GenerateTemplate(context.Background())
	require.NoError(t, err)

	rows, err := utils.ReadXLSX(data)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, scheduleTemplateHeaders, rows[0])
	assert.True(t, isValidScheduleHeader(rows[0]))
}

func TestResolveDateRangeDefaultsToCurrentWeekAndValidatesOrder(t *testing.T) {
	fixedNow := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC) // Rabu
	svc := &ScheduleService{now: func() time.Time { return fixedNow }}

	from, to, err := svc.resolveDateRange("", "")
	require.NoError(t, err)
	assert.Equal(t, "2026-11-16", from.Format(dateFormatYYYYMMDD)) // Senin
	assert.Equal(t, "2026-11-22", to.Format(dateFormatYYYYMMDD))   // Minggu

	_, _, err = svc.resolveDateRange("2026-11-20", "2026-11-18")
	require.Error(t, err)
	appErr := asAppError(err)
	require.NotNil(t, appErr)
	assert.Equal(t, utils.CodeValidation, appErr.Code)
}

func TestToScheduleResponseHidesSubjectWhenActivity(t *testing.T) {
	actName := "Upacara Bendera"
	sched := &models.Schedule{
		ID:           10,
		ScheduleDate: time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC),
		StartTime:    "07:00:00",
		EndTime:      "08:00:00",
		ClassID:      1,
		SubjectID:    5,
		TeacherID:    3,
		ScheduleType: models.ScheduleTypeActivity,
		ActivityName: &actName,
		Subject:      &models.Subject{ID: 5, Code: "MTK-10", Name: "Matematika", IsActive: true},
	}

	resp := toScheduleResponse(sched)
	assert.Equal(t, models.ScheduleTypeActivity, resp.Type)
	assert.Nil(t, resp.SubjectID)
	assert.Nil(t, resp.Subject)
	assert.Equal(t, int64(3), resp.TeacherID)
	assert.Equal(t, "07:00", resp.StartTime)
	assert.Equal(t, "08:00", resp.EndTime)
}

func TestSameTimestampMicrosecondPrecision(t *testing.T) {
	t1 := time.Date(2026, 11, 17, 15, 0, 0, 123456000, time.UTC)
	t2 := time.Date(2026, 11, 17, 15, 0, 0, 123456789, time.UTC)
	t3 := time.Date(2026, 11, 17, 15, 0, 1, 123456000, time.UTC)

	assert.True(t, sameTimestamp(t1, t2))
	assert.False(t, sameTimestamp(t1, t3))
}
