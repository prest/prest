package transactions

import "net/http"

// OpenTransactions contains a list of opentransactions
var OpenTransactions map[string][]Content

// Content contains a open transaction
type Content struct {
	Request *http.Request    `json:"request"`
	Next    http.HandlerFunc `json:"next_func"`
}
