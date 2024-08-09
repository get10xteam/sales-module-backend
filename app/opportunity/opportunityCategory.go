package opportunity

import (
	"context"
	"errors"
	"time"

	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/get10xteam/sales-module-backend/app/user"
	"github.com/get10xteam/sales-module-backend/errs"
	"github.com/get10xteam/sales-module-backend/plumbings/config"
	"github.com/get10xteam/sales-module-backend/plumbings/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"gitlab.com/intalko/gosuite/pgdb"
)

// remove FileType

type OpportunityCategory struct {
	Id            config.ObfuscatedInt       `json:"id" db:"id"`
	OpportunityId config.ObfuscatedInt       `json:"opportunityId" db:"opportunity_id"`
	Name          string                     `json:"name" db:"name"`
	Files         []*OpportunityCategoryFile `json:"files"`
	CreateTs      time.Time                  `json:"createTs" db:"create_ts"`
}

func OpportunityCategoryById(ctx context.Context, id config.ObfuscatedInt, cols ...string) (*OpportunityCategory, error) {
	if len(cols) == 0 {
		cols = []string{"id"}
	}

	r, err := pgdb.QbQuery(ctx, pgdb.Qb.Select(cols...).From("opportunity_categories").Where("id = ?", id))
	if err != nil {
		return nil, err
	}

	oc := &OpportunityCategory{}
	err = pgxscan.ScanOne(oc, r)
	if err != nil {
		return nil, err
	}
	return oc, err
}

func (oc *OpportunityCategory) CreateToDB(ctx context.Context) error {
	insertMap := map[string]any{
		"opportunity_id": oc.OpportunityId,
		"name":           oc.Name,
	}

	r, err := pgdb.QbQueryRow(ctx, pgdb.Qb.Insert("opportunity_categories").SetMap(insertMap).Suffix("returning id, create_ts"))
	if err != nil {
		return err
	}

	if err = r.Scan(&oc.Id, &oc.CreateTs); err != nil {
		return err
	}

	return nil
}

func (oc *OpportunityCategory) UpdateToDB(ctx context.Context, updateMap map[string]any) (err error) {
	if oc.Id.IsEmpty() {
		return errs.ErrBadParameter()
	}
	_, err = pgdb.QbExec(ctx, pgdb.Qb.Update("opportunity_categories").SetMap(updateMap).Where("id = ?", oc.Id))
	return
}

func (o *Opportunity) LoadOpportunityCategories(ctx context.Context) (err error) {
	const sql = "SELECT id, opportunity_id, name, create_ts FROM opportunity_categories WHERE opportunity_id=$1 order by id"
	rows, err := pgdb.Query(ctx, sql, o.Id)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var oc OpportunityCategory
		err = rows.Scan(&oc.Id, &oc.OpportunityId, &oc.Name, &oc.CreateTs)
		if err != nil {
			return
		}
		o.Categories = append(o.Categories, &oc)
	}

	for _, category := range o.Categories {
		err = category.LoadOpportunityCategoriesFiles(ctx)
		if err != nil {
			return
		}
	}

	return nil
}

func CreateOpportunityCategoryHandler(c *fiber.Ctx) error {
	oc := OpportunityCategory{}
	err := c.BodyParser(&oc)
	if err != nil {
		return errs.ErrBadParameter().WithMessage("Body not valid")
	}

	ctx := c.Context()

	opportunityID, ok := c.Locals(opportunityLocalsKey).(config.ObfuscatedInt)
	if !ok {
		return errs.ErrBadParameter().WithMessage("invalid path :opportunityID parameter")
	}

	o, err := OpportunityById(ctx, opportunityID, "assignee_id")
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.ErrNotExist().WithDetail("cannot find opportunity_id")
		}
		return errs.ErrServerError().WithDetail(err)
	}

	oc.OpportunityId = opportunityID

	u := user.UserFromHttp(c)
	if u.Id != o.AssigneeId {
		return errs.ErrUnauthorized().WithMessage("cannot authorize assignee_id")
	}

	err = oc.CreateToDB(ctx)
	if err != nil {
		return errs.ErrServerError().WithDetail(err)
	}

	return utils.FiberJSONWrapWithStatusCreated(c, map[string]config.ObfuscatedInt{
		"id": oc.Id,
	})
}

const opportunityCategoryLocalsKey = "localsOpportunityCategory"

// Should only be called after MustAuthMiddleware.
func MustOpportunityCategoryIDMiddleware(c *fiber.Ctx) error {

	var id config.ObfuscatedInt

	opportunityCategoryID := c.Params("opportunityCategoryID")
	if opportunityCategoryID == "" {
		return errs.ErrBadParameter().WithMessage("invalid path :opportunityCategoryID parameter")
	}

	err := id.Parse(opportunityCategoryID)
	if err != nil {
		return errs.ErrBadParameter().WithMessage("invalid path :opportunityCategoryID parameter")
	}

	c.Locals(opportunityCategoryLocalsKey, id)

	return c.Next()
}

func ChangeOpportunityCategoryHandler(c *fiber.Ctx) (err error) {
	ctx := c.Context()
	var oc OpportunityCategory
	err = c.BodyParser(&oc)
	if err != nil {
		return errs.ErrBadParameter().WithDetail(err)
	}

	opportunityID, ok := c.Locals(opportunityLocalsKey).(config.ObfuscatedInt)
	if !ok {
		return errs.ErrBadParameter().WithMessage("invalid path :opportunityID parameter")
	}

	o, err := OpportunityById(ctx, opportunityID, "assignee_id")
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.ErrNotExist().WithDetail("cannot find opportunity_id")
		}
		return errs.ErrServerError().WithDetail(err)
	}

	u := user.UserFromHttp(c)
	if u.Id != o.AssigneeId {
		return errs.ErrUnauthorized().WithMessage("cannot authorize assignee_id")
	}

	oc.OpportunityId = opportunityID

	opportunityCategoryID, ok := c.Locals(opportunityCategoryLocalsKey).(config.ObfuscatedInt)
	if !ok {
		return errs.ErrBadParameter().WithMessage("invalid path :opportunityCategoryID parameter")
	}

	_, err = OpportunityCategoryById(ctx, opportunityCategoryID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.ErrNotExist().WithDetail("cannot find opportunity_category_id")
		}
		return errs.ErrServerError().WithDetail(err)
	}

	oc.Id = opportunityCategoryID

	update := map[string]any{}
	if len(oc.Name) > 0 {
		update["name"] = oc.Name
	}

	if len(update) > 0 {
		err = oc.UpdateToDB(ctx, update)
		if err != nil {
			return errs.ErrServerError().WithDetail(err)
		}
	}
	return utils.FiberJSONWrap(c, oc)
}
