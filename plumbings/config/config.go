package config

import (
	"github.com/get10xteam/sales-module-backend/plumbings/storage"

	"gitlab.com/intalko/gosuite/logger"
	"gitlab.com/intalko/gosuite/pgdb"
)

type SMTPConfigType struct {
	Host     string `yaml:"Host"`
	Port     int    `yaml:"Port"`
	AuthType string `yaml:"AuthType"`
	Username string `yaml:"Username"`
	Password string `yaml:"Password"`
	From     string `yaml:"From"`
}
type RuntimeConfigType struct {
	ListenAddress          string `yaml:"ListenAddress" env:"PORT"`
	LogHTTP                bool   `yaml:"LogHTTP" env:"LOG_HTTP"`
	LogAllHeaderBody       bool   `yaml:"LogAllHeaderBody" env:"LOG_ALL_HEADER_BODY"`
	MaxBodySizeMB          int    `yaml:"MaxBodySizeMB" env:"MAX_BODY_SIZE"`
	AppName                string `yaml:"AppName" env:"APP_NAME"`
	ExposeErrorDetails     bool   `yaml:"ExposeErrorDetails" env:"EXPOSE_ERROR_DETAILS"`
	SessionDurationMinutes int    `yaml:"SessionDurationMinutes" env:"SESSION_DURATION_MINUTES"`
	ServeFrontEndDir       string `yaml:"ServeFrontEndDir"`
	AllowCors              bool   `yaml:"AllowCors" env:"ALLOW_CORS"`
	ListenHttps            bool   `yaml:"ListenHttps" env:"LISTEN_HTTPS"`
}
type DeploymentURLsType struct {
	UsrResetPassword     string `yaml:"UsrResetPassword"`
	UsrSignUpVerify      string `yaml:"UsrSignUpVerify"`
	HQResetPassword      string `yaml:"HQResetPassword"`
	HQWelcomeSetPassword string `yaml:"HQWelcomeSetPassword"`
}

type AuthType struct {
	AuthorizationExpirationSeconds int                `yaml:"AuthorizationExpirationSeconds"`
	GoogleAuth                     *GoogleAuthType    `yaml:"GoogleAuth"`
	MicrosoftAuth                  *MicrosoftAuthType `yaml:"MicrosoftAuth"`
}

type ConfigType struct {
	DB             pgdb.Config               `yaml:"DB"`
	Log            logger.Config             `yaml:"Log"`
	SMTP           SMTPConfigType            `yaml:"SMTP"`
	Auth           AuthType                  `yaml:"Auth"`
	Runtime        RuntimeConfigType         `yaml:"Runtime"`
	Storage        storage.StorageConfigType `yaml:"Storage"`
	DeploymentURLs DeploymentURLsType        `yaml:"DeploymentURLs"`
	IntObfuscation intObfuscation            `yaml:"IntObfuscation"`
}

var Config ConfigType
