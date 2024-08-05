package client

import (
	"context"
	"errors"
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
	Name     string               `json:"name,omitempty" db:"name"`
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

func (cl *Client) UpdateToDB(ctx context.Context, updateMap map[string]any) (err error) {
	if cl.Id.IsEmpty() {
		return errs.ErrBadParameter()
	}
	_, err = pgdb.QbExec(ctx, pgdb.Qb.Update("clients").
		SetMap(updateMap).
		Where("id = ?", cl.Id))
	return
}

func (cl *Client) CreateToDB(ctx context.Context) (err error) {
	if !cl.Id.IsEmpty() {
		return errs.ErrBadParameter()
	}
	insertMap := map[string]any{
		"name": cl.Name,
	}
	if cl.LogoUrl != nil {
		insertMap["logo_url"] = cl.LogoUrl
	}
	r, err := pgdb.QbQueryRow(ctx, pgdb.Qb.Insert("clients").SetMap(insertMap).Suffix("returning id, create_ts"))
	if err != nil {
		return
	}
	err = r.Scan(&cl.Id, &cl.CreateTs)
	return
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

	q := s.q.Columns(s.columns()...).OrderBy(orderBy).Offset(offset).Limit(s.PageSize)

	r, err := pgdb.QbQuery(ctx, q)
	if err != nil {
		return nil, err
	}
	clients, err := pgx.CollectRows(r, func(row pgx.CollectableRow) (*Client, error) {
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

	return clients, nil
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

func (s *ClientsSearchParams) columns() []string {
	return []string{
		"c.id",
		"c.name",
		"c.logo_url",
		"c.create_ts",
	}
}

func (s *ClientsSearchParams) GetSingle(ctx context.Context) (*Client, error) {
	s.q = s.q.Columns(s.columns()...).Where("c.id = ?", s.ClientID).Limit(1)
	r, err := pgdb.QbQuery(ctx, s.q)
	if err != nil {
		return nil, err
	}

	client, err := pgx.CollectOneRow(r, func(row pgx.CollectableRow) (*Client, error) {
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

	return client, nil
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

func (cl *Client) LoadIdentifiersFromHttp(c *fiber.Ctx) (err error) {
	if clientIdStr := c.Params("clientId"); len(clientIdStr) > 0 {
		err = cl.Id.Parse(clientIdStr)
		if err != nil {
			return errs.ErrBadParameter().WithMessage("invalid path :clientId parameter")
		}
	} else {
		return errs.ErrBadParameter().WithMessage("invalid path :clientId parameter")
	}
	return
}

func CreateClientHandler(c *fiber.Ctx) (err error) {
	ctx := c.Context()
	var cl Client
	err = c.BodyParser(&cl)
	if err != nil {
		return errs.ErrBadParameter().WithDetail(err)
	}
	if len(cl.Name) == 0 {
		return errs.ErrBadParameter().WithMessage("client name must be set")
	}

	err = cl.CreateToDB(ctx)
	if err != nil {
		return errs.ErrServerError().WithDetail(err)
	}
	return utils.FiberJSONWrap(c, cl)
}

const clientLocalsKey = "localsclient"

// Should only be called after MustAuthMiddleware.
func MustClientIDMiddleware(c *fiber.Ctx) error {

	var id config.ObfuscatedInt

	clientID := c.Params("clientID")
	if clientID == "" {
		return errs.ErrBadParameter().WithMessage("invalid path :clientID parameter")
	}

	err := id.Parse(clientID)
	if err != nil {
		return errs.ErrBadParameter().WithMessage("invalid path :clientID parameter")
	}

	c.Locals(clientLocalsKey, id)

	return c.Next()
}

func ChangeClientHandler(c *fiber.Ctx) (err error) {
	ctx := c.Context()
	var cl Client
	err = c.BodyParser(&cl)
	if err != nil {
		return errs.ErrBadParameter().WithDetail(err)
	}

	clientID, ok := c.Locals(clientLocalsKey).(config.ObfuscatedInt)
	if !ok {
		return errs.ErrBadParameter().WithMessage("invalid path :clientID parameter")
	}

	cl.Id = clientID

	update := map[string]any{}
	if len(cl.Name) > 0 {
		update["name"] = cl.Name
	}

	if cl.LogoUrl != nil {
		if len(*cl.LogoUrl) > 0 {
			update["logo_url"] = cl.LogoUrl
		} else {
			update["logo_url"] = nil
		}
	}

	if len(update) > 0 {
		err = cl.UpdateToDB(ctx, update)
		if err != nil {
			return errs.ErrServerError().WithDetail(err)
		}
	}
	return utils.FiberJSONWrap(c, cl)
}

func ClientDetailHandler(c *fiber.Ctx) (err error) {
	ctx := c.Context()

	clientID, ok := c.Locals(clientLocalsKey).(config.ObfuscatedInt)
	if !ok {
		return errs.ErrBadParameter().WithMessage("invalid path :clientID parameter")
	}

	var s ClientsSearchParams
	s.ClientID = clientID
	s.Apply()

	data, err := s.GetSingle(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.ErrNotExist().WithDetail(err)
		}
		return errs.ErrServerError().WithDetail(err)
	}

	return utils.FiberJSONWrap(c, data)
}
