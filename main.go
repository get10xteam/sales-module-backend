package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/get10xteam/sales-module-backend/errs"
	"github.com/get10xteam/sales-module-backend/plumbings/config"
	"github.com/get10xteam/sales-module-backend/plumbings/httpServer"
	"github.com/get10xteam/sales-module-backend/plumbings/storage"
	"github.com/get10xteam/sales-module-backend/plumbings/utils"

	"gitlab.com/intalko/gosuite/logger"
	"gitlab.com/intalko/gosuite/mainhelper"
	"gitlab.com/intalko/gosuite/pgdb"

	"github.com/benedictjohannes/gracefulcontext"
)

func main() {
	err := mainhelper.GetConfig(&config.Config)
	if err != nil {
		log.Fatalln("Failed to read configuration")
	}
	cmd, hasCmd := mainhelper.SubcommandEntryPoint(mainhelper.FuncMap{
		"migrate": func() {
			pgdb.MigrateEntryPoint(config.Config.DB)
		},
		"passwordHash": func() {
			flag.Parse()
			args := flag.Args()
			if len(args) != 2 {
				fmt.Println("Usage: ./get10x_project pass <password>")
				return
			}
			h, _ := utils.CheckAndHashPassword(args[1])
			fmt.Println("Password hash: " + h)
		},
	})
	if hasCmd {
		cmd()
		return
	}
	bgCtx := context.Background()
	loggerCtx := gracefulcontext.NewGracefulContext(bgCtx).Make()
	dbCtx := gracefulcontext.NewGracefulContext(loggerCtx).WithCleanupTimeout(2 * time.Second).
		WithCleanupFunc(func() error {
			pgdb.DbPool.Close()
			logger.Info().Msg("DB connection closed")
			return nil
		}).
		Make()
	loggerCtx.SubscribeCancellation(dbCtx)
	httpCtx := gracefulcontext.NewGracefulContext(dbCtx).WithCleanupTimeout(2 * time.Second).
		WithCleanupFunc(httpServer.Shutdown).
		Make()
	dbCtx.SubscribeCancellation(httpCtx)
	errs.SetErrorDetailMarshallable(config.Config.Runtime.ExposeErrorDetails)
	err = logger.InitLogger(loggerCtx, config.Config.Log)
	if err != nil {
		log.Fatalln("Failed to initialize logger")
	}
	err = pgdb.ConnectPgxPool(dbCtx, config.Config.DB)

	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to Postgres")
	} else {
		logger.Info().Msg("DB connection established")
	}
	err = storage.InitializeStorage(&config.Config.Storage)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize storage")
	} else {
		logger.Info().Msg("Storage initialized")
	}
	httpServer.Init()
	interruptChan := make(chan os.Signal, 1)
	exitChan := make(chan struct{}, 1)
	signal.Notify(interruptChan, os.Interrupt)
	go func() {
		<-interruptChan
		logger.Info().Msg("Received interrupt signal")
		httpCtx.Cancel()
	}()
	go func() {
		<-loggerCtx.Done()
		exitChan <- struct{}{}
	}()
	<-exitChan
	os.Exit(0)
}
