package user

import (
	"context"
	"net"
	"time"

	"github.com/get10xteam/sales-module-backend/errs"
	"github.com/get10xteam/sales-module-backend/plumbings/config"
	"github.com/get10xteam/sales-module-backend/plumbings/utils"

	"gitlab.com/intalko/gosuite/pgdb"

	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/gofiber/fiber/v2"
	"gitlab.com/benedictjohannes/b64uuid"
)

const sessionCookieKey = "appSession"

var cookieExpiryTimeToSet = time.Date(2006, 01, 02, 0, 0, 0, 0, time.Local)

type userSession struct {
	Id                   b64uuid.B64Uuid      `json:"id" db:"id"`
	UserId               config.ObfuscatedInt `json:"-" db:"user_id"`
	IPAddr               net.IP               `json:"ipAddr" db:"ip_addr"`
	UserAgent            *string              `json:"-" db:"user_agent"`
	UserAgentDescription *string              `json:"device" db:"user_agent_description"`
	CreateTs             time.Time            `json:"createTs" db:"create_ts"`
	ExpiryTs             *time.Time           `json:"expiryTs" db:"expiry_ts"`
	LogOutTs             *time.Time           `json:"logoutTs" db:"logout_ts"`
}

func sessionByStrId(ctx context.Context, strId string, cols ...string) (s *userSession, err error) {
	id, err := b64uuid.Parse(strId)
	if err != nil {
		err = errs.ErrBadParameter().WithDetail(err)
		return
	}
	if len(cols) == 0 {
		cols = []string{"id", "user_id", "expiry_ts", "logout_ts"}
	}
	r, err := pgdb.QbQuery(ctx, pgdb.Qb.Select(cols...).From("user_sessions").Where("id = ?", id))
	if err != nil {
		return
	}
	s = &userSession{}
	err = pgxscan.ScanOne(s, r)
	return
}
func (u *User) NewSession(ctx context.Context, ipAddr net.IP, userAgentStr string, expiryMinutes int) (s *userSession, err error) {
	userAgentDesc := utils.ProcessUserAgent(userAgentStr)
	s = &userSession{
		Id:                   b64uuid.NewRandom(),
		UserId:               u.Id,
		IPAddr:               ipAddr,
		UserAgent:            &userAgentStr,
		UserAgentDescription: &userAgentDesc,
		CreateTs:             time.Now(),
	}
	if expiryMinutes > 0 {
		expiry := s.CreateTs.Add(time.Minute * time.Duration(expiryMinutes))
		s.ExpiryTs = &expiry
	}
	const sql = `insert into user_sessions 
	(id, user_id, ip_addr, user_agent, user_agent_description, create_ts, expiry_ts)
	values ($1,$2,$3,$4,$5,$6,$7)`
	_, err = pgdb.Exec(ctx, sql, s.Id, s.UserId, s.IPAddr, s.UserAgent, s.UserAgentDescription, s.CreateTs, s.ExpiryTs)
	return
}

// to set cookie in http
func (s *userSession) setHTTP(c *fiber.Ctx) {
	cookie := fiber.Cookie{
		Name:     sessionCookieKey,
		SameSite: fiber.CookieSameSiteLaxMode,
		HTTPOnly: true,
		Secure:   true,
		Path:     "/api",
		Value:    s.Id.String(),
	}
	if s.LogOutTs != nil {
		cookie.Expires = cookieExpiryTimeToSet
	} else if s.ExpiryTs != nil {
		cookie.Expires = *s.ExpiryTs
	}
	c.Cookie(&cookie)
}

func (s *userSession) ensureRenewal(c *fiber.Ctx) (err error) {
	if s.ExpiryTs != nil {
		now := time.Now()
		expDur := s.ExpiryTs.Sub(now)
		if time.Duration(defaultSessionExpirationMinutes/2)*time.Minute > expDur {
			ctx := c.Context()
			nextExp := now.Add(time.Duration(defaultSessionExpirationMinutes) * time.Minute)
			const sql = "update user_sessions set expiry_ts = $2 where id = $1"
			_, err = pgdb.Exec(ctx, sql, s.Id, nextExp)
			if err != nil {
				return errs.ErrServerError().WithDetail(err).WithFiberStatus(c)
			}
			s.ExpiryTs = &nextExp
			s.setHTTP(c)
		}
	}
	if s.LogOutTs != nil {
		s.setHTTP(c)
		return errs.ErrUnauthenticated().WithFiberStatus(c)
	}
	return
}
func (s *userSession) logOut(ctx context.Context) (err error) {
	_, err = pgdb.Exec(ctx, "update user_sessions set logout_ts = now() where id = $1", s.Id)
	return
}
