package leave

import (
	"net/http"
	"time"

	"hris/backend/internal/httputil"
)

func parseDateRange(startRaw, endRaw string, location *time.Location) (time.Time, time.Time, float64, error) {
	start, err := time.ParseInLocation("2006-01-02", startRaw, location)
	if err != nil {
		return time.Time{}, time.Time{}, 0, httputil.NewDomainError(http.StatusUnprocessableEntity, "INVALID_START_DATE", "Start date must use YYYY-MM-DD")
	}
	end, err := time.ParseInLocation("2006-01-02", endRaw, location)
	if err != nil {
		return time.Time{}, time.Time{}, 0, httputil.NewDomainError(http.StatusUnprocessableEntity, "INVALID_END_DATE", "End date must use YYYY-MM-DD")
	}
	if end.Before(start) {
		return time.Time{}, time.Time{}, 0, httputil.NewDomainError(http.StatusUnprocessableEntity, "INVALID_DATE_RANGE", "End date cannot be earlier than start date")
	}
	if start.Year() != end.Year() {
		return time.Time{}, time.Time{}, 0, httputil.NewDomainError(http.StatusUnprocessableEntity, "CROSS_YEAR_LEAVE_NOT_SUPPORTED", "Leave request must be within one calendar year")
	}
	days := 0
	for cursor := start; !cursor.After(end); cursor = cursor.AddDate(0, 0, 1) {
		days++
	}
	return start, end, float64(days), nil
}
