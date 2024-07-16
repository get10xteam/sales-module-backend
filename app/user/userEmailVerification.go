package user

import (
	"context"
	"errors"
	"time"

	"github.com/get10xteam/sales-module-backend/errs"
	"github.com/get10xteam/sales-module-backend/plumbings/config"
	"github.com/get10xteam/sales-module-backend/plumbings/mailer"
	"github.com/get10xteam/sales-module-backend/plumbings/utils"

	"gitlab.com/intalko/gosuite/pgdb"

	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	mail "github.com/xhit/go-simple-mail/v2"
	"gitlab.com/benedictjohannes/b64uuid"
)

type UserEmailVerification struct {
	Id       b64uuid.B64Uuid              `json:"token" db:"id" query:"token"`
	UserId   *config.ObfuscatedInt        `json:"user_id" db:"user_id"`
	Purpose  UserEmailVerificationPurpose `json:"purpose" db:"purpose"`
	CreateTs time.Time                    `json:"createTs" db:"create_ts"`
	ExpiryTs *time.Time                   `json:"expiryTs" db:"expiry_ts"`
	UsedTs   *time.Time                   `json:"usedTs" db:"used_ts"`
	Meta     json.RawMessage              `json:"meta" db:"meta"`
	Password string                       `json:"password" db:"-"`
}

func UserEmailVerificationByPurposeId(ctx context.Context, purpose UserEmailVerificationPurpose, id b64uuid.B64Uuid) (v *UserEmailVerification, err error) {
	v = &UserEmailVerification{}
	const sql = "select id, expiry_ts, used_ts, user_id, meta from user_email_verifications where purpose = $1 and id = $2"
	err = pgdb.QueryRow(ctx, sql, purpose, id).Scan(&v.Id, &v.ExpiryTs, &v.UsedTs, &v.UserId, &v.Meta)
	return
}
func (v *UserEmailVerification) InsertToDB(ctx context.Context) (err error) {
	const sql = `insert into user_email_verifications 
	(id, create_ts, expiry_ts, used_ts, user_id, purpose, meta)
	values ($1,$2,$3,null,$4,$5,$6)
	`
	_, err = pgdb.Exec(ctx, sql, v.Id, v.CreateTs, v.ExpiryTs, v.UserId, v.Purpose, v.Meta)
	return
}
func (v *UserEmailVerification) Update(ctx context.Context, updateMap map[string]any) (err error) {
	if v.Id.IsEmpty() {
		return errs.ErrBadParameter()
	}
	_, err = pgdb.QbExec(ctx, pgdb.Qb.Update("user_email_verifications").SetMap(updateMap).Where("id = ?", v.Id))
	return
}
func (v *UserEmailVerification) MarkUsed(ctx context.Context) (err error) {
	return v.Update(ctx, fiber.Map{"used_ts": time.Now()})
}
func SendUserEmailVerification(ctx context.Context, purpose UserEmailVerificationPurpose, emailAddress string, userId *config.ObfuscatedInt, meta any) (v UserEmailVerification, err error) {
	const emailVerificationExpirationMinutes = 30
	now := time.Now()
	expiry := now.Add(time.Minute * emailVerificationExpirationMinutes)
	v = UserEmailVerification{
		Id:       b64uuid.NewRandom(),
		Purpose:  purpose,
		CreateTs: now,
		ExpiryTs: &expiry,
	}
	if meta != nil {
		v.Meta, err = json.Marshal(meta)
		if err != nil {
			return
		}
	}
	if userId != nil {
		v.UserId = userId
	}
	err = v.InsertToDB(ctx)
	if err != nil {
		return
	}
	email := mailer.NewEmail().
		AddTo(emailAddress).
		SetSubject(purpose.EmailSubject()).
		SetBody(mail.TextHTML, purpose.EmailBody(v.Id))
	go mailer.SendEmail(email)
	return
}

func UserResetPasswordStartHandler(c *fiber.Ctx) (err error) {
	ctx := c.Context()
	f := &struct {
		Email string `json:"email"`
	}{}
	err = c.BodyParser(f)
	if err != nil {
		return errs.ErrBadParameter().WithMessage("Email address must be set").WithFiberStatus(c)
	}
	if f.Email == "" {
		return utils.FiberJSONWrap(c, f.Email)
	}
	u, err := UserByEmail(ctx, f.Email, "id", "email")
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return utils.FiberJSONWrap(c, f.Email)
		}
		return errs.ErrServerError().WithDetail(err).WithFiberStatus(c)
	}
	_, err = SendUserEmailVerification(ctx, UserEmailVerificationPurpose_PasswordReset, u.Email, &u.Id, nil)
	if err != nil {
		return errs.ErrServerError().WithDetail(err).WithFiberStatus(c)
	}
	return utils.FiberJSONWrap(c, f.Email)
}
func UserResetPasswordSubmitHandler(c *fiber.Ctx) (err error) {
	ctx := c.Context()
	v := &UserEmailVerification{}
	if c.Method() == "GET" {
		err = c.QueryParser(v)
		if err != nil {
			return errs.ErrBadParameter().WithDetail(err).WithMessage("Reset token invalid").WithFiberStatus(c)
		}
	} else {
		err = c.BodyParser(v)
		if err != nil {
			return errs.ErrBadParameter().WithDetail(err).WithMessage("Password or reset token invalid").WithFiberStatus(c)
		}
	}
	if v.Id.IsEmpty() {
		return errs.ErrBadParameter().WithMessage("Reset token not set").WithFiberStatus(c)
	}
	if c.Method() == "PUT" && v.Password == "" {
		return errs.ErrBadParameter().WithMessage("Password must be set").WithFiberStatus(c)
	}
	newPassword := v.Password
	v, err = UserEmailVerificationByPurposeId(ctx, UserEmailVerificationPurpose_PasswordReset, v.Id)
	if err != nil {
		if errs.Is(err, pgx.ErrNoRows) {
			return errs.ErrNotExist().WithMessage("Reset token not found").WithFiberStatus(c)
		}
		return errs.ErrServerError().WithDetail(err).WithFiberStatus(c)
	}
	if v.UsedTs != nil {
		return errs.ErrBadParameter().WithMessage("Reset token has been used").WithFiberStatus(c)
	}
	if v.ExpiryTs.Before(time.Now()) {
		return errs.ErrBadParameter().WithMessage("Reset token has expired").WithFiberStatus(c)
	}
	if v.UserId == nil {
		return errs.ErrBadParameter().WithMessage("Reset token not linked to a user").WithFiberStatus(c)
	}
	u, err := UserById(ctx, *v.UserId, "id", "email", "email_confirmed")
	if err != nil {
		return errs.ErrServerError().WithDetail(err).WithFiberStatus(c)
	}
	if c.Method() == "GET" {
		if !u.EmailConfirmed {
			userUpdates := fiber.Map{"email_confirmed": true}
			err = u.UpdateToDB(ctx, userUpdates)
			if err != nil {
				return errs.ErrServerError().WithDetail(err).WithFiberStatus(c)
			}
		}
		return utils.FiberJSONWrap(c, u.Email)
	}
	hashed, err := utils.CheckAndHashPassword(newPassword)
	if err != nil {
		return errs.ErrServerError().WithDetail(err).WithFiberStatus(c)
	}
	userUpdates := fiber.Map{"password": hashed}
	if !u.EmailConfirmed {
		userUpdates["email_confirmed"] = true
	}
	err = u.UpdateToDB(ctx, userUpdates)
	if err != nil {
		return errs.ErrServerError().WithDetail(err).WithFiberStatus(c)
	}
	err = v.MarkUsed(ctx)
	if err != nil {
		return errs.ErrServerError().WithDetail(err).WithFiberStatus(c)
	}
	return utils.FiberJSONWrap(c, u.Email)
}
