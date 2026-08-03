package handler

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const compositeRouteResolverContextKey = "composite_route_resolver"

// AttachCompositeRouteResolver makes the request-scoped resolver available to
// handlers that must read a payload after the HTTP middleware phase, notably
// the Responses WebSocket first frame.
func AttachCompositeRouteResolver(c *gin.Context, resolver *service.CompositeRouteResolver) {
	if c == nil || resolver == nil {
		return
	}
	c.Set(compositeRouteResolverContextKey, resolver)
}

func resolveCompositeTargetPlatform(c *gin.Context, apiKey *service.APIKey, model, endpoint string) (service.CompositeRouteDecision, bool, error) {
	if c == nil || c.Request == nil || apiKey == nil || apiKey.Group == nil || apiKey.Group.Platform != service.PlatformComposite {
		return service.CompositeRouteDecision{}, true, nil
	}
	if value, ok := c.Get(compositeRouteResolverContextKey); ok {
		if resolver, resolverOK := value.(*service.CompositeRouteResolver); resolverOK && resolver != nil {
			decision, err := resolver.Resolve(c.Request.Context(), apiKey.Group.ID, model, endpoint)
			if err != nil {
				return decision, false, err
			}
			if decision.Matched {
				c.Request = c.Request.WithContext(service.WithCompositeRouteDecision(c.Request.Context(), decision))
				return decision, true, nil
			}
			return decision, false, nil
		}
	}
	if platform, ok := service.DetectModelPlatform(model); ok {
		decision := service.CompositeRouteDecision{
			Matched:        true,
			Source:         service.CompositeRouteSourceDetector,
			GroupID:        apiKey.Group.ID,
			PublicModel:    model,
			TargetPlatform: platform,
			UpstreamModel:  model,
			Endpoint:       endpoint,
		}
		c.Request = c.Request.WithContext(service.WithCompositeRouteDecision(c.Request.Context(), decision))
		return decision, true, nil
	}
	return service.CompositeRouteDecision{}, false, nil
}

func ensureCompositeTargetPlatform(c *gin.Context, apiKey *service.APIKey, model string) {
	if c == nil || c.Request == nil || apiKey == nil || apiKey.Group == nil || apiKey.Group.Platform != service.PlatformComposite {
		return
	}
	if _, ok := service.ResolvedTargetPlatformFromContext(c.Request.Context()); ok {
		return
	}
	_, _, _ = resolveCompositeTargetPlatform(c, apiKey, model, service.CompositeRouteEndpointAny)
}

func compositeTargetPlatformAllowed(c *gin.Context, apiKey *service.APIKey, model string, allowed ...string) bool {
	if c == nil || c.Request == nil || apiKey == nil || apiKey.Group == nil || apiKey.Group.Platform != service.PlatformComposite {
		return true
	}
	ensureCompositeTargetPlatform(c, apiKey, model)
	platform, ok := service.ResolvedTargetPlatformFromContext(c.Request.Context())
	if !ok {
		return false
	}
	for _, allowedPlatform := range allowed {
		if platform == allowedPlatform {
			return true
		}
	}
	return false
}

func compositeTargetPlatformResolved(c *gin.Context, apiKey *service.APIKey, model string) bool {
	if c == nil || c.Request == nil || apiKey == nil || apiKey.Group == nil || apiKey.Group.Platform != service.PlatformComposite {
		return true
	}
	ensureCompositeTargetPlatform(c, apiKey, model)
	_, ok := service.ResolvedTargetPlatformFromContext(c.Request.Context())
	return ok
}

func effectiveAPIKeyPlatform(c *gin.Context, apiKey *service.APIKey) string {
	if c != nil && c.Request != nil {
		if platform, ok := service.ResolvedTargetPlatformFromContext(c.Request.Context()); ok {
			return platform
		}
	}
	if apiKey == nil || apiKey.Group == nil {
		return ""
	}
	return apiKey.Group.Platform
}

func openAIReasoningEffortPolicyForRequest(c *gin.Context, apiKey *service.APIKey) (string, []service.ReasoningEffortMapping, bool) {
	if apiKey == nil || apiKey.Group == nil {
		return "", nil, false
	}
	if apiKey.Group.Platform != service.PlatformOpenAI && apiKey.Group.Platform != service.PlatformComposite {
		return "", nil, false
	}
	if effectiveAPIKeyPlatform(c, apiKey) != service.PlatformOpenAI {
		return "", nil, false
	}
	return apiKey.Group.MaxReasoningEffort, apiKey.Group.ReasoningEffortMappings, true
}

func applyOpenAIReasoningEffortPolicyForRequest(c *gin.Context, apiKey *service.APIKey, body []byte) ([]byte, bool) {
	maxEffort, mappings, ok := openAIReasoningEffortPolicyForRequest(c, apiKey)
	if !ok {
		return body, false
	}
	return service.ApplyOpenAIReasoningEffortPolicy(body, maxEffort, mappings)
}

func bindOpenAIReasoningEffortPolicyForMessagesRequest(c *gin.Context, apiKey *service.APIKey, body []byte) {
	if c == nil || c.Request == nil {
		return
	}
	// The Messages bridge synthesizes a default OpenAI effort when
	// output_config.effort is omitted. Bind the group policy only for an
	// explicit client value so the ceiling does not alter that default.
	effort := gjson.GetBytes(body, "output_config.effort")
	if !effort.Exists() || effort.Type != gjson.String || strings.TrimSpace(effort.String()) == "" {
		return
	}
	maxEffort, mappings, ok := openAIReasoningEffortPolicyForRequest(c, apiKey)
	if !ok {
		return
	}
	c.Request = c.Request.WithContext(service.WithOpenAIReasoningEffortPolicy(c.Request.Context(), maxEffort, mappings))
}
