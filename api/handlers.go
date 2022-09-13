package api

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
	"gitlab.com/NebulousLabs/errors"
)

// healthGET returns the status of the service
func (api *API) healthGET(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	ph := api.staticDB.Health()
	api.WriteJSON(w, HealthGET{
		DBAlive: ph.Database == nil,
	})
}

// paymentPOST registers a new payment. The payment is represented by a txn id,
// user's sub, and an amount. The amount is in credits that are to be added to
// the user's balance. The txn id ensures the idempotency of the operation.
func (api *API) paymentPOST(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	// TODO Implement
	api.WriteError(w, errors.New("TODO: IMPLEMENT"), http.StatusInternalServerError)
}
