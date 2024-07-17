package user

import (
	"context"
	"fmt"
	"strings"

	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/get10xteam/sales-module-backend/errs"
	"github.com/get10xteam/sales-module-backend/plumbings/config"
	"github.com/get10xteam/sales-module-backend/plumbings/utils"
	"github.com/gofiber/fiber/v2"
	"gitlab.com/intalko/gosuite/pgdb"
)

/* to simulate levels
INSERT INTO levels("name") VALUES('Director');
INSERT INTO levels("name") VALUES('Director of Operation');
INSERT INTO levels("name") VALUES('Manager');
INSERT INTO levels("name") VALUES('Sales');
*/

/*
	TODO

1. Having a full user listing should not be allowed.
Pagination should be on by default, unless getAll is supplied in the search query
2. So we should create `type UserSearchQuery` to allow for better flexibility
Why is this required?
Because from HTTP/handler level, we can read the active user
Don't forget because of the hierarchical rule,
the user is only allowed to chose any user that is below the user hierarchically
(use where in a WITH RECURSIVE ... SELECT ... UNION query)

The only place where the user is allowed to load all users in the organization
would be for user management

DeactivatedTs should be also included with includeDeactivated queryParam
*/

type UserSearchQuery struct {
	Search             string               `query:"search"`
	IncludeDeactivated bool                 `query:"includeDeactivated"`
	IncludeRefs        bool                 `query:"includeRefs"`
	UserID             config.ObfuscatedInt `query:"userID"`
	ParentUserID       config.ObfuscatedInt `query:"parentUserID"`
	q                  string
	params             []any
}

func (usq *UserSearchQuery) Apply() {
	usq.params = make([]any, 0)

	query := `SELECT 
				nu.id, 
				nu.name, 
				nu.email, 
				nu.create_ts,
				nu.deactivated_ts, 
				nu.parent_id 
			FROM 
				users as nu`

	if !usq.ParentUserID.IsEmpty() {
		query = `WITH RECURSIVE new_users AS (
			SELECT 
				u.id, 
				u.name, 
				u.email, 
				u.create_ts,
				u.deactivated_ts, 
				u.parent_id 
			FROM 
				users u 
			WHERE 
				u.id = ?
			UNION 
			SELECT 
				us.id, 
				us.name, 
				us.email, 
				us.create_ts,
				us.deactivated_ts, 
				us.parent_id
			FROM 
				users us 
			INNER JOIN new_users so ON so.id = us.parent_id
				) 
			SELECT * FROM new_users as nu`
		usq.params = append(usq.params, usq.ParentUserID)
	}

	whereClause := ""

	if usq.IncludeRefs {
		whereClause = usq.WhereClause(whereClause, " nu.id = ?")
		usq.params = append(usq.params, usq.UserID)
	}

	if usq.Search != "" {
		search := "%" + usq.Search + "%"
		whereClause = usq.WhereClause(whereClause, "(nu.name ilike ? or nu.email ilike ?)")
		usq.params = append(usq.params, search, search)
	}

	if !usq.IncludeDeactivated {
		whereClause = usq.WhereClause(whereClause, "nu.deactivated_ts is null")
	}

	usq.q = query + whereClause
	usq.ReplaceQMarks()
}

func (usq *UserSearchQuery) WhereClause(current, newString string) string {
	if current == "" {
		return " where " + newString
	}

	return current + " and " + newString
}

func (usq *UserSearchQuery) ReplaceQMarks() {
	num := 1
	for strings.Contains(usq.q, "?") {
		usq.q = strings.Replace(usq.q, "?", fmt.Sprintf("$%d", num), 1)
		num = num + 1
	}
}

func (usq *UserSearchQuery) GetData(ctx context.Context) ([]*User, error) {
	r, err := pgdb.Query(ctx, usq.q, usq.params...)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	users := []*User{}

	for r.Next() {
		var u User
		err = pgxscan.ScanRow(&u, r)
		if err != nil {
			return nil, err
		}
		users = append(users, &u)
	}

	return users, nil
}

func UserHierarchyDropdownHandler(c *fiber.Ctx) (err error) {

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
