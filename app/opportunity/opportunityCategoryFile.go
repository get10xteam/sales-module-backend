package opportunity

import (
	"context"
	"time"

	"github.com/get10xteam/sales-module-backend/plumbings/config"
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
