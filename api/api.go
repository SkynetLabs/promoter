package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"gitlab.com/NebulousLabs/errors"
	"go.mongodb.org/mongo-driver/mongo"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/SkynetLabs/promoter/database"
	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
)

const (
	// DBTxnRetryCount specifies the number of times we should retry an API
	// call in case we run into transaction errors.
	DBTxnRetryCount = 5
)

type (
	// API manages the http API and all of its routes.
	API struct {
		staticDB       *database.DB
		staticListener net.Listener
		staticLogger   *logrus.Entry
		staticRouter   *httprouter.Router
		staticServer   *http.Server
	}

	// Error is the error type returned by the API in case the status code
	// is not a 2xx code.
	Error struct {
		Message string `json:"message"`
	}

	// errorWrap is a helper type for converting an `error` struct to JSON.
	errorWrap struct {
		Message string `json:"message"`
	}
)

// Error implements the error interface for the Error type. It returns only the
// Message field.
func (err Error) Error() string {
	return err.Message
}

// New creates a new API with the given logger and database.
func New(log *logrus.Entry, db *database.DB, port int) (*API, error) {
	l, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return nil, err
	}
	router := httprouter.New()
	router.RedirectTrailingSlash = true
	api := &API{
		staticDB:       db,
		staticListener: l,
		staticLogger:   log,
		staticRouter:   router,
		staticServer: &http.Server{
			Handler: router,

			// Set low timeouts since we expect to only talk to this
			// service on the same machine.
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       10 * time.Second,
		},
	}
	api.buildHTTPRoutes()
	return api, nil
}

// Address returns the address the API is listening on.
func (api *API) Address() string {
	return api.staticListener.Addr().String()
}

// ListenAndServe starts the API. To unblock this call Shutdown.
func (api *API) ListenAndServe() error {
	return api.staticServer.Serve(api.staticListener)
}

// Shutdown gracefully shuts down the API.
func (api *API) Shutdown(ctx context.Context) error {
	return api.staticServer.Shutdown(ctx)
}

// WithDBSession injects a session context into the request context of the
// handler. In case of a MongoDB WriteConflict error, the call is retried up to
// DBTxnRetryCount times or until the request context expires.
func (api *API) WithDBSession(h httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		numRetriesLeft := DBTxnRetryCount
		var body []byte
		var err error
		if req.Body != nil {
			// Read the request's body and replace its Body io.ReadCloser with a
			// new one based off the read data.
			body, err = io.ReadAll(req.Body)
			if err != nil {
				api.WriteError(w, errors.AddContext(err, "failed to read body"), http.StatusBadRequest)
				return
			}
			_ = req.Body.Close()
		}

		// handleFn wraps a full execution of the handler, combined with a retry
		// detection and counting. It also takes care of creating and cancelling
		// Mongo sessions and transactions.
		handleFn := func() (retry bool) {
			req.Body = io.NopCloser(bytes.NewReader(body))
			// Create a new db session
			sess, err := api.staticDB.NewSession()
			if err != nil {
				api.WriteError(w, errors.AddContext(err, "failed to start a new mongo session"), http.StatusInternalServerError)
				return false
			}
			// Close session after the handler is done.
			defer sess.EndSession(req.Context())
			// Create session context.
			sctx := mongo.NewSessionContext(req.Context(), sess)
			// Get a special response writer which provide the necessary tools
			// to retry requests on error.
			mw, err := NewMongoWriter(w, sctx, api.staticLogger)
			if err != nil {
				api.WriteError(w, errors.AddContext(err, "failed to start a new transaction"), http.StatusInternalServerError)
				return false
			}
			// Create a new request with our session context.
			req = req.WithContext(sctx)
			// Forward the new response writer and request to the handler.
			h(&mw, req, ps)

			// If the call succeeded then we're done because both the status and
			// the response content are already written to the response writer.
			if mw.ErrorStatus() == 0 {
				return false
			}
			// If the call failed with a WriteConflict error and we still have
			// retries left, we'll retry it. Otherwise, we'll write the error to
			// the response writer and finish the call.
			if mw.FailedWithWriteConflict() && numRetriesLeft > 0 {
				select {
				case <-req.Context().Done():
					// If the request context has expired we won't retry anymore.
				default:
					api.staticLogger.Tracef("Retrying call because of WriteConflict (%d out of %d). Request: %+v", numRetriesLeft, DBTxnRetryCount, req)
					numRetriesLeft--
					return true
				}
			}
			// If the call failed with a non-WriteConflict error or we ran out
			// of retries, we write the error and status to the response writer
			// and finish the call.
			w.WriteHeader(mw.ErrorStatus())
			_, err = w.Write(mw.ErrorBuffer())
			if err != nil {
				api.staticLogger.Warnf("Failed to write to response writer: %+v", err)
			}
			return false
		}

		// Keep retrying the handleFn until it returns a false, indicating that
		// no more retries are needed or possible.
		for handleFn() {
		}
	}
}

// WriteError an error to the API caller.
func (api *API) WriteError(w http.ResponseWriter, err error, code int) {
	api.staticLogger.WithError(err).WithField("statuscode", code).Debug("WriteError")

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	encodingErr := json.NewEncoder(w).Encode(errorWrap{Message: err.Error()})
	if encodingErr != nil {
		api.staticLogger.WithError(encodingErr).Error("Failed to encode error response object")
	}
}

// WriteJSON writes the object to the ResponseWriter. If the encoding fails, an
// error is written instead. The Content-Type of the response header is set
// accordingly.
func (api *API) WriteJSON(w http.ResponseWriter, obj interface{}) {
	api.staticLogger.Debug("WriteJSON", obj)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(obj)
	if err != nil {
		api.staticLogger.WithError(err).Error("Failed to encode response object")
	}
}
