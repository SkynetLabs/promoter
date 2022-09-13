package test

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/SkynetLabs/promoter/api"
	"github.com/SkynetLabs/promoter/database"
	"github.com/sirupsen/logrus"
	"gitlab.com/NebulousLabs/errors"
)

// newTestDB creates a DB instance for testing.
func newTestDB(domain string) (*database.DB, error) {
	username := "admin"
	// nolint:gosec // Disable gosec since these are only test credentials.
	password := "aO4tV5tC1oU3oQ7u"
	uri := "mongodb://localhost:37017"
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	return database.New(context.Background(), logrus.NewEntry(logger), uri, username, password, domain, domain)
}

// Tester is a pair of an API and a client to talk to that API for testing.
// Multiple testers will always talk to the same underlying database but have
// their APIs listen on different ports.
type Tester struct {
	*api.Client
	staticAPI *api.API
	staticDB  *database.DB

	shutDown    chan struct{}
	shutDownErr error
}

// Close shuts the tester down gracefully.
func (t *Tester) Close() error {
	if err := t.staticAPI.Shutdown(context.Background()); err != nil {
		return err
	}
	<-t.shutDown
	if errors.Contains(t.shutDownErr, http.ErrServerClosed) {
		return nil // Ignore shutdown error
	}
	return t.shutDownErr
}

// newTester creates a new, ready-to-go tester.
func newTester(server string) (*Tester, error) {
	// Create discard logger.
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	db, err := newTestDB(server)
	if err != nil {
		return nil, err
	}

	// Create API.
	a, err := api.New(logrus.NewEntry(logger), db, 0)
	if err != nil {
		return nil, err
	}
	tester := &Tester{
		Client:    api.NewClient(fmt.Sprintf("http://%s", a.Address())),
		staticAPI: a,
		staticDB:  db,
		shutDown:  make(chan struct{}),
	}

	// Start listening.
	go func() {
		tester.shutDownErr = tester.staticAPI.ListenAndServe()
		close(tester.shutDown)
	}()
	return tester, nil
}
