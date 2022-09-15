package api

import (
	"encoding/json"
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
	var payment PaymentPOST
	err := json.NewDecoder(req.Body).Decode(&payment)
	if err != nil {
		api.WriteError(w, errors.AddContext(err, "failed to parse body"), http.StatusBadRequest)
		return
	}
	if err = payment.Validate(); err != nil {
		api.WriteError(w, err, http.StatusBadRequest)
		return
	}
	err = api.staticDB.CreditUser(req.Context(), payment.Sub, payment.Credits, payment.TxnID)
	if err != nil {
		api.WriteError(w, err, http.StatusInternalServerError)
		return
	}
	api.WriteSuccess(w)
}
