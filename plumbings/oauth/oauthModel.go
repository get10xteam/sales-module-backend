package oauth

import (
	"context"
	"time"

	"github.com/get10xteam/sales-module-backend/errs"
	"github.com/get10xteam/sales-module-backend/plumbings/config"

	"gitlab.com/intalko/gosuite/pgdb"

	"github.com/goccy/go-json"
	"gitlab.com/benedictjohannes/b64uuid"
)

type oauthProviderEnum int

const (
	GoogleProvider    oauthProviderEnum = 1
	MicrosoftProvider oauthProviderEnum = 2
)

func (p oauthProviderEnum) String() string {
	switch p {
	case GoogleProvider:
		return "google"
	case MicrosoftProvider:
		return "microsoft"
	default:
		return ""
	}
}
func (p *oauthProviderEnum) MarshalJSON() ([]byte, error) {
	s := p.String()
	if len(s) == 0 {
		return json.Marshal(nil)
	}
	return json.Marshal(s)
}
func (p *oauthProviderEnum) UnmarshalJSON(b []byte) (err error) {
	var s string
	err = json.Unmarshal(b, &s)
	if err != nil {
		return
	}
	switch s {
	case "google":
		*p = GoogleProvider
		return
	case "microsoft":
		*p = MicrosoftProvider
		return
	default:
		return errs.ErrBadParameter().WithMessage("unsupported provider enum value")
	}
}

type oauthIdTokenPayload struct {
	Subject *string `db:"subject" json:"sub"`
	Name    *string `json:"name,omitempty"`
	Email   *string `db:"email" json:"email,omitempty"`
}

type OauthAuthorization struct {
	Provider       oauthProviderEnum `db:"provider" json:"provider,omitempty"`
	State          b64uuid.B64Uuid   `db:"id" json:"state,omitempty"`
	Code           string            `json:"code,omitempty"`
	Subject        *string           `db:"subject" json:"subject,omitempty"`
	Email          *string           `db:"email" json:"email,omitempty"`
	Name           *string           `db:"name" json:"name,omitempty"`
	DestinationURL *string           `db:"destination_url" json:"destinationUrl,omitempty"`
	Expiry         time.Time         `db:"expiry" json:"expiry,omitempty"`
	CreateTs       time.Time         `db:"create_ts" json:"-"`
	ExchangeTs     *time.Time        `db:"exchange_ts" json:"-"`
}

func OauthAuthorizationById(ctx context.Context, provider oauthProviderEnum, stateId b64uuid.B64Uuid) (o *OauthAuthorization, err error) {
	o = &OauthAuthorization{Provider: provider, State: stateId}
	err = o.LoadColumns(ctx)
	return
}
func (o *OauthAuthorization) LoadColumns(ctx context.Context) (err error) {
	const sql = `select email, destination_url, expiry, create_ts, exchange_ts from oauth_authorizations
 where provider = $1 and state = $2`
	err = pgdb.QueryRow(ctx, sql, o.Provider, o.State).Scan(
		&o.Email, &o.DestinationURL, &o.Expiry, &o.CreateTs, &o.Expiry,
	)
	return
}

// Provider is mandatory, DestinationURL is optional. Other fields ignored.
func (o *OauthAuthorization) CreateSaveToDB(ctx context.Context) (err error) {
	if o.Provider == 0 {
		return errs.ErrBadParameter()
	}
	o.State = b64uuid.NewRandom()
	const sql = `insert into oauth_authorizations 
 (provider, state, destination_url, expiry, create_ts)
 values ($1, $2, $3, $4, $5)`
	now := time.Now()
	expirationSeconds := config.Config.Auth.AuthorizationExpirationSeconds
	if expirationSeconds == 0 {
		expirationSeconds = 180
	}
	expiry := now.Add(time.Duration(expirationSeconds) * time.Second)
	_, err = pgdb.Exec(ctx, sql,
		o.Provider, o.State, o.DestinationURL, expiry, now,
	)
	if err != nil {
		return
	}
	o.Expiry = expiry
	o.CreateTs = now
	return
}

// Updates email, subject, name, exchange_ts with field values
func (o *OauthAuthorization) UpdateExchange(ctx context.Context) (err error) {
	const sql = `update oauth_authorizations set 
 email = $3, subject = $4, name = $5, exchange_ts = $6
 where provider = $1 and state = $2`
	_, err = pgdb.Exec(ctx, sql, o.Provider, o.State,
		o.Email, o.Subject, o.Name, o.ExchangeTs)
	return
}
