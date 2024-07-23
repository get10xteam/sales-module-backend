package opportunity

import (
	"context"
	"errors"
	"fmt"
	"sync"
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

type CreateOpportunityPayload struct {
	OwnerId         config.ObfuscatedInt `json:"ownerId,omitempty"`    // check users
	AssigneeId      config.ObfuscatedInt `json:"assigneeId,omitempty"` // check users
	ClientId        config.ObfuscatedInt `json:"clientId,omitempty"`   // check clients
	StatusCode      string               `json:"statusCode,omitempty"` // check statuses
	Name            string               `json:"name"`
	Description     *string              `json:"description,omitempty"`
	TalentBudget    float64              `json:"talentBudget,omitempty"`
	NonTalentBudget float64              `json:"nonTalentBudget,omitempty"`
	Revenue         float64              `json:"revenue,omitempty"`
}

func (cop *CreateOpportunityPayload) Validate(ctx context.Context) *errs.Error {

	if cop.Name == "" {
		return errs.ErrBadParameter().WithMessage("name cannot be empty")
	}

	if cop.TalentBudget < 1 {
		return errs.ErrBadParameter().WithMessage("talent_budget must be larger than 0")
	}

	if cop.NonTalentBudget < 1 {
		return errs.ErrBadParameter().WithMessage("non_talent_budget must be larger than 0")
	}

	if cop.Revenue < 1 {
		return errs.ErrBadParameter().WithMessage("revenue must be larger than 0")
	}

	type fieldUsers struct {
		id   config.ObfuscatedInt
		name string
		err  error
	}

	// check if cop.OwnerId and cop.AssigneeId is valid or not
	var err error
	var wg sync.WaitGroup
	errsUserID := make(chan fieldUsers, 2)

	for _, field := range []fieldUsers{{id: cop.OwnerId, name: "owner_id"}, {id: cop.AssigneeId, name: "assignee_id"}} {
		wg.Add(1)
		go func(field fieldUsers) {
			defer wg.Done()
			_, err = user.UserById(ctx, field.id)
			if err != nil {
				field.err = err
				errsUserID <- field
			}
		}(field)
	}

	wg.Wait()
	close(errsUserID)

	errUserDetail, errWhenGetUser := <-errsUserID
	if errWhenGetUser {
		if errors.Is(errUserDetail.err, pgx.ErrNoRows) {
			return errs.ErrNotExist().WithMessage(fmt.Sprintf("cannot find %s", errUserDetail.name))
		}

		return errs.ErrServerError().WithDetail(err)
	}

	_, err = client.ClientById(ctx, cop.ClientId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.ErrNotExist().WithMessage("cannot find client_id")
		}

		return errs.ErrServerError().WithDetail(err)
	}

	_, err = status.StatusByCode(ctx, cop.StatusCode)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.ErrNotExist().WithMessage("cannot find status_code")
		}

		return errs.ErrServerError().WithDetail(err)
	}

	return nil
}

// for testing purposes
// INSERT INTO clients (id, "name", logo_url) VALUES(1, 'client test', NULL);
// INSERT INTO statuses (code, "name", description) VALUES('DRAFT', 'draft', 'drafting mode', now());

func CreateOpportunityHandler(c *fiber.Ctx) error {
	opportunityReq := CreateOpportunityPayload{}
	err := c.BodyParser(&opportunityReq)
	if err != nil {
		return errs.ErrBadParameter().WithMessage("Body not valid")
	}

	ctx := c.Context()
	if err := opportunityReq.Validate(ctx); err != nil {
		return err
	}

	opportunity := Opportunity{
		OwnerId:         opportunityReq.OwnerId,
		AssigneeId:      opportunityReq.AssigneeId,
		ClientId:        opportunityReq.ClientId,
		StatusCode:      opportunityReq.StatusCode,
		Name:            opportunityReq.Name,
		Description:     opportunityReq.Description,
		TalentBudget:    opportunityReq.TalentBudget,
		NonTalentBudget: opportunityReq.NonTalentBudget,
		Revenue:         opportunityReq.Revenue,
	}

	err = opportunity.CreateToDB(ctx)
	if err != nil {
		return errs.ErrServerError().WithDetail(err)
	}

	return utils.FiberJSONWrapWithStatusCreated(c, map[string]config.ObfuscatedInt{
		"id": opportunity.Id,
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
	s.q = pgdb.Qb.Select().From("opportunities o").Columns(
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
	).LeftJoin("users uo on uo.id = o.owner_id").Columns(
		"uo.name as owner_name",
	).LeftJoin("users ua on ua.id = o.assignee_id").Columns(
		"ua.name as assignee_name",
	).LeftJoin("clients c on c.id = o.client_id").Columns(
		"c.name as client_name",
	).LeftJoin("statuses s on s.code = o.status_code").Columns(
		"s.name as status_name",
	)
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

	switch s.OrderBy {
	case "createTs":
		s.OrderBy = "o.create_ts"
	case "name":
		s.OrderBy = "o.name"
	case "ownerName":
		s.OrderBy = "uo.name"
	case "assigneeName":
		s.OrderBy = "ua.name"
	case "clientName":
		s.OrderBy = "c.name"
	case "statusName":
		s.OrderBy = "s.name"
	default:
		s.OrderBy = "o.id"
	}

	orderClause := s.OrderBy
	if s.OrderDesc {
		orderClause = orderClause + " desc"
	} else {
		orderClause = orderClause + " asc"
	}

	s.q = s.q.OrderBy(orderClause).GroupBy(
		"o.id",
		"o.name",
		"uo.name",
		"ua.name",
		"c.name",
		"s.name",
	)
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
	s.q = s.q.Offset(offset)
	s.q = s.q.Limit(s.PageSize)

	r, err := pgdb.QbQuery(ctx, s.q)
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
	s.q = s.q.
		RemoveColumns().
		RemoveLimit().
		RemoveOffset().
		Column("count(1) as count")
	r, err := pgdb.QbQueryRow(ctx, s.q)
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
