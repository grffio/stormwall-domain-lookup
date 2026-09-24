package httpx

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

const (
	contentTypeText           = "text/plain; charset=utf-8"
	contentTypeJSON           = "application/json; charset=utf-8"
	contentTypeOptionsNoSniff = "nosniff"

	headerContentType        = "Content-Type"
	headerContentLength      = "Content-Length"
	headerContentTypeOptions = "X-Content-Type-Options"
)

// WriteText writes a plain-text HTTP response.
func WriteText(w http.ResponseWriter, status int, body string) {
	if status == http.StatusNoContent {
		WriteNoContent(w)
		return
	}

	writePayload(w, status, contentTypeText, []byte(body))
}

// WriteError writes a plain-text error response.
func WriteError(w http.ResponseWriter, status int, message string) {
	if message == "" {
		message = defaultStatusMessage(status)
	}

	WriteText(w, status, message)
}

// WriteJSON writes a JSON HTTP response.
func WriteJSON(w http.ResponseWriter, status int, v any) error {
	if status == http.StatusNoContent {
		WriteNoContent(w)
		return nil
	}

	payload, err := json.Marshal(v)
	if err != nil {
		body := []byte(`{"message":"internal server error"}` + "\n")

		writePayload(
			w,
			http.StatusInternalServerError,
			contentTypeJSON,
			body,
		)

		return fmt.Errorf("marshal JSON response: %w", err)
	}

	payload = append(payload, '\n')
	writePayload(w, status, contentTypeJSON, payload)

	return nil
}

type errorResponse struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}

// WriteErrorJSON writes a JSON error response.
func WriteErrorJSON(w http.ResponseWriter, status int, code, message string) error {
	if message == "" {
		message = defaultStatusMessage(status)
	}

	return WriteJSON(w, status, errorResponse{
		Code:    code,
		Message: message,
	})
}

// WriteNoContent writes an empty HTTP 204 response.
func WriteNoContent(w http.ResponseWriter) {
	h := w.Header()
	writeCommonHeaders(h)
	h.Del(headerContentType)
	h.Del(headerContentLength)

	w.WriteHeader(http.StatusNoContent)
}

func writePayload(w http.ResponseWriter, status int, contentType string, payload []byte) {
	h := w.Header()
	writeCommonHeaders(h)
	h.Set(headerContentType, contentType)
	h.Set(headerContentLength, strconv.Itoa(len(payload)))

	w.WriteHeader(status)
	_, _ = w.Write(payload)
}

func writeCommonHeaders(h http.Header) {
	h.Set(headerContentTypeOptions, contentTypeOptionsNoSniff)
}

func defaultStatusMessage(status int) string {
	if message := http.StatusText(status); message != "" {
		return message
	}

	return "error"
}
