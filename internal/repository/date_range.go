package repository

import (
	"fmt"

	"github.com/atlas/knowledge-api/internal/domain"
)

func dateRangeSQL(column string, period domain.DateRange, startIdx int) (clause string, args []interface{}, nextIdx int) {
	clause = fmt.Sprintf(
		"%s >= $%d::date AND %s < ($%d::date + interval '1 day')",
		column, startIdx, column, startIdx+1,
	)
	// Datas como string evitam deslocamento de fuso ao comparar com timestamptz.
	return clause, []interface{}{
		period.From.Format("2006-01-02"),
		period.To.Format("2006-01-02"),
	}, startIdx + 2
}
