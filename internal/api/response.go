// Package api holds wire-contract types shared between API servers and their
// clients. It is a leaf package: it imports no other internal package so both
// servers (e.g. worker) and clients (e.g. manager) can depend on it freely.
package api

// ErrResponse is the JSON error body returned by API servers.
type ErrResponse struct {
	HTTPStatusCode int
	Message        string
}
