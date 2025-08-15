package pgconv

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
)

func TestUUIDConversions(t *testing.T) {
	t.Run("UUIDToPg and PgToUUID roundtrip", func(t *testing.T) {
		original := uuid.New()
		pg := UUIDToPg(original)
		converted := PgToUUID(pg)

		assert.True(t, pg.Valid)
		assert.Equal(t, original, converted)
	})

	t.Run("PgToUUID with invalid UUID", func(t *testing.T) {
		pg := pgtype.UUID{Valid: false}
		converted := PgToUUID(pg)

		assert.Equal(t, uuid.Nil, converted)
	})

	t.Run("UUIDPtrToPg with nil pointer", func(t *testing.T) {
		pg := UUIDPtrToPg(nil)

		assert.False(t, pg.Valid)
	})

	t.Run("UUIDPtrToPg with valid pointer", func(t *testing.T) {
		original := uuid.New()
		pg := UUIDPtrToPg(&original)
		converted := PgToUUIDPtr(pg)

		assert.True(t, pg.Valid)
		assert.NotNil(t, converted)
		assert.Equal(t, original, *converted)
	})

	t.Run("PgToUUIDPtr with invalid UUID", func(t *testing.T) {
		pg := pgtype.UUID{Valid: false}
		converted := PgToUUIDPtr(pg)

		assert.Nil(t, converted)
	})
}

func TestTimeConversions(t *testing.T) {
	t.Run("TimeToPg and PgToTime roundtrip", func(t *testing.T) {
		original := time.Now().UTC().Truncate(time.Microsecond)
		pg := TimeToPg(original)
		converted := PgToTime(pg)

		assert.True(t, pg.Valid)
		assert.True(t, original.Equal(converted))
	})

	t.Run("PgToTime with invalid time", func(t *testing.T) {
		pg := pgtype.Timestamptz{Valid: false}
		converted := PgToTime(pg)

		assert.True(t, converted.IsZero())
	})

	t.Run("TimePtrToPg with nil pointer", func(t *testing.T) {
		pg := TimePtrToPg(nil)

		assert.False(t, pg.Valid)
	})

	t.Run("TimePtrToPg with valid pointer", func(t *testing.T) {
		original := time.Now().UTC().Truncate(time.Microsecond)
		pg := TimePtrToPg(&original)
		converted := PgToTimePtr(pg)

		assert.True(t, pg.Valid)
		assert.NotNil(t, converted)
		assert.True(t, original.Equal(*converted))
	})

	t.Run("PgToTimePtr with invalid time", func(t *testing.T) {
		pg := pgtype.Timestamptz{Valid: false}
		converted := PgToTimePtr(pg)

		assert.Nil(t, converted)
	})
}

func TestStringConversions(t *testing.T) {
	t.Run("StringToPg and PgToString roundtrip", func(t *testing.T) {
		original := "hello world"
		pg := StringToPg(original)
		converted := PgToString(pg)

		assert.True(t, pg.Valid)
		assert.Equal(t, original, converted)
	})

	t.Run("StringToPg with empty string", func(t *testing.T) {
		original := ""
		pg := StringToPg(original)
		converted := PgToString(pg)

		assert.True(t, pg.Valid)
		assert.Equal(t, original, converted)
	})

	t.Run("PgToString with invalid text", func(t *testing.T) {
		pg := pgtype.Text{Valid: false}
		converted := PgToString(pg)

		assert.Equal(t, "", converted)
	})

	t.Run("StringPtrToPg with nil pointer", func(t *testing.T) {
		pg := StringPtrToPg(nil)

		assert.False(t, pg.Valid)
	})

	t.Run("StringPtrToPg with valid pointer", func(t *testing.T) {
		original := "test string"
		pg := StringPtrToPg(&original)
		converted := PgToStringPtr(pg)

		assert.True(t, pg.Valid)
		assert.NotNil(t, converted)
		assert.Equal(t, original, *converted)
	})

	t.Run("PgToStringPtr with invalid text", func(t *testing.T) {
		pg := pgtype.Text{Valid: false}
		converted := PgToStringPtr(pg)

		assert.Nil(t, converted)
	})
}

func TestIntConversions(t *testing.T) {
	t.Run("Int64ToPg and PgToInt64 roundtrip", func(t *testing.T) {
		original := int64(1234567890)
		pg := Int64ToPg(original)
		converted := PgToInt64(pg)

		assert.True(t, pg.Valid)
		assert.Equal(t, original, converted)
	})

	t.Run("PgToInt64 with invalid int8", func(t *testing.T) {
		pg := pgtype.Int8{Valid: false}
		converted := PgToInt64(pg)

		assert.Equal(t, int64(0), converted)
	})

	t.Run("Int32ToPg and PgToInt32 roundtrip", func(t *testing.T) {
		original := int32(123456)
		pg := Int32ToPg(original)
		converted := PgToInt32(pg)

		assert.True(t, pg.Valid)
		assert.Equal(t, original, converted)
	})

	t.Run("PgToInt32 with invalid int4", func(t *testing.T) {
		pg := pgtype.Int4{Valid: false}
		converted := PgToInt32(pg)

		assert.Equal(t, int32(0), converted)
	})
}
