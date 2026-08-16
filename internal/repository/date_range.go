package repository

import (
	"fmt"

	"github.com/atlas/knowledge-api/internal/domain"
)

// AppTimeZone is the calendar timezone used for date filters and displayed dates.
const AppTimeZone = "America/Sao_Paulo"

// dateRangeSQL compares a timestamptz column by calendar day in AppTimeZone,
// so a project created at 00:30 in Brazil on July 1 is not treated as June 30 UTC.
func dateRangeSQL(column string, period domain.DateRange, startIdx int) (clause string, args []interface{}, nextIdx int) {
	clause = fmt.Sprintf(
		"(%s AT TIME ZONE '%s')::date >= $%d::date AND (%s AT TIME ZONE '%s')::date <= $%d::date",
		column, AppTimeZone, startIdx,
		column, AppTimeZone, startIdx+1,
	)
	// Datas como string evitam deslocamento de fuso ao enviar o intervalo.
	return clause, []interface{}{
		period.From.Format("2006-01-02"),
		period.To.Format("2006-01-02"),
	}, startIdx + 2
}
