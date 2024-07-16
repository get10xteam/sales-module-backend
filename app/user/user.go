package user

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/Masterminds/squirrel"
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
	/* TODO
		ALLOW user to login and access their profile (loadAuth)
		but DISALLOW user do ANY OTHER API CALL (MustAuthMiddleware)
	 */
	DeactivatedTs  *time.Time            `json:"deactivatedTs" db:"deactivatedts"`
	/* TODO
	I'd imagine that on the user list, the showing Level I, Level II would be nice
	We shouldn't need to develop CREATE/UPDATE APIs for level, but we should develop List APIs
	That way, the list of levels can be loaded by frontend and loaded into ReactContext
	and matched to be displayed to the users
	*/
	LevelId int `json:"levelId"`
	/* TODO
	use left join on the query builder for listing to always load the user's parent's name.
	Of course it doesn't always have to be loaded, such in user drop down.
	Which is why we need `type UserSearchQuery`.
	I'd suggest define this outside of this file (user.go), as in userManagement.go
	*/
	ParentUserId   *int    `json:"parentUserId,omitempty"`
	ParentUserName *string `json:"parentUserName,omitempty"`
}

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
*/
func UserDropdown(ctx context.Context, search string) ([]*User, error) {
	selectBuilder := pgdb.Qb.Select("id", "name", "email", "create_ts").From("users")

	if search != "" {
		search = "%" + search + "%"
		selectBuilder = selectBuilder.Where(squirrel.Or{
			squirrel.Expr("name ilike ?", search), squirrel.Expr("email ilike ?", search),
		})
	}

	r, err := pgdb.QbQuery(ctx, selectBuilder)
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

	return users, err
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
		return errs.ErrBadParameter().WithMessage("Body not valid").WithFiberStatus(c)
	}
	if f.Email == "" || f.Password == "" {
		return errs.ErrBadParameter().WithMessage("Email and password must be set").WithFiberStatus(c)
	}
	u, err := UserByEmail(ctx, f.Email, "id", "email_confirmed", "email", "name", "password", "profile_img_url")
	if err != nil || u == nil || u.Password == nil {
		if errors.Is(err, pgx.ErrNoRows) || u == nil || u.Password == nil {
			return errs.ErrInvalidUser().WithFiberStatus(c)
		}
		return errs.ErrServerError().WithDetail(err).WithFiberStatus(c)
	}
	err = utils.VerifyPassword(*u.Password, f.Password)
	if err != nil {
		return errs.ErrInvalidUser().WithFiberStatus(c)
	}
	var expiry int
	if !f.Remember {
		expiry = defaultSessionExpirationMinutes
	}
	s, err := u.NewSession(ctx, net.ParseIP(c.IP()), string(c.Context().UserAgent()), expiry)
	if err != nil {
		return errs.ErrServerError().WithDetail(err).WithFiberStatus(c)
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
		return errs.ErrUnauthenticated().WithFiberStatus(c)
	}
	s, err := sessionByStrId(ctx, sStr)
	if err != nil {
		return errs.ErrUnauthenticated().WithFiberStatus(c)
	}
	if s.LogOutTs != nil || s.ExpiryTs != nil && s.ExpiryTs.Before(time.Now()) {
		c.ClearCookie(sessionCookieKey)
		return errs.ErrUnauthenticated().WithFiberStatus(c)
	}
	err = s.ensureRenewal(c)
	if err != nil {
		return
	}
	// check if user exist or not
	u, err := UserById(ctx, s.UserId)
	if err != nil {
		return errs.ErrServerError().WithDetail(err).WithFiberStatus(c)
	}
	c.Locals(userIdLocalsKey, u)
	return c.Next()
}
func UserProfileHandler(c *fiber.Ctx) (err error) {
	u := c.Locals(userIdLocalsKey).(*User)
	u, err = UserById(c.Context(), u.Id, "id", "email", "email_confirmed", "name", "profile_img_url")
	if err != nil {
		return errs.ErrServerError().WithDetail(err).WithFiberStatus(c)
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
			return errs.ErrServerError().WithDetail(err).WithFiberStatus(c)
		}
		return utils.FiberJSONWrap(c, u)
	}
	updates := make(map[string]any)
	err = c.BodyParser(&updates)
	if err != nil {
		return errs.ErrServerError().WithDetail(err).WithFiberStatus(c)
	}
	if name, ok := updates["name"].(string); ok {
		toUpdate["name"] = name
		u.Name = &name
		err = u.UpdateToDB(c.Context(), toUpdate)
		if err != nil {
			return errs.ErrServerError().WithDetail(err).WithFiberStatus(c)
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
		return errs.ErrServerError().WithDetail(err).WithFiberStatus(c)
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

func UserDropdownHandler(c *fiber.Ctx) (err error) {

	search := c.Query("search", "")

	users, err := UserDropdown(c.Context(), search)
	if err != nil {
		return errs.ErrServerError().WithDetail(err).WithFiberStatus(c)
	}

	return utils.FiberJSONWrap(c, users)
}
