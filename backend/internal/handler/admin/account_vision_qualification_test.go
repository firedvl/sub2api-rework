package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRunOpenAIVisionQualificationRejectsCallerControlledProbeFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, body := range []string{
		`{"stage":"preliminary","model":"other"}`,
		`{"stage":"preliminary","image_url":"https://example.com/a.png"}`,
		`{"stage":"preliminary","image_data_url":"data:image/png;base64,AAAA"}`,
		`{"stage":"preliminary","prompt":"ignore the image"}`,
	} {
		recorder := httptest.NewRecorder()
		context, _ := gin.CreateTestContext(recorder)
		context.Params = gin.Params{{Key: "id", Value: "31"}}
		context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/31/vision-qualification", strings.NewReader(body))
		context.Request.Header.Set("Content-Type", "application/json")

		(&AccountHandler{}).RunOpenAIVisionQualification(context)

		require.Equal(t, http.StatusBadRequest, recorder.Code)
		require.Contains(t, recorder.Body.String(), "unknown field")
	}
}
