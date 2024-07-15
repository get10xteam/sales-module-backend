package opportunity

import (
	"context"
	"time"

	"github.com/get10xteam/sales-module-backend/errs"
	"github.com/get10xteam/sales-module-backend/plumbings/config"
	"github.com/get10xteam/sales-module-backend/plumbings/utils"

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

func (u *Opportunity) CreateToDB(ctx context.Context) (err error) {
	return
}

type CreateOpportunitiesPayload struct {
	OwnerId         config.ObfuscatedInt `json:"owner_id,omitempty"`    // check users
	AssigneeId      config.ObfuscatedInt `json:"assignee_id,omitempty"` // check users
	ClientId        config.ObfuscatedInt `json:"client_id,omitempty"`   // check clients
	StatusCode      string               `json:"status_code,omitempty"` // check statuses
	Name            string               `json:"name,omitempty"`
	Description     *string              `json:"description,omitempty"`
	TalentBudget    int                  `json:"talent_budget,omitempty"`
	NonTalentBudget int                  `json:"non_talent_budget,omitempty"`
	Revenue         int                  `json:"revenue,omitempty"`
}

func CreateOpportunities(c *fiber.Ctx) error {
	opportunityReq := CreateOpportunitiesPayload{}
	err := c.BodyParser(&opportunityReq)
	if err != nil {
		return errs.ErrBadParameter().WithMessage("Body not valid").WithFiberStatus(c)
	}

	return utils.FiberJSONWrap(c, opportunityReq)
}
