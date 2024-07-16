package mailer

import (
	"strings"
	"sync"
	"time"

	"github.com/get10xteam/sales-module-backend/plumbings/config"

	"gitlab.com/intalko/gosuite/logger"

	mail "github.com/xhit/go-simple-mail/v2"
)

var smtpServer *mail.SMTPServer
var smtpServerMu sync.Mutex

func initEmail() *mail.SMTPServer {
	smtpServerMu.Lock()
	defer smtpServerMu.Unlock()
	/* if smtpServer != nil {
		return smtpServer
	} */
	smtpServer := mail.NewSMTPClient()
	smtpServer.Host = config.Config.SMTP.Host
	smtpServer.Port = config.Config.SMTP.Port
	switch smtpServer.Port {
	case 587:
		smtpServer.Encryption = mail.EncryptionSTARTTLS
	case 465:
		smtpServer.Encryption = mail.EncryptionSSLTLS
	}
	switch strings.ToUpper(config.Config.SMTP.AuthType) {
	case "CRAMMD5":
		smtpServer.Authentication = mail.AuthCRAMMD5
	case "LOGIN":
		smtpServer.Authentication = mail.AuthLogin
	case "NONE":
		smtpServer.Authentication = mail.AuthNone
	case "PLAIN":
		smtpServer.Authentication = mail.AuthPlain
	}
	smtpServer.Username = config.Config.SMTP.Username
	smtpServer.Password = config.Config.SMTP.Password
	smtpServer.ConnectTimeout = time.Second * 10
	smtpServer.SendTimeout = time.Second * 15
	smtpServer.KeepAlive = false
	return smtpServer
}

// MailServer() function returns the server which parameters
// are configured in the config
//
// call returnedServer.Connect() to get smtpClient,
// to be use in returnedMail.Send(smtpClient)
func MailServer() *mail.SMTPServer {
	if smtpServer != nil {
		return smtpServer
	}
	smtpServer := initEmail()
	return smtpServer
}

// NewEmail creates a new email with the default FROM email address
//
// Often used returnedValue.subMethods:
// - SetSubject
//
// - SetBody (mail.TextHTML | mail.TextPlain, string)
//
// - AddAlternative (mail.TextHTML | mail.TextPlain, string)
//
// - Attach(*mail.File)
//
// - Send(smtpClient)
func NewEmail() *mail.Email {
	email := mail.NewMSG()
	return email
}

// SendEmail is a convenience function that can be run as a goroutine
func SendEmail(email *mail.Email) (err error) {
	email.SetFrom(config.Config.SMTP.From)
	err = email.GetError()
	if err != nil {
		return
	}
	ms := MailServer()
	smtpClient, err := ms.Connect()
	if err != nil {
		logger.Logger.Warn().Err(err).Str("scope", "email").Msg("sendMail smtpConnect error")
		return
	}
	logger.Logger.Info().Str("scope", "email").Msg("sendMail smtpConnect")
	defer smtpClient.Close()
	err = email.Send(smtpClient)
	if err != nil {
		logger.Logger.Warn().Err(err).Str("scope", "email").Msg("sendMail error")
	} else {
		logger.Logger.Info().Str("scope", "email").Strs("receiver", email.GetRecipients()).Msg("sendMail")
	}
	return
}
