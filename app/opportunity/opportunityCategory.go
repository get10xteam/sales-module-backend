package opportunity

import (
	"context"
	"fmt"
	"time"

	"github.com/get10xteam/sales-module-backend/plumbings/config"
	"gitlab.com/intalko/gosuite/pgdb"
)

type OpportunityCategory struct {
	Id            config.ObfuscatedInt       `json:"id" db:"id"`
	OpportunityId config.ObfuscatedInt       `json:"opportunityId" db:"opportunity_id"`
	Name          string                     `json:"name" db:"name"`
	Files         []*OpportunityCategoryFile `json:"files"`
	CreateTs      time.Time                  `json:"createTs" db:"create_ts"`
}

func (oc *OpportunityCategory) SyncToDB(ctx context.Context) error {
	ocMap := map[string]any{
		"opportunity_id": oc.OpportunityId,
		"name":           oc.Name,
	}

	if oc.Id.IsEmpty() {
		r, err := pgdb.QbQueryRow(ctx, pgdb.Qb.Insert("opportunity_categories").SetMap(ocMap).Suffix("returning id, create_ts"))
		if err != nil {
			return err
		}

		if err = r.Scan(&oc.Id, &oc.CreateTs); err != nil {
			return err
		}
		fmt.Println("cek 1")
	}

	_, err := pgdb.QbExec(ctx, pgdb.Qb.Update("opportunity_categories").SetMap(ocMap).Where("id = ?", oc.Id))
	if err != nil {
		return err
	}

	fmt.Println("cek 2")
	return nil
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
