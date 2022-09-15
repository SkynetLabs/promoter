package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/SkynetLabs/promoter/api"
	"github.com/SkynetLabs/promoter/database"
	"github.com/sirupsen/logrus"
	"gitlab.com/NebulousLabs/errors"
)

type (
	// config contains the configuration for the service which is parsed
	// from the environment vars.
	config struct {
		LogLevel     logrus.Level
		Port         int
		DBURI        string
		DBUser       string
		DBPassword   string
		ServerDomain string
		AccountsHost string
		AccountsPort string
	}
)

const (
	// envAPIShutdownTimeout is the timeout for gracefully shutting down the
	// API before killing it.
	envAPIShutdownTimeout = 20 * time.Second

	// envAccountsHost is the environment variable for the host where we can
	// find the accounts service.
	envAccountsHost = "ACCOUNTS_HOST"

	// envAccountsPort is the environment variable for the host where we can
	// find the accounts service.
	envAccountsPort = "ACCOUNTS_PORT"

	// envMongoDBURI is the environment variable for the mongodb URI.
	envMongoDBURI = "MONGODB_URI"

	// envMongoDBUser is the environment variable for the mongodb user.
	envMongoDBUser = "MONGODB_USER"

	// envMongoDBPassword is the environment variable for the mongodb password.
	envMongoDBPassword = "MONGODB_PASSWORD"

	// envLogLevel is the environment variable for the log level used by
	// this service.
	envLogLevel = "PROMOTER_LOG_LEVEL"

	// envServerDomain is the environment variable for setting the domain of
	// the server within the cluster.
	envServerDomain = "SERVER_DOMAIN"
)

// parseConfig parses a Config struct from the environment.
func parseConfig() (*config, error) {
	// Create config with default vars.
	cfg := &config{
		LogLevel:     logrus.InfoLevel,
		AccountsHost: "10.10.10.70",
		AccountsPort: "3000",
	}

	// Parse custom vars from environment.
	var ok bool
	var err error

	logLevelStr, ok := os.LookupEnv(envLogLevel)
	if ok {
		cfg.LogLevel, err = logrus.ParseLevel(logLevelStr)
		if err != nil {
			return nil, errors.AddContext(err, "failed to parse log level")
		}
	}
	cfg.DBURI, ok = os.LookupEnv(envMongoDBURI)
	if !ok {
		return nil, fmt.Errorf("%s wasn't specified", envMongoDBURI)
	}
	cfg.DBUser, ok = os.LookupEnv(envMongoDBUser)
	if !ok {
		return nil, fmt.Errorf("%s wasn't specified", envMongoDBUser)
	}
	cfg.DBPassword, ok = os.LookupEnv(envMongoDBPassword)
	if !ok {
		return nil, fmt.Errorf("%s wasn't specified", envMongoDBPassword)
	}
	cfg.ServerDomain, ok = os.LookupEnv(envServerDomain)
	if !ok {
		return nil, fmt.Errorf("%s wasn't specified", envServerDomain)
	}
	cfg.AccountsHost, ok = os.LookupEnv(envAccountsHost)
	if !ok {
		return nil, fmt.Errorf("%s wasn't specified", envAccountsHost)
	}
	cfg.AccountsPort, ok = os.LookupEnv(envAccountsPort)
	if !ok {
		return nil, fmt.Errorf("%s wasn't specified", envAccountsPort)
	}
	return cfg, nil
}

func main() {
	logger := logrus.New()

	// Create application context.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Parse env vars.
	cfg, err := parseConfig()
	if err != nil {
		logger.WithError(err).Fatal("Failed to parse Config")
	}

	// Create the loggers for the submodules.
	logger.SetLevel(cfg.LogLevel)
	apiLogger := logger.WithField("modules", "api")
	dbLogger := logger.WithField("modules", "db")

	// Create the promoter that talks to skyd and the database.
	db, err := database.New(ctx, dbLogger, cfg.DBURI, cfg.DBUser, cfg.DBPassword, cfg.ServerDomain, database.DBName)
	if err != nil {
		logger.WithError(err).Fatal("Failed to connect to database")
	}

	// Create API.
	a, err := api.New(apiLogger, db, cfg.Port)
	if err != nil {
		logger.WithError(err).Fatal("Failed to init API")
	}

	// Register handler for shutdown.
	var wg sync.WaitGroup
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-sigChan

		// Log that we are shutting down.
		logger.Info("Caught stop signal. Shutting down...")

		// Shut down API with sane timeout.
		shutdownCtx, cancel := context.WithTimeout(ctx, envAPIShutdownTimeout)
		defer cancel()
		if err := a.Shutdown(shutdownCtx); err != nil {
			logger.WithError(err).Error("Failed to shut down api")
		}
	}()

	// Start serving API.
	err = a.ListenAndServe()
	if err != nil && !errors.Contains(err, http.ErrServerClosed) {
		logger.WithError(err).Error("ListenAndServe returned an error")
	}

	// Wait for the goroutine to finish before continuing with the remaining
	// shutdown procedures.
	wg.Wait()

	// Close database.
	if err = db.Close(); err != nil {
		logger.WithError(err).Fatal("Failed to close database gracefully")
	}
}
