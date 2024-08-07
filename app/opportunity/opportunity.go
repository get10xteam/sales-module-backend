package opportunity

import (
	"context"
	"errors"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/georgysavva/scany/v2/pgxscan"
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

type Opportunity struct {
	Id                              config.ObfuscatedInt   `json:"id" db:"id"`
	OwnerId                         config.ObfuscatedInt   `json:"ownerId" db:"owner_id"`
	AssigneeId                      config.ObfuscatedInt   `json:"assigneeId" db:"assignee_id"`
	ClientId                        *config.ObfuscatedInt  `json:"clientId" db:"client_id"`
	StatusCode                      string                 `json:"statusCode" db:"status_code"`
	Name                            string                 `json:"name" db:"name"`
	Description                     *string                `json:"description,omitempty" db:"description"`
	TalentBudget                    *float64               `json:"talentBudget" db:"talent_budget"`
	NonTalentBudget                 *float64               `json:"nonTalentBudget" db:"non_talent_budget"`
	Revenue                         *float64               `json:"revenue" db:"revenue"`
	ExpectedProfitabilityPercentage *float64               `json:"expectedProfitabilityPercentage" db:"expected_profitability_percentage"`
	ExpectedProfitabilityAmount     *float64               `json:"expectedProfitabilityAmount" db:"expected_profitability_amount"`
	CreateTs                        time.Time              `json:"createTs" db:"create_ts"`
	OwnerName                       string                 `json:"ownerName,omitempty" db:"owner_name"`
	AssigneeName                    string                 `json:"assigneeName,omitempty" db:"assignee_name"`
	ClientName                      *string                `json:"clientName" db:"client_name"`
	Categories                      []*OpportunityCategory `json:"categories,omitempty"`
	OwnerProfileImgUrl              *string                `json:"ownerProfileImgUrl" db:"owner_profile_img_url"`
	AssigneeProfileImgUrl           *string                `json:"assigneeProfileImgUrl" db:"assignee_profile_img_url"`
	ClientLogoUrl                   *string                `json:"clientLogoUrl" db:"client_logo_url"`
}

func OpportunityById(ctx context.Context, id config.ObfuscatedInt, cols ...string) (*Opportunity, error) {
	if len(cols) == 0 {
		cols = []string{"id"}
	}

	r, err := pgdb.QbQuery(ctx, pgdb.Qb.Select(cols...).From("opportunities").Where("id = ?", id))
	if err != nil {
		return nil, err
	}

	o := &Opportunity{}
	err = pgxscan.ScanOne(o, r)
	if err != nil {
		return nil, err
	}
	return o, err
}

func (o *Opportunity) CreateToDB(ctx context.Context) error {

	// var talentBudget *float64
	if o.TalentBudget != nil && *o.TalentBudget == 0 {
		o.TalentBudget = nil
	}
	// // var nonTalentBudget *float64
	if o.NonTalentBudget != nil && *o.NonTalentBudget == 0 {
		o.NonTalentBudget = nil
	}
	// // var revenue *float64
	if o.Revenue != nil && *o.Revenue == 0 {
		o.Revenue = nil
	}

	insertMap := map[string]any{
		"owner_id":          o.OwnerId,
		"assignee_id":       o.AssigneeId,
		"client_id":         o.ClientId,
		"status_code":       o.StatusCode,
		"name":              o.Name,
		"description":       o.Description,
		"talent_budget":     o.TalentBudget,
		"non_talent_budget": o.NonTalentBudget,
		"revenue":           o.Revenue,
	}

	r, err := pgdb.QbQueryRow(ctx, pgdb.Qb.Insert("opportunities").SetMap(insertMap).Suffix("returning id"))
	if err != nil {
		return err
	}

	if err = r.Scan(&o.Id); err != nil {
		return err
	}

	if len(o.Categories) > 0 {
		qb := pgdb.Qb.Insert("opportunity_categories").Suffix("returning id, create_ts")
		qb = qb.Columns("name", "opportunity_id")
		for _, category := range o.Categories {
			qb = qb.Values(category.Name, o.Id)
		}
		var r pgx.Rows
		r, err = pgdb.QbQuery(ctx, qb)
		if err != nil {
			return err
		}
		defer r.Close()
		for _, category := range o.Categories {
			if r.Next() {
				err = r.Scan(&category.Id, &category.CreateTs)
				if err != nil {
					return err
				}
			}
		}

		for _, category := range o.Categories {
			if len(category.Files) > 0 {
				qb = pgdb.Qb.Insert("opportunity_category_files").Suffix("returning id, create_ts")
				qb = qb.Columns("opportunity_category_id", "url", "name", "version", "creator_id")
				for index, file := range category.Files {
					file.Version = index + 1
					qb = qb.Values(category.Id, file.URL, file.Name, file.Version, o.AssigneeId)
				}

				r, err = pgdb.QbQuery(ctx, qb)
				if err != nil {
					return err
				}

				defer r.Close()
				for _, files := range category.Files {
					if r.Next() {
						err = r.Scan(&files.Id, &files.CreateTs)
						if err != nil {
							return err
						}
					}
				}
			}
		}
	}

	return nil
}

func (o *Opportunity) Validate(ctx context.Context) (err error) {

	if o.Name == "" {
		return errs.ErrBadParameter().WithMessage("name cannot be empty")
	}

	if o.ClientId != nil && !o.ClientId.IsEmpty() {
		_, err = client.ClientById(ctx, *o.ClientId)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return errs.ErrNotExist().WithMessage("cannot find client_id")
			}

			return errs.ErrServerError().WithDetail(err)
		}
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

func (o *Opportunity) LoadFromDB(ctx context.Context) (err error) {
	if o.Id.IsEmpty() {
		return errs.ErrBadParameter()
	}
	const sql = `SELECT 
		owner_id,
		assignee_id,
		client_id,
		status_code,
		name,
		description,
		talent_budget,
		non_talent_budget,
		revenue,
		expected_profitability_percentage,
		expected_profitability_amount,
		create_ts
	FROM 
		opportunities
	WHERE 
		id=$1;`
	err = pgdb.QueryRow(ctx, sql, o.Id).Scan(
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
	)

	return
}

// UpdateToDB can only edit name, description, non_talent_budget, talent_budget, revenue
func (o *Opportunity) UpdateToDB(ctx context.Context, updateMap map[string]any) (err error) {
	if o.Id.IsEmpty() {
		return errs.ErrBadParameter()
	}

	_, err = pgdb.QbExec(ctx, pgdb.Qb.Update("opportunities").SetMap(updateMap).Where("id = ?", o.Id))
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
	Search             string `query:"search"`
	Page               uint64 `query:"page"`
	PageSize           uint64 `query:"pageSize"`
	q                  squirrel.SelectBuilder
	OrderBy            string `query:"orderBy"`
	OrderDesc          bool   `query:"orderDesc"`
	OpportunityID      config.ObfuscatedInt
	IncludeDescription bool                   `query:"includeDescription"`
	AssigneeIds        []config.ObfuscatedInt `query:"assigneeIds"`
	AssigneeId         config.ObfuscatedInt   // TODO add logic with AssigneeId tidak boleh refer selain bawahan dan diri sendiri
	AssigneeIdRecurse  bool                   // TODO add logic with AssigneeIdRecurse
	OwnerIds           []config.ObfuscatedInt `query:"ownerIds"`
	OwnerId            config.ObfuscatedInt   // TODO add logic with OwnerId tidak boleh refer selain bawahan dan diri sendiri
	OwnerIdRecurse     bool                   // TODO add logic with OwnerIdRecurse
	StatusCodes        []string               `query:"statusCodes"`
	ClientIds          []config.ObfuscatedInt `query:"clientIds"`
}

func (s *opportunitiesSearchParams) Apply() {
	s.q = pgdb.Qb.Select().From("opportunities o").
		LeftJoin("users uo on uo.id = o.owner_id").
		LeftJoin("users ua on ua.id = o.assignee_id").
		LeftJoin("clients c on c.id = o.client_id").
		LeftJoin("statuses s on s.code = o.status_code")

	if len(s.Search) > 0 {
		search := "%" + s.Search + "%"
		orCondition := squirrel.Or{
			squirrel.Expr("o.name ilike ?", search),
		}

		if s.IncludeDescription {
			orCondition = append(
				orCondition,
				squirrel.Expr("o.description ilike ?", search),
			)
		}

		s.q = s.q.Where(orCondition)
	}

	if len(s.AssigneeIds) > 0 {
		s.q = s.q.Where(squirrel.Eq{"o.assignee_id": s.AssigneeIds})
	}

	if len(s.OwnerIds) > 0 {
		s.q = s.q.Where(squirrel.Eq{"o.owner_id": s.OwnerIds})
	}

	if len(s.StatusCodes) > 0 {
		s.q = s.q.Where(squirrel.Eq{"s.code": s.StatusCodes})
	}

	if len(s.ClientIds) > 0 {
		s.q = s.q.Where(squirrel.Eq{"c.id": s.ClientIds})
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
		&o.OwnerProfileImgUrl,
		&o.AssigneeProfileImgUrl,
		&o.ClientLogoUrl,
	)
}

func (s *opportunitiesSearchParams) columns() []string {
	return []string{
		"o.id",
		"o.owner_id",
		"o.assignee_id",
		"o.client_id",
		"o.status_code",
		"o.name",
		"o.description",
		"COALESCE(o.talent_budget, 0)",
		"COALESCE(o.non_talent_budget, 0)",
		"COALESCE(o.revenue, 0)",
		"COALESCE(o.expected_profitability_percentage, 0)",
		"COALESCE(o.expected_profitability_amount, 0)",
		"o.create_ts",
		"uo.name as owner_name",
		"ua.name as assignee_name",
		"c.name as client_name",
		"uo.profile_img_url as owner_profile_img_url",
		"ua.profile_img_url as assignee_profile_img_url",
		"c.logo_url as client_logo_url",
	}
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
	case "statusCode":
		orderBy = "s.code"
	default:
		orderBy = "o.id"
	}

	if s.OrderDesc {
		orderBy += " desc"
	}

	q := s.q.Columns(s.columns()...).OrderBy(orderBy).Offset(offset).Limit(s.PageSize)

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
	s.q = s.q.Columns(s.columns()...).Where("o.id = ?", s.OpportunityID).Limit(1)
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

	opportunityID, ok := c.Locals(opportunityLocalsKey).(config.ObfuscatedInt)
	if !ok {
		return errs.ErrBadParameter().WithMessage("invalid path :opportunityID parameter")
	}

	var s opportunitiesSearchParams
	s.OpportunityID = opportunityID
	s.Apply()

	o, err := s.GetSingle(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.ErrNotExist().WithDetail(err)
		}
		return errs.ErrServerError().WithDetail(err)
	}

	err = o.LoadOpportunityCategories(ctx)
	if err != nil {
		return errs.ErrServerError().WithDetail(err)
	}

	return utils.FiberJSONWrap(c, o)
}

func OpportunityEditHandlerHandler(c *fiber.Ctx) (err error) {
	ctx := c.Context()

	opportunityID, ok := c.Locals(opportunityLocalsKey).(config.ObfuscatedInt)
	if !ok {
		return errs.ErrBadParameter().WithMessage("invalid path :opportunityID parameter")
	}

	o := Opportunity{}
	err = c.BodyParser(&o)
	if err != nil {
		return errs.ErrBadParameter().WithMessage("Body not valid")
	}

	u := user.UserFromHttp(c)

	var s opportunitiesSearchParams
	s.OpportunityID = opportunityID
	s.Apply()

	data, err := s.GetSingle(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.ErrNotExist().WithDetail(err)
		}
		return errs.ErrServerError().WithDetail(err)
	}

	if u.Id != data.AssigneeId {
		return errs.ErrUnauthorized().WithMessage("cannot authorize assignee_id")
	}

	o.Id = opportunityID

	updateMap := map[string]any{}
	if len(o.Name) > 0 {
		updateMap["name"] = o.Name
	}
	if o.Description != nil && len(*o.Description) > 0 {
		updateMap["description"] = o.Description
	}
	if o.NonTalentBudget != nil {
		if *o.NonTalentBudget == 0 {
			updateMap["non_talent_budget"] = nil
		} else {
			updateMap["non_talent_budget"] = o.NonTalentBudget
		}
	}
	if o.TalentBudget != nil {
		if *o.TalentBudget == 0 {
			updateMap["talent_budget"] = nil
		} else {
			updateMap["talent_budget"] = o.TalentBudget
		}
	}
	if o.Revenue != nil {
		if *o.Revenue == 0 {
			updateMap["revenue"] = nil
		} else {
			updateMap["revenue"] = o.Revenue
		}
	}

	err = o.UpdateToDB(ctx, updateMap)
	if err != nil {
		return
	}

	return utils.FiberJSONWrap(c, o)
}
