package router

import (
	"fmt"
	"slices"

	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	pluginruntime "github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
)

// playgroundProtocolAliases exposes host protocol operations to signed-in
// console users and records what they generate.
var playgroundProtocolAliases = map[string]struct {
	path string
	kind string
}{
	"openai_image.generate": {path: "/pg/images/generations", kind: model.UserCreationKindImage},
	"openai_video.create":   {path: "/pg/videos", kind: model.UserCreationKindVideo},
}

func SetTaskPluginProtocolRouter(router *gin.Engine) {
	for _, protocol := range pluginruntime.HostProtocols() {
		for _, operation := range protocol.Operations {
			key := protocol.Name + "." + operation.Name
			alias, hasAlias := playgroundProtocolAliases[key]
			for _, method := range operation.Methods {
				handlers, err := taskPluginProtocolHandlers(protocol.Name, operation.Name, middleware.TokenAuth())
				if err != nil {
					panic(err)
				}
				router.Handle(method, operation.Path, handlers...)
				if !hasAlias {
					continue
				}
				handlers, err = taskPluginProtocolHandlers(protocol.Name, operation.Name,
					middleware.UserAuth(), middleware.PlaygroundRelayAuth(operation.Path), controller.RecordUserCreation(alias.kind))
				if err != nil {
					panic(err)
				}
				router.Handle(method, alias.path, handlers...)
			}
		}
	}
}

func taskPluginProtocolHandlers(protocol, operation string, auth ...gin.HandlerFunc) ([]gin.HandlerFunc, error) {
	switch protocol + "." + operation {
	case "openai_responses.create":
		return slices.Concat(
			[]gin.HandlerFunc{middleware.RouteTag("relay"), middleware.SystemPerformanceCheck()},
			auth,
			[]gin.HandlerFunc{
				middleware.ModelRequestRateLimit(), middleware.RouteTaskPluginEndpoint(), middleware.PrepareTaskPluginEndpoint(), middleware.Distribute(),
				func(c *gin.Context) {
					controller.RelayTaskPluginEndpoint(c, func(c *gin.Context) { controller.Relay(c, types.RelayFormatOpenAIResponses) })
				},
			},
		), nil
	case "openai_image.generate", "openai_image.edit":
		return slices.Concat(
			[]gin.HandlerFunc{middleware.RouteTag("relay"), middleware.SystemPerformanceCheck()},
			auth,
			[]gin.HandlerFunc{
				middleware.ModelRequestRateLimit(), middleware.RouteTaskPluginEndpoint(), middleware.PrepareTaskPluginEndpoint(), middleware.Distribute(),
				func(c *gin.Context) {
					controller.RelayTaskPluginEndpoint(c, func(c *gin.Context) { controller.Relay(c, types.RelayFormatOpenAIImage) })
				},
			},
		), nil
	case "openai_video.create":
		return slices.Concat(
			[]gin.HandlerFunc{middleware.RouteTag("relay")},
			auth,
			[]gin.HandlerFunc{
				middleware.SystemPerformanceCheck(),
				middleware.RouteTaskPluginEndpoint(), middleware.TaskPluginEndpointOnly(middleware.ModelRequestRateLimit()), middleware.PrepareTaskPluginEndpoint(), middleware.Distribute(),
				func(c *gin.Context) { controller.RelayTaskPluginEndpoint(c, controller.RelayTask) },
			},
		), nil
	case "openai_responses.retrieve":
		return slices.Concat([]gin.HandlerFunc{middleware.RouteTag("relay")}, auth, []gin.HandlerFunc{controller.RetrieveTaskPluginResponse}), nil
	case "openai_video.retrieve":
		return slices.Concat([]gin.HandlerFunc{middleware.RouteTag("relay")}, auth, []gin.HandlerFunc{middleware.Distribute(), controller.RelayTaskFetch}), nil
	case "openai_video.content":
		return slices.Concat([]gin.HandlerFunc{middleware.RouteTag("relay")}, auth, []gin.HandlerFunc{controller.VideoProxy}), nil
	default:
		return nil, fmt.Errorf("host protocol registry operation %s.%s has no handler", protocol, operation)
	}
}
