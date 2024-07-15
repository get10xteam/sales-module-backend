package client

import (
	"context"
	"time"

	"github.com/get10xteam/sales-module-backend/plumbings/config"

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
