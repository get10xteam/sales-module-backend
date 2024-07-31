package level

import (
	"context"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/get10xteam/sales-module-backend/errs"
	"github.com/get10xteam/sales-module-backend/plumbings/config"
	"github.com/get10xteam/sales-module-backend/plumbings/utils"
	"github.com/jackc/pgx/v5"
	"gitlab.com/intalko/gosuite/pgdb"

	"github.com/gofiber/fiber/v2"
)

type Level struct {
	Id       config.ObfuscatedInt `json:"id" db:"id"`
	Name     string               `json:"name" db:"name"`
	CreateTs time.Time            `json:"createTs" db:"create_ts"`
}

type levelsSearchParams struct {
	Search    string `query:"search"`
	Page      uint64 `query:"page"`
	PageSize  uint64 `query:"pageSize"`
	q         squirrel.SelectBuilder
	OrderBy   string `query:"orderBy"`
	OrderDesc bool   `query:"orderDesc"`
	LevelID   config.ObfuscatedInt
}

func (s *levelsSearchParams) Apply() {
	s.q = pgdb.Qb.Select().From("levels l")

	if len(s.Search) > 0 {
		search := "%" + s.Search + "%"
		s.q = s.q.Where("l.name ilike ?", search)
	}
}

func (s *levelsSearchParams) scanFullColumns(r pgx.Rows, o *Level) error {
	return r.Scan(
		&o.Id,
		&o.Name,
		&o.CreateTs,
	)
}

func (s *levelsSearchParams) GetData(ctx context.Context) ([]*Level, error) {
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
		orderBy = "l.create_ts"
	case "name":
		orderBy = "l.name"
	default:
		orderBy = "l.id"
	}

	if s.OrderDesc {
		orderBy += " desc"
	}

	q := s.q.Columns(
		"l.id",
		"l.name",
		"l.create_ts",
	).OrderBy(orderBy).Offset(offset).Limit(s.PageSize)

	r, err := pgdb.QbQuery(ctx, q)
	if err != nil {
		return nil, err
	}
	levels, err := pgx.CollectRows(r, func(row pgx.CollectableRow) (*Level, error) {
		var o Level
		err := s.scanFullColumns(r, &o)
		if err != nil {
			return nil, err
		}

		return &o, nil
	})

	if err != nil {
		return nil, err
	}

	return levels, nil
}

func (s *levelsSearchParams) GetCount(ctx context.Context) (cnt int, err error) {
	q := s.q.Column("count(1) as count")
	r, err := pgdb.QbQueryRow(ctx, q)
	if err != nil {
		return
	}

	err = r.Scan(&cnt)
	return
}

func ListLevelsHandler(c *fiber.Ctx) (err error) {
	ctx := c.Context()
	var s levelsSearchParams
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
