package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateCodexModelsManifestEnvelopeRequiresExactModelsKey(t *testing.T) {
	for _, body := range []string{`{"models":[]}`, `{"models":[{"slug":"custom"}]}`} {
		require.NoError(t, validateCodexModelsManifestEnvelope([]byte(body)))
	}
	for _, body := range []string{`{"Models":[]}`, `{"models":null}`, `{"models":{}}`, `{"models":[}`, `null`, `[]`} {
		require.Error(t, validateCodexModelsManifestEnvelope([]byte(body)), body)
	}
}
