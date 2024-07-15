package oauth

import (
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/get10xteam/sales-module-backend/errs"
	"github.com/get10xteam/sales-module-backend/plumbings/config"
	"github.com/get10xteam/sales-module-backend/plumbings/utils"

	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/valyala/fasthttp"
)

func MicrosoftGetSignInUrlHandler(c *fiber.Ctx) (err error) {
	var uri fasthttp.URI
	var noReferer bool
	if err = uri.Parse(nil, c.Context().Referer()); err != nil {
		uri = *c.Context().URI()
		noReferer = true
	}
	redirectUri := string(uri.Scheme()) + "://" + string(uri.Host()) + "/oauth/microsoft"
	oa := OauthAuthorization{Provider: MicrosoftProvider}
	destinationUrl := c.Query("destinationUrl")
	if destinationUrl == "" {
		if !noReferer {
			urlPath := string(uri.RequestURI())
			if strings.HasPrefix(urlPath, "/oauth") {
				urlPath = "/"
			}
			oa.DestinationURL = &urlPath
		}
	} else {
		oa.DestinationURL = &destinationUrl
	}
	err = oa.CreateSaveToDB(c.Context())
	if err != nil {
		return
	}
	v := url.Values{
		"client_id":     {config.Config.Auth.MicrosoftAuth.ClientId},
		"response_type": {"code"},
		"redirect_uri":  {redirectUri},
		"scope":         {"openid email profile"},
		"state":         {oa.State.String()},
		"nonce":         {oa.State.String()},
	}
	authUrl := "https://login.microsoftonline.com/common/oauth2/v2.0/authorize?" + v.Encode()

	return utils.FiberJSONWrap(c, authUrl)
}

func ObtainMicrosoftIdTokenFromPost(c *fiber.Ctx) (o *OauthAuthorization, err error) {
	ctx := c.Context()
	var p OauthAuthorization
	err = c.BodyParser(&p)
	if err != nil {
		err = errs.ErrBadParameter().WithDetail(err)
		return
	}
	if len(p.Code) == 0 || len(p.State) == 0 {
		err = errs.ErrBadParameter()
		return
	}
	o, err = OauthAuthorizationById(ctx, MicrosoftProvider, p.State)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = errs.ErrNotExist()
			return
		}
	}
	if o.ExchangeTs != nil {
		err = errs.ErrBadParameter().WithMessage("authorization code has been used")
		return
	}
	now := time.Now()
	if o.Expiry.Before(now) {
		err = errs.ErrBadParameter().WithMessage("authorization code has expired")
		return
	}
	msAuth := config.Config.Auth.MicrosoftAuth
	token, err := msAuth.GetToken()
	if err != nil {
		return
	}
	var uri fasthttp.URI
	if err = uri.Parse(nil, c.Context().Referer()); err != nil {
		err = errs.ErrBadParameter().WithMessage("illegal referer header")
		return
	}
	redirectUri := string(uri.Scheme()) + "://" + string(uri.Host()) + "/oauth/microsoft"
	v := url.Values{
		"client_id":             {msAuth.ClientId},
		"scope":                 {"openid email profile"},
		"code":                  {p.Code},
		"redirect_uri":          {redirectUri},
		"grant_type":            {"authorization_code"},
		"client_assertion_type": {"urn:ietf:params:oauth:client-assertion-type:jwt-bearer"},
		"client_assertion":      {token},
	}
	req, err := http.NewRequestWithContext(ctx, "POST", msAuth.ReqUrl(), strings.NewReader(v.Encode()))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	req.Body.Close()
	if err != nil {
		return
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		err = errs.ErrServerError().WithDetail(fiber.Map{
			"microsoftResponseBody": body, "microsoftResponseStatus": res.StatusCode,
		})
		return
	}
	var t map[string]any
	err = json.Unmarshal(body, &t)
	if err != nil {
		err = errs.ErrServerError().WithDetail(err)
		return
	}
	idToken, ok := t["id_token"].(string)
	if !ok {
		err = errs.ErrServerError().WithDetail("microsoft server does not return id token")
		return
	}
	idTokenSplit := strings.Split(idToken, ".")
	if len(idTokenSplit) != 3 {
		err = errs.ErrServerError().WithDetail("microsoft server returned malformed id token").WithDetail(idToken)
		return
	}
	b, err := base64.RawStdEncoding.DecodeString(idTokenSplit[1])
	if err != nil {
		err = errs.ErrBadParameter().WithMessage("failed to b64decode microsoft returned token").WithDetail(idToken)
		return
	}
	var idTokenContent oauthIdTokenPayload
	err = json.Unmarshal(b, &idTokenContent)
	if err != nil {
		err = errs.ErrBadParameter().WithMessage("failed to json unmarshal microsoft returned token").WithDetail(err)
		return
	}
	if idTokenContent.Email == nil {
		err = errs.ErrBadParameter().WithMessage("microsoft returned token does not have email").WithDetail(idTokenContent)
		return
	}
	if idTokenContent.Subject == nil {
		err = errs.ErrBadParameter().WithMessage("microsoft returned token does not have subject").WithDetail(idTokenContent)
		return
	}
	if idTokenContent.Email == nil {
		err = errs.ErrBadParameter().WithMessage("microsoft returned token does not have email").WithDetail(idTokenContent)
		return
	}
	o.Email = idTokenContent.Email
	o.Subject = idTokenContent.Subject
	o.Name = idTokenContent.Name
	o.ExchangeTs = &now
	err = o.UpdateExchange(ctx)
	return
}
