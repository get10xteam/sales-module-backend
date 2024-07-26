package status

import (
	"context"
	"time"

	"gitlab.com/intalko/gosuite/pgdb"

	"github.com/Masterminds/squirrel"
	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/get10xteam/sales-module-backend/errs"
	"github.com/get10xteam/sales-module-backend/plumbings/config"
	"github.com/get10xteam/sales-module-backend/plumbings/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type Status struct {
	Code        *string   `json:"code,omitempty" db:"code"`
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

type StatusSearchParams struct {
	Search    string `query:"search"`
	Page      uint64 `query:"page"`
	PageSize  uint64 `query:"pageSize"`
	q         squirrel.SelectBuilder
	OrderBy   string `query:"orderBy"`
	OrderDesc bool   `query:"orderDesc"`
	StatusID  config.ObfuscatedInt
}

func (s *StatusSearchParams) Apply() {
	s.q = pgdb.Qb.Select().From("statuses s")

	if len(s.Search) > 0 {
		search := "%" + s.Search + "%"
		s.q = s.q.Where(squirrel.Or{
			squirrel.Expr("s.description ilike ?", search),
		})
	}
}

func (s *StatusSearchParams) scanFullColumns(r pgx.Rows, c *Status) error {
	return r.Scan(
		&c.Code,
		&c.Description,
		&c.CreateTs,
	)
}

func (s *StatusSearchParams) GetData(ctx context.Context) ([]*Status, error) {
	if s.PageSize == 0 {
		s.PageSize = 20
	}

	if s.Page == 0 {
		s.Page = 1
	}

	offset := s.PageSize * (s.Page - 1)

	var orderBy string
	switch s.OrderBy {
	case "createTs":
		orderBy = "s.create_ts"
	case "description":
		orderBy = "s.description"
	default:
		orderBy = "s.code"
	}

	if s.OrderDesc {
		orderBy += " desc"
	}

	q := s.q.Columns(
		"s.code",
		"s.description",
		"s.create_ts",
	).OrderBy(orderBy).Offset(offset).Limit(s.PageSize)

	r, err := pgdb.QbQuery(ctx, q)
	if err != nil {
		return nil, err
	}
	opportunities, err := pgx.CollectRows(r, func(row pgx.CollectableRow) (*Status, error) {
		var o Status
		err := s.scanFullColumns(r, &o)
		if err != nil {
			return nil, err
		}

		return &o, nil
	})

	if err != nil {
		return nil, err
	}

	return opportunities, nil
}

func (s *StatusSearchParams) GetCount(ctx context.Context) (cnt int, err error) {
	q := s.q.Column("count(1) as count")
	r, err := pgdb.QbQueryRow(ctx, q)
	if err != nil {
		return
	}

	err = r.Scan(&cnt)
	return
}

func ListStatusHandler(c *fiber.Ctx) (err error) {
	ctx := c.Context()
	var s StatusSearchParams
	err = c.QueryParser(&s)
	if err != nil {
		return errs.ErrBadParameter().WithDetail(err)
	}

	s.Apply()
	data, err := s.GetData(ctx)
	if err != nil {
		return errs.ErrServerError().WithDetail(err)
	}

	count, err := s.GetCount(ctx)
	if err != nil {
		return errs.ErrServerError().WithDetail(err)
	}

	return utils.FiberJSONWrap(c, data, count)
}
