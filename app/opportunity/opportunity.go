package opportunity

import (
	"context"
	"errors"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/get10xteam/sales-module-backend/app/client"
	"github.com/get10xteam/sales-module-backend/app/status"
	"github.com/get10xteam/sales-module-backend/app/user"
	"github.com/get10xteam/sales-module-backend/errs"
	"github.com/get10xteam/sales-module-backend/plumbings/config"
	"github.com/get10xteam/sales-module-backend/plumbings/utils"
	"github.com/jackc/pgx/v5"
	"gitlab.com/intalko/gosuite/pgdb"

	"github.com/gofiber/fiber/v2"
)

// id
// owner_id
// assignee_id
// client_id
// status_code
// name
// description
// talent_budget
// non_talent_budget
// revenue
// expected_profitability_percentage
// expected_profitability_amount
// create_ts

type Opportunity struct {
	Id                              config.ObfuscatedInt `json:"id" db:"id"`
	OwnerId                         config.ObfuscatedInt `json:"ownerId" db:"owner_id"`
	AssigneeId                      config.ObfuscatedInt `json:"assigneeId" db:"assignee_id"`
	ClientId                        config.ObfuscatedInt `json:"clientId" db:"client_id"`
	StatusCode                      string               `json:"statusCode" db:"status_code"`
	Name                            string               `json:"name" db:"name"`
	Description                     *string              `json:"description,omitempty" db:"description"`
	TalentBudget                    float64              `json:"talentBudget" db:"talent_budget"`
	NonTalentBudget                 float64              `json:"nonTalentBudget" db:"non_talent_budget"`
	Revenue                         float64              `json:"revenue" db:"revenue"`
	ExpectedProfitabilityPercentage float64              `json:"expectedProfitabilityPercentage" db:"expected_profitability_percentage"`
	ExpectedProfitabilityAmount     float64              `json:"expectedProfitabilityAmount" db:"expected_profitability_amount"`
	CreateTs                        time.Time            `json:"createTs" db:"create_ts"`
	OwnerName                       string               `json:"ownerName,omitempty" db:"owner_name"`       // join table user
	AssigneeName                    string               `json:"assigneeName,omitempty" db:"assignee_name"` // join table user
	ClientName                      string               `json:"clientName,omitempty" db:"client_name"`     // join table client
	StatusName                      string               `json:"statusName,omitempty" db:"status_name"`     // join table status
}

func (u *Opportunity) CreateToDB(ctx context.Context) error {
	insertMap := map[string]any{
		"owner_id":          u.OwnerId,
		"assignee_id":       u.AssigneeId,
		"client_id":         u.ClientId,
		"status_code":       u.StatusCode,
		"name":              u.Name,
		"description":       u.Description,
		"talent_budget":     u.TalentBudget,
		"non_talent_budget": u.NonTalentBudget,
		"revenue":           u.Revenue,
	}

	r, err := pgdb.QbQueryRow(ctx, pgdb.Qb.Insert("opportunities").SetMap(insertMap).Suffix("returning id"))
	if err != nil {
		return err
	}

	return r.Scan(&u.Id)
}

func (o *Opportunity) Validate(ctx context.Context) (err error) {

	if o.Name == "" {
		return errs.ErrBadParameter().WithMessage("name cannot be empty")
	}

	if o.TalentBudget < 1 {
		return errs.ErrBadParameter().WithMessage("talent_budget must be larger than 0")
	}

	if o.NonTalentBudget < 1 {
		return errs.ErrBadParameter().WithMessage("non_talent_budget must be larger than 0")
	}

	if o.Revenue < 1 {
		return errs.ErrBadParameter().WithMessage("revenue must be larger than 0")
	}

	_, err = client.ClientById(ctx, o.ClientId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.ErrNotExist().WithMessage("cannot find client_id")
		}

		return errs.ErrServerError().WithDetail(err)
	}

	_, err = status.StatusByCode(ctx, o.StatusCode)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.ErrNotExist().WithMessage("cannot find status_code")
		}

		return errs.ErrServerError().WithDetail(err)
	}

	return
}

func CreateOpportunityHandler(c *fiber.Ctx) error {
	o := Opportunity{}
	err := c.BodyParser(&o)
	if err != nil {
		return errs.ErrBadParameter().WithMessage("Body not valid")
	}

	ctx := c.Context()
	if err := o.Validate(ctx); err != nil {
		return err
	}

	u := user.UserFromHttp(c)
	o.OwnerId = u.Id
	o.AssigneeId = u.Id

	err = o.CreateToDB(ctx)
	if err != nil {
		return errs.ErrServerError().WithDetail(err)
	}

	return utils.FiberJSONWrapWithStatusCreated(c, map[string]config.ObfuscatedInt{
		"id": o.Id,
	})
}

type opportunitiesSearchParams struct {
	Search        string `query:"search"`
	Page          uint64 `query:"page"`
	PageSize      uint64 `query:"pageSize"`
	q             squirrel.SelectBuilder
	OrderBy       string `query:"orderBy"`
	OrderDesc     bool   `query:"orderDesc"`
	OpportunityID config.ObfuscatedInt
}

func (s *opportunitiesSearchParams) Apply() {
	s.q = pgdb.Qb.Select().From("opportunities o").
		LeftJoin("users uo on uo.id = o.owner_id").
		LeftJoin("users ua on ua.id = o.assignee_id").
		LeftJoin("clients c on c.id = o.client_id").
		LeftJoin("statuses s on s.code = o.status_code")

	if len(s.Search) > 0 {
		search := "%" + s.Search + "%"
		s.q = s.q.Where(squirrel.Or{
			squirrel.Expr("o.name ilike ?", search),
			squirrel.Expr("o.description ilike ?", search),
			squirrel.Expr("uo.name ilike ?", search),
			squirrel.Expr("ua.name ilike ?", search),
			squirrel.Expr("c.name ilike ?", search),
			squirrel.Expr("s.name ilike ?", search),
		})
	}
}

func (s *opportunitiesSearchParams) scanFullColumns(r pgx.Rows, o *Opportunity) error {
	return r.Scan(
		&o.Id,
		&o.OwnerId,
		&o.AssigneeId,
		&o.ClientId,
		&o.StatusCode,
		&o.Name,
		&o.Description,
		&o.TalentBudget,
		&o.NonTalentBudget,
		&o.Revenue,
		&o.ExpectedProfitabilityPercentage,
		&o.ExpectedProfitabilityAmount,
		&o.CreateTs,
		&o.OwnerName,
		&o.AssigneeName,
		&o.ClientName,
		&o.StatusName,
	)
}

func (s *opportunitiesSearchParams) GetData(ctx context.Context) ([]*Opportunity, error) {
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
		orderBy = "o.create_ts"
	case "name":
		orderBy = "o.name"
	case "ownerName":
		orderBy = "uo.name"
	case "assigneeName":
		orderBy = "ua.name"
	case "clientName":
		orderBy = "c.name"
	case "statusName":
		orderBy = "s.name"
	default:
		orderBy = "o.id"
	}

	if s.OrderDesc {
		orderBy += " desc"
	}

	q := s.q.Columns(
		"o.id",
		"o.owner_id",
		"o.assignee_id",
		"o.client_id",
		"o.status_code",
		"o.name",
		"o.description",
		"o.talent_budget",
		"o.non_talent_budget",
		"o.revenue",
		"o.expected_profitability_percentage",
		"o.expected_profitability_amount",
		"o.create_ts",
		"uo.name as owner_name",
		"ua.name as assignee_name",
		"c.name as client_name",
		"s.name as status_name",
	).OrderBy(orderBy).Offset(offset).Limit(s.PageSize)

	r, err := pgdb.QbQuery(ctx, q)
	if err != nil {
		return nil, err
	}
	opportunities, err := pgx.CollectRows(r, func(row pgx.CollectableRow) (*Opportunity, error) {
		var o Opportunity
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

func (s *opportunitiesSearchParams) GetSingle(ctx context.Context) (*Opportunity, error) {
	s.q = s.q.Limit(1)

	r, err := pgdb.QbQuery(ctx, s.q)
	if err != nil {
		return nil, err
	}

	opportunity, err := pgx.CollectOneRow(r, func(row pgx.CollectableRow) (*Opportunity, error) {
		var o Opportunity
		err := s.scanFullColumns(r, &o)
		if err != nil {
			return nil, err
		}

		return &o, nil
	})

	if err != nil {
		return nil, err
	}

	return opportunity, nil
}

func (s *opportunitiesSearchParams) GetCount(ctx context.Context) (cnt int, err error) {
	q := s.q.Column("count(1) as count")
	r, err := pgdb.QbQueryRow(ctx, q)
	if err != nil {
		return
	}

	err = r.Scan(&cnt)
	return
}

func ListOpportunitiesHandler(c *fiber.Ctx) (err error) {
	ctx := c.Context()
	var s opportunitiesSearchParams
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

const opportunityLocalsKey = "localsOpportunity"

// Should only be called after MustAuthMiddleware.
func MustOpportunityIDMiddleware(c *fiber.Ctx) error {

	var id config.ObfuscatedInt

	opportunityID := c.Params("opportunityID")
	if opportunityID == "" {
		return errs.ErrBadParameter().WithMessage("invalid path :opportunityID parameter")
	}

	err := id.Parse(opportunityID)
	if err != nil {
		return errs.ErrBadParameter().WithMessage("invalid path :opportunityID parameter")
	}

	c.Locals(opportunityLocalsKey, id)

	return c.Next()
}

func OpportunityDetailHandler(c *fiber.Ctx) (err error) {
	ctx := c.Context()
	var s opportunitiesSearchParams
	err = c.QueryParser(&s)
	if err != nil {
		return errs.ErrBadParameter().WithDetail(err)
	}

	opportunityID, ok := c.Locals(opportunityLocalsKey).(config.ObfuscatedInt)
	if !ok {
		return errs.ErrBadParameter().WithMessage("invalid path :opportunityID parameter")
	}

	s.OpportunityID = opportunityID
	s.Search = ""
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
