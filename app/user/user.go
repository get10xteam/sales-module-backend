package user

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/get10xteam/sales-module-backend/errs"
	"github.com/get10xteam/sales-module-backend/plumbings/config"
	"github.com/get10xteam/sales-module-backend/plumbings/storage"
	"github.com/get10xteam/sales-module-backend/plumbings/utils"

	"gitlab.com/intalko/gosuite/pgdb"

	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"gitlab.com/benedictjohannes/b64uuid"
)

const defaultSessionExpirationMinutes = 30
const userIdLocalsKey = "localsAppUsr"

type User struct {
	Id             config.ObfuscatedInt `json:"id" db:"id"`
	EmailConfirmed bool                 `json:"emailConfirmed,omitempty" db:"email_confirmed"`
	Email          string               `json:"email" db:"email"`
	Password       *string              `json:"-" db:"password"`
	Name           *string              `json:"name,omitempty" db:"name"`
	ProfileImgUrl  *string              `json:"profileImgUrl,omitempty" db:"profile_img_url"`
	CreateTs       time.Time            `json:"createTs,omitempty" db:"create_ts"`
	DeactivatedTs  *time.Time           `json:"deactivatedTs,omitempty" db:"deactivated_ts"`
	LevelId        *int                 `json:"levelId" db:"level_id"`
	// only appear when IncludeRefs is true
	ParentId   *config.ObfuscatedInt `json:"parentId,omitempty" db:"parent_id"`
	ParentName *string               `json:"parentName,omitempty" db:"parent_name"`
}

// level id on created user must be calculated based on the next level
func (u *User) CreateToDB(ctx context.Context) (err error) {
	insertMap := map[string]any{
		"email_confirmed": u.EmailConfirmed,
		"email":           u.Email,
	}
	if u.Name == nil {
		var n string
		insertMap["name"] = &n
	} else {
		insertMap["name"] = u.Name
	}
	if u.Password != nil {
		insertMap["password"] = u.Password
	}
	if u.ProfileImgUrl != nil {
		insertMap["profile_img_url"] = u.ProfileImgUrl
	}
	/* TODO
	ParentId HARUS dikirim dari frontend, it's NOT optional, kecuali mau bikin TOP hierarchical level
	Kalau ParentId nya nil, berarti dari frontend levelnya harus set level tertinggi (100)
	Kalau bukan bikin level tertinggi (ParentId != nil), kita load parent user nya. Baca level nya dan obtain lower level.
	Kalau lower level not exist, error out!
	*/

	if u.ParentId.IsEmpty() {
		// if parent id is exist, ambil data user nya lalu search data lower levelnya, kalo kosong kasih error
		rLevel, err := pgdb.QbQueryRow(ctx, pgdb.Qb.Select("level_id").From("users").Where("id = ?", u.ParentId))
		if err != nil {
			return err
		}

		var parentLevelID int
		err = rLevel.Scan(&parentLevelID)
		if err != nil {
			return err
		}

		rNextLevel := pgdb.QueryRow(ctx, `SELECT next_level FROM ( SELECT id, lead(id) OVER w as next_level FROM levels l WINDOW w AS (ORDER BY id) ) s WHERE id = $1`, parentLevelID)
		err = rNextLevel.Scan(&u.LevelId)
		if err != nil {
			return err
		}

		if u.LevelId == nil {
			return errors.New("error when obtaining level")
		}

		insertMap["level_id"] = u.LevelId
		insertMap["parent_id"] = u.ParentId

	} else {
		// if parent id is empty set into highest level in db
		rLevel, err := pgdb.QbQueryRow(ctx, pgdb.Qb.Select("id").From("levels").OrderBy("id asc").Limit(1))
		if err != nil {
			return err
		}

		err = rLevel.Scan(&u.LevelId)
		if err != nil {
			return err
		}

		insertMap["level_id"] = u.LevelId
	}

	r, err := pgdb.QbQueryRow(ctx, pgdb.Qb.Insert("users").SetMap(insertMap).Suffix("returning id"))
	if err != nil {
		return
	}
	err = r.Scan(&u.Id)
	return
}

func (u *User) UpdateToDB(ctx context.Context, updateMap map[string]any) (err error) {
	if u.Id.IsEmpty() {
		return errs.ErrBadParameter()
	}
	_, err = pgdb.QbExec(ctx, pgdb.Qb.Update("users").SetMap(updateMap).Where("id = ?", u.Id))
	return
}

// UserByEmail returns only the ID column by default. To get more, specify columns to be included
func UserByEmail(ctx context.Context, email string, cols ...string) (u *User, err error) {
	if len(cols) == 0 {
		cols = []string{"id"}
	}
	r, err := pgdb.QbQuery(ctx, pgdb.Qb.Select(cols...).From("users").Where("email = ?", email))
	if err != nil {
		return
	}
	u = &User{}
	err = pgxscan.ScanOne(u, r)
	if err != nil {
		return
	}
	return u, err
}

func UserById(ctx context.Context, id config.ObfuscatedInt, cols ...string) (u *User, err error) {
	if len(cols) == 0 {
		cols = []string{"id"}
	}
	r, err := pgdb.QbQuery(ctx, pgdb.Qb.Select(cols...).From("users").Where("id = ?", id))
	if err != nil {
		return
	}
	u = &User{}
	err = pgxscan.ScanOne(u, r)
	if err != nil {
		return
	}
	return u, err
}

type UserEmailVerificationPurpose int

const (
	UserEmailVerificationPurpose_PasswordReset UserEmailVerificationPurpose = 0
	UserEmailVerificationPurpose_SignUpVerify  UserEmailVerificationPurpose = 1
)

func (p UserEmailVerificationPurpose) EmailSubject() string {
	switch p {
	case UserEmailVerificationPurpose_PasswordReset:
		return config.Config.Runtime.AppName + ": Reset Your Password"
	case UserEmailVerificationPurpose_SignUpVerify:
		return config.Config.Runtime.AppName + ": Verify Your Account"
	}
	return ""
}

func (p UserEmailVerificationPurpose) EmailBody(resetId b64uuid.B64Uuid) string {
	switch p {
	case UserEmailVerificationPurpose_PasswordReset:
		url := config.Config.DeploymentURLs.UsrResetPassword + resetId.String()
		return "<p>To reset your password, please <a href=\"" + url + "\">click this link</a>.</p>" +
			"<p>Alternatively, copy this URL to your browser: <br></br>" +
			url +
			"</p>"
	case UserEmailVerificationPurpose_SignUpVerify:
		url := config.Config.DeploymentURLs.UsrSignUpVerify + resetId.String()
		return "<p>Your email address is being registered to 10x.</p><p>To continue your account registration, please <a href=\"" + url + "\">click this link</a>.</p>" +
			"<p>Alternatively, copy this URL to your browser: <br></br>" +
			url +
			"</p>"
	}
	return ""
}

func UserLoginHandler(c *fiber.Ctx) (err error) {
	ctx := c.Context()
	var f struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Remember bool   `json:"remember"`
	}
	err = c.BodyParser(&f)
	if err != nil {
		return errs.ErrBadParameter().WithMessage("Body not valid")
	}
	if f.Email == "" || f.Password == "" {
		return errs.ErrBadParameter().WithMessage("Email and password must be set")
	}
	u, err := UserByEmail(ctx, f.Email, "id", "email_confirmed", "email", "name", "password", "profile_img_url")
	if err != nil || u == nil || u.Password == nil {
		if errors.Is(err, pgx.ErrNoRows) || u == nil || u.Password == nil {
			return errs.ErrInvalidUser()
		}
		return errs.ErrServerError().WithDetail(err)
	}
	err = utils.VerifyPassword(*u.Password, f.Password)
	if err != nil {
		return errs.ErrInvalidUser()
	}
	var expiry int
	if !f.Remember {
		expiry = defaultSessionExpirationMinutes
	}
	s, err := u.NewSession(ctx, net.ParseIP(c.IP()), string(c.Context().UserAgent()), expiry)
	if err != nil {
		return errs.ErrServerError().WithDetail(err)
	}
	s.setHTTP(c)

	return utils.FiberJSONWrap(c, u)
}

func LoadAuthMiddleware(c *fiber.Ctx) (err error) {
	ctx := c.Context()
	sStr := c.Cookies(sessionCookieKey)
	if sStr == "" {
		return c.Next()
	}
	s, err := sessionByStrId(ctx, sStr)
	if err != nil {
		return c.Next()
	}
	if s.LogOutTs != nil || s.ExpiryTs != nil && s.ExpiryTs.Before(time.Now()) {
		c.ClearCookie(sessionCookieKey)
		return c.Next()
	}
	err = s.ensureRenewal(c)
	if err != nil {
		return
	}
	u, err := UserById(ctx, s.UserId)
	if err != nil {
		return c.Next()
	}
	c.Locals(userIdLocalsKey, u)
	return c.Next()
}

func MustAuthMiddleware(c *fiber.Ctx) (err error) {
	ctx := c.Context()
	sStr := c.Cookies(sessionCookieKey)
	if sStr == "" {
		return errs.ErrUnauthenticated()
	}
	s, err := sessionByStrId(ctx, sStr)
	if err != nil {
		return errs.ErrUnauthenticated()
	}
	if s.LogOutTs != nil || s.ExpiryTs != nil && s.ExpiryTs.Before(time.Now()) {
		c.ClearCookie(sessionCookieKey)
		return errs.ErrUnauthenticated()
	}
	err = s.ensureRenewal(c)
	if err != nil {
		return
	}
	// check if user exist or not
	u, err := UserById(ctx, s.UserId)
	if err != nil {
		return errs.ErrServerError().WithDetail(err)
	}
	c.Locals(userIdLocalsKey, u)
	return c.Next()
}

func UserProfileHandler(c *fiber.Ctx) (err error) {
	u := c.Locals(userIdLocalsKey).(*User)
	u, err = UserById(c.Context(), u.Id, "id", "email", "email_confirmed", "name", "profile_img_url")
	if err != nil {
		return errs.ErrServerError().WithDetail(err)
	}

	return utils.FiberJSONWrap(c, u)
}

func ChangeProfileHandler(c *fiber.Ctx) (err error) {
	u := c.Locals(userIdLocalsKey).(*User)
	toUpdate := make(map[string]any)
	profileImgUrl := storage.GetUploadedUrlFromHttp(c)
	if len(profileImgUrl) > 0 {
		toUpdate["profile_img_url"] = profileImgUrl
		u.ProfileImgUrl = &profileImgUrl
		err = u.UpdateToDB(c.Context(), toUpdate)
		if err != nil {
			return errs.ErrServerError().WithDetail(err)
		}
		return utils.FiberJSONWrap(c, u)
	}
	updates := make(map[string]any)
	err = c.BodyParser(&updates)
	if err != nil {
		return errs.ErrServerError().WithDetail(err)
	}
	if name, ok := updates["name"].(string); ok {
		toUpdate["name"] = name
		u.Name = &name
		err = u.UpdateToDB(c.Context(), toUpdate)
		if err != nil {
			return errs.ErrServerError().WithDetail(err)
		}
	}
	return utils.FiberJSONWrap(c, u)
}

func UserLogoutHandler(c *fiber.Ctx) (err error) {
	ctx := c.Context()
	k := c.Cookies(sessionCookieKey)
	if k == "" {
		return utils.FiberJSONWrap(c, true)
	}
	s, err := sessionByStrId(ctx, k)
	if err != nil {
		c.ClearCookie(sessionCookieKey)
		return utils.FiberJSONWrap(c, true)
	}
	err = s.logOut(ctx)
	if err != nil {
		return errs.ErrServerError().WithDetail(err)
	}
	s.setHTTP(c)
	return utils.FiberJSONWrap(c, true)
}

// Valid only after MustAuthMiddleware
func UserFromHttp(c *fiber.Ctx) *User {
	u, ok := c.Locals(userIdLocalsKey).(*User)
	if ok {
		return u
	}
	return nil
}
