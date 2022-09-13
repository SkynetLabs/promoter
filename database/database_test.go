package database

import (
	"context"
	"io"
	"testing"

	"github.com/sirupsen/logrus"
)

const (
	testUsername = "admin"
	// nolint:gosec // Disable gosec since these are only test credentials.
	testPassword = "aO4tV5tC1oU3oQ7u"
	testURI      = "mongodb://localhost:37017"
)

// newTestDB creates a DB instance for testing
// without the background threads being launched.
func newTestDB(domain, dbName string) (*DB, error) {
	// Create discard logger.
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	p, err := New(context.Background(), logrus.NewEntry(logger), testURI, testUsername, testPassword, domain, dbName)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// TestPromoterHealth is a unit test for the promoter's Health method.
func TestPromoterHealth(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	t.Parallel()

	db, err := newTestDB(t.Name(), t.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	if ph := db.Health(); ph.Database != nil {
		t.Fatal("not healthy", ph)
	}
}
