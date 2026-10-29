package middleware

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// PlaygroundRelayAuth lets a signed-in console user call a relay endpoint
// without an API key. It runs after UserAuth, bills the user through a
// keyless token in the user's own group, and rewrites the path to the relay
// endpoint because plugin routing, relay modes, and channel selection are
// keyed by that path.
func PlaygroundRelayAuth(relayPath string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userId := c.GetInt("id")
		userCache, err := model.GetUserCache(userId)
		if err != nil {
			common.SysLog(common.LogText("PlaygroundRelayAuth GetUserCache error for user %d: %v", userId, err))
			abortWithOpenAiMessage(c, http.StatusInternalServerError, common.TranslateMessage(c, i18n.MsgDatabaseError))
			return
		}
		userCache.WriteContext(c)
		common.SetContextKey(c, constant.ContextKeyUsingGroup, userCache.Group)
		token := &model.Token{UserId: userId, Name: "playground-" + userCache.Group, Group: userCache.Group}
		if err = SetupContextForToken(c, token); err != nil {
			abortWithOpenAiMessage(c, http.StatusInternalServerError, err.Error())
			return
		}
		common.SetContextKey(c, constant.ContextKeyPlayground, true)
		c.Request.URL.Path = relayPath
		c.Next()
	}
}
