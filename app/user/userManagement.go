package user

import (
	"context"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/get10xteam/sales-module-backend/errs"
	"github.com/get10xteam/sales-module-backend/plumbings/config"
	"github.com/get10xteam/sales-module-backend/plumbings/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"gitlab.com/intalko/gosuite/pgdb"
)

type UserSearchQuery struct {
	Search             string               `query:"search"`
	IncludeDeactivated bool                 `query:"includeDeactivated"`
	IncludeRefs        bool                 `query:"includeRefs"`
	ParentUserID       config.ObfuscatedInt `query:"parentUserID"`
	Page               uint64               `query:"page"`
	PageSize           uint64               `query:"pageSize"`
	q                  squirrel.SelectBuilder
	OrderBy            string `query:"orderBy"`
	OrderDesc          bool   `query:"orderDesc"`
}

func (usq *UserSearchQuery) Apply() {
	usq.q = pgdb.Qb.Select(
		"u.id",
		"u.name",
		"u.email",
		"u.create_ts",
		"u.level_id",
	).From("users as u")

	if !usq.ParentUserID.IsEmpty() {
		usq.q = usq.q.Where(`u.id IN (
		WITH RECURSIVE new_users AS (
			SELECT 
				u.id
			FROM 
				users u 
			WHERE 
				u.id = ?
			UNION 
			SELECT 
				us.id
			FROM 
				users us 
			INNER JOIN new_users so ON so.id = us.parent_id
				) 
			SELECT * FROM new_users as nu
		)`, usq.ParentUserID)
	}

	if usq.IncludeRefs {
		usq.q = usq.q.LeftJoin("users as user_parent on user_parent.parent_id = u.id").
			Columns("user_parent.name as parent_name").
			Columns("user_parent.id as parent_id")
	}

	if usq.Search != "" {
		search := "%" + usq.Search + "%"
		usq.q = usq.q.Where(squirrel.Or{
			squirrel.Expr("u.name ilike ?", search),
			squirrel.Expr("email ilike ?", search),
		})
	}

	if usq.IncludeDeactivated {
		usq.q = usq.q.Where("u.deactivated_ts is not null").Columns("u.deactivated_ts")
	}

	switch usq.OrderBy {
	case "name":
		usq.OrderBy = "u.name"
	case "email":
		usq.OrderBy = "u.email"
	case "createTs":
		usq.OrderBy = "u.create_ts"
	default:
		usq.OrderBy = "u.id"
	}

	orderClause := usq.OrderBy
	if usq.OrderDesc {
		orderClause = orderClause + " desc"
	} else {
		orderClause = orderClause + " asc"
	}

	usq.q = usq.q.OrderBy(orderClause).GroupBy(
		"u.id",
		"u.name",
		"u.name",
		"u.create_ts",
	)
}

func (usq *UserSearchQuery) GetData(ctx context.Context) ([]*User, error) {
	if usq.PageSize == 0 {
		usq.PageSize = 20
	}

	if usq.Page == 0 {
		usq.Page = 1
	}

	offset := usq.PageSize * (usq.Page - 1)
	usq.q = usq.q.Offset(offset)
	usq.q = usq.q.Limit(usq.PageSize)
	sql, _, _ := usq.q.ToSql()
	fmt.Println(sql)

	r, err := pgdb.QbQuery(ctx, usq.q)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	// users := []*User{}

	// for r.Next() {
	// 	var u User
	// 	err = pgxscan.ScanRow(&u, r)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	users = append(users, &u)
	// }

	users, err := pgx.CollectRows(r, func(row pgx.CollectableRow) (*User, error) {
		var o User
		err := usq.scanSimple(r, &o)
		if err != nil {
			return nil, err
		}

		return &o, nil
	})

	if err != nil {
		return nil, err
	}

	return users, nil
}

func (s *UserSearchQuery) scanSimple(r pgx.Rows, u *User) error {
	return r.Scan(
		&u.Id,
		&u.Name,
		&u.Email,
		&u.CreateTs,
		&u.LevelId,
	)
}

func ListUsersHandler(c *fiber.Ctx) (err error) {

	u := UserFromHttp(c)

	query := UserSearchQuery{}
	err = c.QueryParser(&query)
	if err != nil {
		return errs.ErrServerError().WithDetail(err)
	}

	query.ParentUserID = u.Id
	query.IncludeDeactivated = false
	query.IncludeRefs = false

	ctx := c.Context()
	query.Apply()

	res, err := query.GetData(ctx)
	if err != nil {
		return errs.ErrServerError().WithDetail(err)
	}

	return utils.FiberJSONWrap(c, res)
}
