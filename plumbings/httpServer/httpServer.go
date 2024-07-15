package httpServer

import (
	"errors"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/get10xteam/sales-module-backend/errs"
	"github.com/get10xteam/sales-module-backend/plumbings/config"

	oLogger "gitlab.com/intalko/gosuite/logger"

	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/utils"
	"github.com/rs/zerolog"
)

var logger = oLogger.Logger
var server *fiber.App
var resTimeout = time.Duration(10 * time.Second)

const maxLoggedBodySize = 30 * 1024 // only log 30KB and below

func Init() (err error) {
	rt := config.Config.Runtime
	if _, err := strconv.Atoi(rt.ListenAddress); err == nil {
		rt.ListenAddress = ":" + rt.ListenAddress
	}
	logger = oLogger.Logger.With().Str("scope", "http").Logger()
	if rt.MaxBodySizeMB <= 0 {
		rt.MaxBodySizeMB = 20
	}
	server = fiber.New(fiber.Config{
		JSONDecoder:  json.Unmarshal,
		JSONEncoder:  json.Marshal,
		BodyLimit:    rt.MaxBodySizeMB * 1024 * 1024,
		AppName:      rt.AppName,
		UnescapePath: true,
		ErrorHandler: errHandler,
		ReadTimeout:  resTimeout * 10,
		WriteTimeout: resTimeout,
		Immutable:    true,
	})
	if rt.LogHTTP {
		server.Use(reqLogger)
	}
	if rt.AllowCors {
		server.Use(cors.New(cors.Config{
			AllowCredentials: true,
		}))
	}
	panicRecoverConfig := recover.Config{
		EnableStackTrace: true,
		StackTraceHandler: func(c *fiber.Ctx, e interface{}) {
			buf := make([]byte, 2048)
			buf = buf[:runtime.Stack(buf, false)]
			logger.Error().Interface("panicErr", e).
				Interface("headers", c.GetReqHeaders()).
				Str("url", c.OriginalURL()).
				Strs("panicStack",
					strings.Split(strings.TrimSpace(strings.ReplaceAll(string(buf), "\t", "")), "\n"),
				).
				Msg("PANIC")
		},
	}
	server.Use(func(c *fiber.Ctx) error {
		c.Set("X-Frame-Options", "SAMEORIGIN")
		return c.Next()
	})
	server.Use(recover.New(panicRecoverConfig))
	server.Use(timeOutNotifierHandler)
	registerRoutes(rt)
	logger.Info().Msg("HTTP Routes registered")
	if rt.ListenHttps {
		const httpsCertPath = "https.cert.pem"
		httpsCert, err := os.Stat(httpsCertPath)
		if err != nil || httpsCert.IsDir() {
			goto LISTENHTTP
		}
		const httpsKeyPath = "https.key.pem"
		httpsKey, err := os.Stat(httpsKeyPath)
		if err != nil || httpsKey.IsDir() {
			goto LISTENHTTP
		}
		go func() {
			logger.Info().Msg("started HTTPS listening on " + rt.ListenAddress)
			err := server.ListenTLS(rt.ListenAddress, httpsCertPath, httpsKeyPath)
			if err != nil {
				logger.Error().Err(err).Msg("failed to start HTTPS server")
			}
		}()
		return nil
	}
LISTENHTTP:
	go func() {
		logger.Info().Msg("started listening on " + rt.ListenAddress)
		err := server.Listen(rt.ListenAddress)
		if err != nil {
			logger.Error().Err(err).Msg("failed to start HTTP server")
		}
	}()
	return nil
}

func timeOutNotifierHandler(c *fiber.Ctx) error {
	url := utils.CopyString(c.OriginalURL())
	nc := make(chan struct{})
	go timeOutNotifier(nc, url)
	err := c.Next()
	close(nc)
	return err
}
func timeOutNotifier(c <-chan struct{}, url string) {
	select {
	case <-c:
		return
	case <-time.NewTimer(resTimeout).C:
		logger.Warn().Str("url", url).Msg("long request timeout")
	}
}

func reqLogger(ctx *fiber.Ctx) error {
	if ctx.Method() == fiber.MethodOptions {
		return ctx.Next()
	}
	start := time.Now()
	url := ctx.OriginalURL()
	method := string(ctx.Method())
	ip := ctx.IPs()
	err := ctx.Next()
	end := time.Now()
	status := ctx.Response().StatusCode()
	duration := end.Sub(start)
	reqSize := ctx.Request().Header.ContentLength()
	var logChain *zerolog.Event
	if err != nil {
		if appErr, ok := err.(*errs.Error); ok {
			if appErr.ShouldLog() {
				logChain = logger.Warn().Object("error", appErr)
			} else {
				logChain = logger.Info().Err(appErr)
			}
		} else {
			logChain = logger.Warn().Err(err)
		}
	} else {
		logChain = logger.Info()
	}
	if config.Config.Runtime.LogAllHeaderBody {
		reqBody := ctx.Body()
		if len(reqBody) <= maxLoggedBodySize && len(reqBody) > 0 {
			if json.Valid(reqBody) {
				logChain.RawJSON("reqBody", reqBody)
			} else {
				logChain.Str("reqBody", string(reqBody))
			}
		}
		resBody := ctx.Response().Body()
		if len(resBody) <= maxLoggedBodySize {
			if ctx.GetRespHeader("Content-Type") == "application/json" {
				logChain.RawJSON("resBody", resBody)
			} else {
				logChain.Str("resBody", string(resBody))
			}
		}
		logChain.Interface("reqHeaders", ctx.GetReqHeaders()).Interface("reqQuery", ctx.Queries())
	}
	logChain.
		Str("url", url).
		Str("method", method).
		Time("start", start).
		Time("end", end).
		Dur("duration", duration).
		Int("reqBodySize", reqSize).
		Strs("ip", ip).
		Int("status", status)
	logChain.Msg("HTTPreq")
	return err
}

func errHandler(c *fiber.Ctx, err error) error {
	if errors.Is(err, fiber.ErrNotFound) {
		return c.JSON(errs.ErrRouteNotFound())
	}
	if appErr, ok := err.(*errs.Error); ok {
		return c.JSON(appErr)
	}
	return c.JSON(errs.ErrServerError().WithDetail(err))
}

func Shutdown() error {
	logger.Info().Msg("httpServer shutting down")
	return server.Shutdown()
}
