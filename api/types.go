package api

import "gitlab.com/NebulousLabs/errors"

// These are the request and response types used by the API.
type (
	// PaymentPOST describes a request which notifies Promoter of an incoming
	// txn that credits the balance of a user with a given sub.
	PaymentPOST struct {
		TxnID   string  `json:"txnID"`
		Sub     string  `json:"sub"`
		Credits float64 `json:"credits"`
	}
)

// Validate ensures the payment information is valid and complete.
func (p *PaymentPOST) Validate() error {
	if p.Credits <= 0 {
		return errors.New("non-positive credits amount")
	}
	if p.Sub == "" {
		return errors.New("missing or empty sub")
	}
	if p.TxnID == "" {
		return errors.New("missing or empty txn ID")
	}
	return nil
}
