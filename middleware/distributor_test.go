package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"

	"github.com/gin-gonic/gin"
)

func TestGetForcedMultiKeyIndex(t *testing.T) {
	newContext := func() *gin.Context {
		context, _ := gin.CreateTestContext(httptest.NewRecorder())
		return context
	}

	t.Run("missing context key does not force key zero", func(t *testing.T) {
		index, forced := getForcedMultiKeyIndex(newContext())
		if forced || index != 0 {
			t.Fatalf("missing forced index = (%d, %t), want (0, false)", index, forced)
		}
	})

	t.Run("explicit key zero is forced", func(t *testing.T) {
		context := newContext()
		common.SetContextKey(context, constant.ContextKeyChannelMultiKeyForcedIndex, 0)
		index, forced := getForcedMultiKeyIndex(context)
		if !forced || index != 0 {
			t.Fatalf("forced index = (%d, %t), want (0, true)", index, forced)
		}
	})

	t.Run("negative index is not forced", func(t *testing.T) {
		context := newContext()
		common.SetContextKey(context, constant.ContextKeyChannelMultiKeyForcedIndex, -1)
		index, forced := getForcedMultiKeyIndex(context)
		if forced || index != -1 {
			t.Fatalf("forced index = (%d, %t), want (-1, false)", index, forced)
		}
	})
}
