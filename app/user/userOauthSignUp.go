package user

import (
	"errors"
	"net"
	stdMail "net/mail"
	"time"

	"github.com/get10xteam/sales-module-backend/errs"
	"github.com/get10xteam/sales-module-backend/plumbings/oauth"
	"github.com/get10xteam/sales-module-backend/plumbings/utils"

	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"gitlab.com/benedictjohannes/b64uuid"
)

func UserOauthLoginGoogleHandler(c *fiber.Ctx) (err error) {
	ctx := c.Context()
	o, err := oauth.ObtainGoogleIdTokenFromPost(c)
	if err != nil {
		return
	}
	u, err := UserByEmail(ctx, *o.Email, "id", "email", "email_confirmed", "name", "profile_img_url")
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			u = &User{
				Email: *o.Email,
				Name:  o.Name,
			}
			err = u.CreateToDB(ctx)
			if err != nil {
				return errs.ErrServerError().WithDetail(err)
			}
			goto PROCEED
		}
		return errs.ErrServerError().WithDetail(err)
	}
PROCEED:
	s, err := u.NewSession(ctx, net.ParseIP(c.IP()), string(c.Context().UserAgent()), defaultSessionExpirationMinutes)
	if err != nil {
		return errs.ErrServerError().WithDetail(err)
	}
	s.setHTTP(c)

	return utils.FiberJSONWrap(c, fiber.Map{"profile": u, "destinationUrl": o.DestinationURL})
}
func UserOauthLoginMicrosoftHandler(c *fiber.Ctx) (err error) {
	ctx := c.Context()
	o, err := oauth.ObtainMicrosoftIdTokenFromPost(c)
	if err != nil {
		return
	}
	u, err := UserByEmail(ctx, *o.Email, "id", "email", "emailconfirmed", "name", "profileimg_url")
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			u = &User{
				Email: *o.Email,
				Name:  o.Name,
			}
			err = u.CreateToDB(ctx)
			if err != nil {
				return errs.ErrServerError().WithDetail(err)
			}
			goto PROCEED
		}
		return errs.ErrServerError().WithDetail(err)
	}
PROCEED:
	s, err := u.NewSession(ctx, net.ParseIP(c.IP()), string(c.Context().UserAgent()), defaultSessionExpirationMinutes)
	if err != nil {
		return errs.ErrServerError().WithDetail(err)
	}
	s.setHTTP(c)

	return utils.FiberJSONWrap(c, fiber.Map{"profile": u, "destinationUrl": o.DestinationURL})
}

type userSignUpPayload struct {
	Email    string `json:"email,omitempty"`
	Password string `json:"password,omitempty"`
	Name     string `json:"name,omitempty"`
}

func UserSignUpHandler(c *fiber.Ctx) (err error) {
	ctx := c.Context()
	var s userSignUpPayload
	err = c.BodyParser(&s)
	if err != nil {
		return errs.ErrBadParameter().WithDetail(err)
	}
	_, err = stdMail.ParseAddress(s.Email)
	if err != nil {
		return errs.ErrBadParameter().WithDetail(err).WithMessage("Invalid Email Address")
	}
	_, err = UserByEmail(ctx, s.Email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			goto PROCEED
		}
		return errs.ErrServerError().WithDetail(err)
	} else {
		return errs.ErrPreexist()
	}
PROCEED:
	if len(s.Name) < 2 {
		return errs.ErrBadParameter().WithDetail("name length too short")
	}
	s.Password, err = utils.CheckAndHashPassword(s.Password)
	if err != nil {
		return errs.ErrServerError().WithDetail(err)
	}
	_, err = SendUserEmailVerification(ctx, UserEmailVerificationPurpose_SignUpVerify, s.Email, nil, s)
	if err != nil {
		return errs.ErrServerError().WithDetail(err)
	}
	return utils.FiberJSONWrap(c, s.Email)
}
func UserSignUpVerifyHandler(c *fiber.Ctx) (err error) {
	ctx := c.Context()
	token := c.Query("token")
	verifyId, err := b64uuid.Parse(token)
	if err != nil || verifyId.IsEmpty() {
		return errs.ErrBadParameter().WithMessage("Verification token not found")
	}
	v, err := UserEmailVerificationByPurposeId(ctx, UserEmailVerificationPurpose_SignUpVerify, verifyId)
	if err != nil {
		if errs.Is(err, pgx.ErrNoRows) {
			return errs.ErrNotExist().WithMessage("Verification token not found")
		}
		return errs.ErrServerError().WithDetail(err)
	}
	if v.ExpiryTs.Before(time.Now()) {
		return errs.ErrBadParameter().WithMessage("Verification token has expired")
	}
	if v.UsedTs != nil {
		return errs.ErrBadParameter().WithMessage("Verification token has been used")
	}
	var s userSignUpPayload
	err = json.Unmarshal(v.Meta, &s)
	if err != nil {
		return errs.ErrServerError().WithDetail(err)
	}
	_, err = UserByEmail(ctx, s.Email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			goto PROCEED
		}
		return errs.ErrServerError().WithDetail(err)
	} else {
		return errs.ErrPreexist()
	}
PROCEED:
	err = v.MarkUsed(ctx)
	if err != nil {
		return errs.ErrServerError().WithDetail(err)
	}
	u := User{
		EmailConfirmed: true,
		Email:          s.Email,
		Name:           &s.Name,
		Password:       &s.Password,
	}
	err = u.CreateToDB(ctx)
	if err != nil {
		return errs.ErrServerError().WithDetail(err)
	}
	return utils.FiberJSONWrap(c, s.Email)
}
