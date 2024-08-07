package opportunity

import (
	"context"
	"errors"
	"time"

	"github.com/get10xteam/sales-module-backend/app/user"
	"github.com/get10xteam/sales-module-backend/errs"
	"github.com/get10xteam/sales-module-backend/plumbings/config"
	"github.com/get10xteam/sales-module-backend/plumbings/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"gitlab.com/intalko/gosuite/pgdb"
)

type OpportunityCategoryFile struct {
	Id                    config.ObfuscatedInt `json:"id" db:"id"`
	OpportunityCategoryId config.ObfuscatedInt `json:"opportunityCategoryId" db:"opportunity_category_id"`
	URL                   string               `json:"url" db:"url"`
	Name                  *string              `json:"name,omitempty" db:"name"`
	Version               int                  `json:"version" db:"version"`
	CreatorId             config.ObfuscatedInt `json:"creatorId" db:"creator_id"`
	CreateTs              time.Time            `json:"createTs" db:"create_ts"`
	CreatorName           string               `json:"creatorName" db:"creator_name"`
}

func (ocf *OpportunityCategoryFile) CreateToDB(ctx context.Context) error {

	var currentMaxVersion int
	const getVersionSql = "select coalesce(max(version),0) from opportunity_category_files " +
		"where opportunity_category_id = $1"
	err := pgdb.QueryRow(ctx, getVersionSql, ocf.OpportunityCategoryId).Scan(&currentMaxVersion)
	if err != nil {
		return err
	}
	if ocf.Version == 0 {
		ocf.Version = currentMaxVersion + 1
	}

	insertMap := map[string]any{
		"opportunity_category_id": ocf.OpportunityCategoryId,
		"url":                     ocf.URL,
		"name":                    ocf.Name,
		"version":                 ocf.Version,
		"creator_id":              ocf.CreatorId,
	}

	r, err := pgdb.QbQueryRow(ctx, pgdb.Qb.Insert("opportunity_category_files").SetMap(insertMap).Suffix("returning id, create_ts"))
	if err != nil {
		return err
	}

	if err = r.Scan(&ocf.Id, &ocf.CreateTs); err != nil {
		return err
	}

	return nil
}

func (oc *OpportunityCategory) LoadOpportunityCategoriesFiles(ctx context.Context) (err error) {
	const sql = `select 
			ocl.id, ocl.opportunity_category_id, ocl.url, ocl.name, ocl.version, ocl.creator_id, ocl.create_ts, u.name as creator_name
		from 
			opportunity_category_files ocl left join users u on ocl.creator_id = u.id
	 	where 
			opportunity_category_id = $1 
		order by 
			ocl.version`
	rows, err := pgdb.Query(ctx, sql, oc.Id)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var ocl OpportunityCategoryFile
		err = rows.Scan(&ocl.Id, &ocl.OpportunityCategoryId, &ocl.URL, &ocl.Name, &ocl.Version, &ocl.CreatorId, &ocl.CreateTs, &ocl.CreatorName)
		if err != nil {
			return
		}
		oc.Files = append(oc.Files, &ocl)
	}

	return nil
}

func CreateOpportunityCategoryFileHandler(c *fiber.Ctx) error {
	ocl := OpportunityCategoryFile{}
	err := c.BodyParser(&ocl)
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

	u := user.UserFromHttp(c)
	if u.Id != o.AssigneeId {
		return errs.ErrUnauthorized().WithMessage("cannot authorize assignee_id")
	}

	ocl.CreatorId = o.AssigneeId

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

	ocl.OpportunityCategoryId = opportunityCategoryID

	err = ocl.CreateToDB(ctx)
	if err != nil {
		return errs.ErrServerError().WithDetail(err)
	}

	return utils.FiberJSONWrapWithStatusCreated(c, map[string]config.ObfuscatedInt{
		"id": ocl.Id,
	})
}
