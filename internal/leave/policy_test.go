package leave

import (
	"errors"
	"testing"
	"time"

	"hris/backend/internal/httputil"
)

func TestParseDateRangeUsesInclusiveCalendarDays(t *testing.T) {
	location, _ := time.LoadLocation("America/New_York")
	_, _, duration, err := parseDateRange("2026-03-07", "2026-03-09", location)
	if err != nil {
		t.Fatal(err)
	}
	if duration != 3 {
		t.Fatalf("want 3 inclusive calendar days across DST, got %v", duration)
	}
}

func TestParseDateRangeRejectsCrossYear(t *testing.T) {
	_, _, _, err := parseDateRange("2026-12-31", "2027-01-01", time.UTC)
	var domainErr *httputil.DomainError
	if !errors.As(err, &domainErr) || domainErr.Code != "CROSS_YEAR_LEAVE_NOT_SUPPORTED" {
		t.Fatalf("want cross-year domain error, got %v", err)
	}
}

func TestParseDateRangeRejectsReverseRange(t *testing.T) {
	_, _, _, err := parseDateRange("2026-09-02", "2026-09-01", time.UTC)
	var domainErr *httputil.DomainError
	if !errors.As(err, &domainErr) || domainErr.Code != "INVALID_DATE_RANGE" {
		t.Fatalf("want invalid range domain error, got %v", err)
	}
}
