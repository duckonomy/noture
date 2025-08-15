package pgconv

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func UUIDToPg(u uuid.UUID) pgtype.UUID {
	return pgtype.UUID{
		Bytes: u,
		Valid: true,
	}
}

func PgToUUID(pg pgtype.UUID) uuid.UUID {
	if !pg.Valid {
		return uuid.Nil
	}
	return pg.Bytes
}

func UUIDPtrToPg(u *uuid.UUID) pgtype.UUID {
	if u == nil {
		return pgtype.UUID{Valid: false}
	}
	return pgtype.UUID{
		Bytes: *u,
		Valid: true,
	}
}

func PgToUUIDPtr(pg pgtype.UUID) *uuid.UUID {
	if !pg.Valid {
		return nil
	}
	u := uuid.UUID(pg.Bytes)
	return &u
}

func TimeToPg(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{
		Time:  t,
		Valid: true,
	}
}

func PgToTime(pg pgtype.Timestamptz) time.Time {
	if !pg.Valid {
		return time.Time{}
	}
	return pg.Time
}

func TimePtrToPg(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{Valid: false}
	}
	return pgtype.Timestamptz{
		Time:  *t,
		Valid: true,
	}
}

func PgToTimePtr(pg pgtype.Timestamptz) *time.Time {
	if !pg.Valid {
		return nil
	}
	return &pg.Time
}

func StringToPg(s string) pgtype.Text {
	return pgtype.Text{
		String: s,
		Valid:  true,
	}
}

func PgToString(pg pgtype.Text) string {
	if !pg.Valid {
		return ""
	}
	return pg.String
}

func StringPtrToPg(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{
		String: *s,
		Valid:  true,
	}
}

func PgToStringPtr(pg pgtype.Text) *string {
	if !pg.Valid {
		return nil
	}
	return &pg.String
}

func Int64ToPg(i int64) pgtype.Int8 {
	return pgtype.Int8{
		Int64: i,
		Valid: true,
	}
}

func PgToInt64(pg pgtype.Int8) int64 {
	if !pg.Valid {
		return 0
	}
	return pg.Int64
}

func Int32ToPg(i int32) pgtype.Int4 {
	return pgtype.Int4{
		Int32: i,
		Valid: true,
	}
}

func PgToInt32(pg pgtype.Int4) int32 {
	if !pg.Valid {
		return 0
	}
	return pg.Int32
}
