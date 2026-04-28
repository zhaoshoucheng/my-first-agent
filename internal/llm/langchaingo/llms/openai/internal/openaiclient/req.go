package openaiclient

import (
	"net/http"
)

// getRequestID returns the request ID from the response header.
func getRequestID(r *http.Response) string {
	if r != nil {
		requestID := r.Header.Get("x-request-id")
		if requestID == "" {
			requestID = r.Header.Get("apim-request-id")
		}
		if requestID != "" {
			return requestID
		}
	}
	return ""
}
