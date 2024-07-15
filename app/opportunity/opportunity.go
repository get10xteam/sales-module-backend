package opportunity

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

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
	OwnerId                         config.ObfuscatedInt `json:"owner_id" db:"owner_id"`
	AssigneeId                      config.ObfuscatedInt `json:"assignee_id" db:"assignee_id"`
	ClientId                        config.ObfuscatedInt `json:"client_id" db:"client_id"`
	StatusCode                      string               `json:"status_code" db:"status_code"`
	Name                            string               `json:"name" db:"name"`
	Description                     *string              `json:"description,omitempty" db:"description"`
	TalentBudget                    int                  `json:"talent_budget" db:"talent_budget"`
	NonTalentBudget                 int                  `json:"non_talent_budget" db:"non_talent_budget"`
	Revenue                         int                  `json:"revenue" db:"revenue"`
	ExpectedProfitabilityPercentage float64              `json:"expected_profitability_percentage" db:"expected_profitability_percentage"`
	ExpectedProfitabilityAmount     int                  `json:"expected_profitability_amount" db:"expected_profitability_amount"`
	CreateTs                        time.Time            `json:"createTs" db:"create_ts"`
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
	OwnerId         config.ObfuscatedInt `json:"owner_id,omitempty"`    // check users
	AssigneeId      config.ObfuscatedInt `json:"assignee_id,omitempty"` // check users
	ClientId        config.ObfuscatedInt `json:"client_id,omitempty"`   // check clients
	StatusCode      string               `json:"status_code,omitempty"` // check statuses
	Name            string               `json:"name"`
	Description     *string              `json:"description,omitempty"`
	TalentBudget    int                  `json:"talent_budget,omitempty"`
	NonTalentBudget int                  `json:"non_talent_budget,omitempty"`
	Revenue         int                  `json:"revenue,omitempty"`
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

func CreateOpportunity(c *fiber.Ctx) error {
	opportunityReq := CreateOpportunityPayload{}
	err := c.BodyParser(&opportunityReq)
	if err != nil {
		return errs.ErrBadParameter().WithMessage("Body not valid").WithFiberStatus(c)
	}

	ctx := c.Context()
	if err := opportunityReq.Validate(ctx); err != nil {
		return err.WithFiberStatus(c)
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
		return errs.ErrServerError().WithDetail(err).WithFiberStatus(c)
	}

	return utils.FiberJSONWrapWithStatusCreated(c, map[string]config.ObfuscatedInt{
		"id": opportunity.Id,
	})
}
