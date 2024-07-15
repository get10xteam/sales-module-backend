package status

import (
	"context"
	"time"

	"gitlab.com/intalko/gosuite/pgdb"

	"github.com/georgysavva/scany/v2/pgxscan"
)

type Status struct {
	Code        *string   `json:"code,omitempty" db:"code"`
	Name        *string   `json:"name,omitempty" db:"name"`
	Description *string   `json:"description,omitempty" db:"description"`
	CreateTs    time.Time `json:"createTs,omitempty" db:"create_ts"`
}

func StatusByCode(ctx context.Context, code string, cols ...string) (*Status, error) {
	if len(cols) == 0 {
		cols = []string{"code"}
	}

	r, err := pgdb.QbQuery(ctx, pgdb.Qb.Select(cols...).From("statuses").Where("code = ?", code))
	if err != nil {
		return nil, err
	}

	u := &Status{}
	err = pgxscan.ScanOne(u, r)
	if err != nil {
		return nil, err
	}
	return u, err
}
