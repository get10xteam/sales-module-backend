package client

import (
	"context"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/get10xteam/sales-module-backend/errs"
	"github.com/get10xteam/sales-module-backend/plumbings/config"
	"github.com/get10xteam/sales-module-backend/plumbings/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"

	"gitlab.com/intalko/gosuite/pgdb"

	"github.com/georgysavva/scany/v2/pgxscan"
)

type Client struct {
	Id       config.ObfuscatedInt `json:"id" db:"id"`
	Name     *string              `json:"name,omitempty" db:"name"`
	LogoUrl  *string              `json:"logoUrl,omitempty" db:"logo_url"`
	CreateTs time.Time            `json:"createTs,omitempty" db:"create_ts"`
}

func ClientById(ctx context.Context, id config.ObfuscatedInt, cols ...string) (*Client, error) {
	if len(cols) == 0 {
		cols = []string{"id"}
	}

	r, err := pgdb.QbQuery(ctx, pgdb.Qb.Select(cols...).From("clients").Where("id = ?", id))
	if err != nil {
		return nil, err
	}

	u := &Client{}
	err = pgxscan.ScanOne(u, r)
	if err != nil {
		return nil, err
	}
	return u, err
}

type ClientsSearchParams struct {
	Search    string `query:"search"`
	Page      uint64 `query:"page"`
	PageSize  uint64 `query:"pageSize"`
	q         squirrel.SelectBuilder
	OrderBy   string `query:"orderBy"`
	OrderDesc bool   `query:"orderDesc"`
	ClientID  config.ObfuscatedInt
}

func (s *ClientsSearchParams) Apply() {
	s.q = pgdb.Qb.Select().From("clients c")

	if len(s.Search) > 0 {
		search := "%" + s.Search + "%"
		s.q = s.q.Where("c.name ilike ?", search)
	}
}

func (s *ClientsSearchParams) scanFullColumns(r pgx.Rows, o *Client) error {
	return r.Scan(
		&o.Id,
		&o.Name,
		&o.LogoUrl,
		&o.CreateTs,
	)
}

func (s *ClientsSearchParams) GetData(ctx context.Context) ([]*Client, error) {
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
		orderBy = "c.create_ts"
	case "name":
		orderBy = "c.name"
	default:
		orderBy = "c.id"
	}

	if s.OrderDesc {
		orderBy += " desc"
	}

	q := s.q.Columns(
		"c.id",
		"c.name",
		"c.logo_url",
		"c.create_ts",
	).OrderBy(orderBy).Offset(offset).Limit(s.PageSize)

	r, err := pgdb.QbQuery(ctx, q)
	if err != nil {
		return nil, err
	}
	opportunities, err := pgx.CollectRows(r, func(row pgx.CollectableRow) (*Client, error) {
		var o Client
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

func (s *ClientsSearchParams) GetCount(ctx context.Context) (cnt int, err error) {
	q := s.q.Column("count(1) as count")
	r, err := pgdb.QbQueryRow(ctx, q)
	if err != nil {
		return
	}

	err = r.Scan(&cnt)
	return
}

func ListClientsHandler(c *fiber.Ctx) (err error) {
	ctx := c.Context()
	var s ClientsSearchParams
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
