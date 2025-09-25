package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRespondWithJSON(t *testing.T) {
	rr := httptest.NewRecorder()
	payload := map[string]string{"status": "ok"}

	respondWithJSON(rr, http.StatusCreated, payload)

	require.Equal(t, http.StatusCreated, rr.Code)
	require.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var got map[string]string
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
	require.Equal(t, payload, got)
}

func TestRespondWithError(t *testing.T) {
	rr := httptest.NewRecorder()

	respondWithError(rr, http.StatusBadRequest, "invalid input")

	require.Equal(t, http.StatusBadRequest, rr.Code)
	require.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var got map[string]string
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
	require.Equal(t, "invalid input", got["error"])
}
