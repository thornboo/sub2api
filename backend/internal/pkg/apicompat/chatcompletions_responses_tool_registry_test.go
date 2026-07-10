package apicompat

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponsesToChatCompletionsRequest_RejectsImplicitHostedToolSearch(t *testing.T) {
	_, err := ResponsesToChatCompletionsRequest(&ResponsesRequest{
		Model: "glm-5.2",
		Input: json.RawMessage(`"find tools"`),
		Tools: []ResponsesTool{{Type: "tool_search"}},
	})
	require.Error(t, err)
	var capabilityErr *ChatCompletionsCapabilityError
	require.ErrorAs(t, err, &capabilityErr)
	assert.Equal(t, "hosted_tool_search", capabilityErr.Feature)
}

func TestResponsesToChatCompletionsRequest_LegacyImplicitClientToolSearchRequiresOptIn(t *testing.T) {
	req := &ResponsesRequest{
		Model: "glm-5.2",
		Input: json.RawMessage(`"find tools"`),
		Tools: []ResponsesTool{{Type: "tool_search"}},
	}
	registry, err := BuildResponsesToolRegistry(req)
	require.NoError(t, err)

	out, err := ResponsesToChatCompletionsRequestWithRegistry(req, registry, ChatCompletionsCapabilities{
		SupportsAllowedTools:          true,
		AllowImplicitClientToolSearch: true,
	})
	require.NoError(t, err)
	require.Len(t, out.Tools, 1)
	assert.Equal(t, "tool_search", out.Tools[0].Function.Name)
	assert.True(t, registry.HasClientToolSearch(ChatCompletionsCapabilities{AllowImplicitClientToolSearch: true}))
}

func TestResponsesToolRegistry_DefersTopLevelFunctionUntilLoaded(t *testing.T) {
	initial := &ResponsesRequest{
		Model: "glm-5.2",
		Input: json.RawMessage(`"find the shipping tool"`),
		Tools: []ResponsesTool{
			{Type: "function", Name: "get_shipping_eta", DeferLoading: true, Parameters: json.RawMessage(`{"type":"object"}`)},
			{Type: "tool_search", Execution: "client"},
		},
	}
	initialRegistry, err := BuildResponsesToolRegistry(initial)
	require.NoError(t, err)
	initialChat, err := ResponsesToChatCompletionsRequestWithRegistry(initial, initialRegistry, DefaultChatCompletionsCapabilities())
	require.NoError(t, err)
	require.Len(t, initialChat.Tools, 1)
	assert.Equal(t, "tool_search", initialChat.Tools[0].Function.Name)

	loaded := &ResponsesRequest{
		Model: "glm-5.2",
		Input: json.RawMessage(`[
			{"type":"tool_search_call","execution":"client","call_id":"call_search","arguments":{"goal":"shipping"}},
			{"type":"tool_search_output","execution":"client","call_id":"call_search","tools":[
				{"type":"function","name":"get_shipping_eta","defer_loading":true,"parameters":{"type":"object"}}
			]}
		]`),
		Tools: initial.Tools,
	}
	loadedRegistry, err := BuildResponsesToolRegistry(loaded)
	require.NoError(t, err)
	loadedChat, err := ResponsesToChatCompletionsRequestWithRegistry(loaded, loadedRegistry, DefaultChatCompletionsCapabilities())
	require.NoError(t, err)
	require.Len(t, loadedChat.Tools, 2)
	assert.Equal(t, "tool_search", loadedChat.Tools[0].Function.Name)
	assert.Equal(t, "get_shipping_eta", loadedChat.Tools[1].Function.Name)
}

func TestResponsesToolRegistry_DefersOnlyMarkedNamespaceChildren(t *testing.T) {
	req := &ResponsesRequest{
		Model: "glm-5.2",
		Input: json.RawMessage(`"inspect CRM"`),
		Tools: []ResponsesTool{
			{
				Type: "namespace",
				Name: "crm",
				Tools: []ResponsesTool{
					{Type: "function", Name: "get_customer", Parameters: json.RawMessage(`{"type":"object"}`)},
					{Type: "function", Name: "list_open_orders", DeferLoading: true, Parameters: json.RawMessage(`{"type":"object"}`)},
				},
			},
			{Type: "tool_search", Execution: "client"},
		},
	}
	registry, err := BuildResponsesToolRegistry(req)
	require.NoError(t, err)
	out, err := ResponsesToChatCompletionsRequestWithRegistry(req, registry, DefaultChatCompletionsCapabilities())
	require.NoError(t, err)
	require.Len(t, out.Tools, 2)
	assert.Equal(t, "crm__get_customer", out.Tools[0].Function.Name)
	assert.Equal(t, "tool_search", out.Tools[1].Function.Name)
}

func TestResponsesToolRegistry_PreservesLoadedTopLevelFunctionIdentity(t *testing.T) {
	req := &ResponsesRequest{
		Model: "glm-5.2",
		Input: json.RawMessage(`[
			{"type":"tool_search_call","execution":"client","call_id":"call_search","arguments":{"goal":"shipping"}},
			{"type":"tool_search_output","execution":"client","call_id":"call_search","tools":[
				{"type":"function","name":"get_shipping_eta","defer_loading":true,"parameters":{"type":"object"}}
			]},
			{"type":"function_call","name":"get_shipping_eta","namespace":"get_shipping_eta","call_id":"call_eta","arguments":"{}"},
			{"type":"function_call_output","call_id":"call_eta","output":"tomorrow"}
		]`),
	}
	registry, err := BuildResponsesToolRegistry(req)
	require.NoError(t, err)

	chatReq, err := ResponsesToChatCompletionsRequestWithRegistry(req, registry, DefaultChatCompletionsCapabilities())
	require.NoError(t, err)
	require.Len(t, chatReq.Tools, 1)
	assert.Equal(t, "get_shipping_eta", chatReq.Tools[0].Function.Name)
	require.Len(t, chatReq.Messages, 4)
	require.Len(t, chatReq.Messages[2].ToolCalls, 1)
	assert.Equal(t, "get_shipping_eta", chatReq.Messages[2].ToolCalls[0].Function.Name)

	chatResp := &ChatCompletionsResponse{
		ID: "chatcmpl_loaded",
		Choices: []ChatChoice{{Message: ChatMessage{
			Role: "assistant",
			ToolCalls: []ChatToolCall{{
				ID:       "call_next",
				Function: ChatFunctionCall{Name: "get_shipping_eta", Arguments: `{}`},
			}},
		}}},
	}
	converted := ChatCompletionsResponseToResponses(chatResp, req.Model, registry.CustomToolNames(), registry.HasClientToolSearch(DefaultChatCompletionsCapabilities()), registry.NamespaceToolNames())
	require.Len(t, converted.Output, 1)
	assert.Equal(t, "function_call", converted.Output[0].Type)
	assert.Equal(t, "get_shipping_eta", converted.Output[0].Name)
	assert.Equal(t, "get_shipping_eta", converted.Output[0].Namespace)
}

func TestResponsesToolRegistry_PreservesLoadedTopLevelFunctionIdentityInStream(t *testing.T) {
	registry, err := BuildResponsesToolRegistry(&ResponsesRequest{
		Input: json.RawMessage(`[
			{"type":"tool_search_call","execution":"client","call_id":"call_search","arguments":{"goal":"shipping"}},
			{"type":"tool_search_output","execution":"client","call_id":"call_search","tools":[
				{"type":"function","name":"get_shipping_eta","defer_loading":true,"parameters":{"type":"object"}}
			]}
		]`),
	})
	require.NoError(t, err)

	state := NewChatCompletionsToResponsesStreamState("glm-5.2")
	state.NamespaceTools = registry.NamespaceToolNames()
	idx := 0
	events := ChatCompletionsChunkToResponsesEvents(&ChatCompletionsChunk{
		ID: "chatcmpl_stream_loaded",
		Choices: []ChatChunkChoice{{Delta: ChatDelta{ToolCalls: []ChatToolCall{{
			Index:    &idx,
			ID:       "call_eta",
			Function: ChatFunctionCall{Name: "get_shipping_eta", Arguments: `{}`},
		}}}}},
	}, state)
	events = append(events, FinalizeChatCompletionsResponsesStream(state)...)

	var added, done *ResponsesOutput
	for i := range events {
		if events[i].Type == "response.output_item.added" && events[i].Item != nil && events[i].Item.Type == "function_call" {
			added = events[i].Item
		}
		if events[i].Type == "response.output_item.done" && events[i].Item != nil && events[i].Item.Type == "function_call" {
			done = events[i].Item
		}
	}
	require.NotNil(t, added)
	require.NotNil(t, done)
	require.NotEmpty(t, added.ID)
	assert.Equal(t, added.ID, done.ID)
	assert.Equal(t, "get_shipping_eta", added.Name)
	assert.Equal(t, "get_shipping_eta", added.Namespace)
	assert.Equal(t, "get_shipping_eta", done.Name)
	assert.Equal(t, "get_shipping_eta", done.Namespace)

	final := events[len(events)-1]
	require.Equal(t, "response.completed", final.Type)
	require.NotNil(t, final.Response)
	require.Len(t, final.Response.Output, 1)
	assert.Equal(t, added.ID, final.Response.Output[0].ID)
	assert.Equal(t, "get_shipping_eta", final.Response.Output[0].Name)
	assert.Equal(t, "get_shipping_eta", final.Response.Output[0].Namespace)
}

func TestResponsesToChatCompletionsRequest_RejectsHostedToolSearchHistory(t *testing.T) {
	_, err := ResponsesToChatCompletionsRequest(&ResponsesRequest{
		Model: "glm-5.2",
		Input: json.RawMessage(`[
			{"type":"tool_search_call","execution":"server","call_id":null,"arguments":{"paths":["crm"]}},
			{"type":"tool_search_output","execution":"server","call_id":null,"tools":[]}
		]`),
	})
	require.Error(t, err)
	var capabilityErr *ChatCompletionsCapabilityError
	require.ErrorAs(t, err, &capabilityErr)
	assert.Equal(t, "hosted_tool_search", capabilityErr.Feature)
}

func TestResponsesToChatCompletionsRequest_RejectsCustomGrammarWithoutOptIn(t *testing.T) {
	_, err := ResponsesToChatCompletionsRequest(&ResponsesRequest{
		Model: "glm-5.2",
		Input: json.RawMessage(`"apply patch"`),
		Tools: []ResponsesTool{{
			Type:   "custom",
			Name:   "apply_patch",
			Format: json.RawMessage(`{"type":"grammar","syntax":"lark","definition":"start: /foo/"}`),
		}},
	})
	require.Error(t, err)
	var capabilityErr *ChatCompletionsCapabilityError
	require.ErrorAs(t, err, &capabilityErr)
	assert.Equal(t, "chat_custom_tool_grammar", capabilityErr.Feature)
}

func TestResponsesToChatCompletionsRequest_AllowedToolsRequiresCapability(t *testing.T) {
	req := &ResponsesRequest{
		Model: "glm-5.2",
		Input: json.RawMessage(`"use browser"`),
		Tools: []ResponsesTool{{
			Type:  "namespace",
			Name:  "browser",
			Tools: []ResponsesTool{{Type: "function", Name: "open"}, {Type: "function", Name: "screenshot"}},
		}},
		ToolChoice: json.RawMessage(`{"type":"namespace","name":"browser"}`),
	}
	registry, err := BuildResponsesToolRegistry(req)
	require.NoError(t, err)

	_, err = ResponsesToChatCompletionsRequestWithRegistry(req, registry, ChatCompletionsCapabilities{})
	require.Error(t, err)
	var capabilityErr *ChatCompletionsCapabilityError
	require.ErrorAs(t, err, &capabilityErr)
	assert.Equal(t, "chat_allowed_tools", capabilityErr.Feature)
}

func TestResponsesToolDefinitionsEqual_DoesNotCollapseLargeJSONNumbers(t *testing.T) {
	left := ResponsesTool{Type: "function", Name: "large", Parameters: json.RawMessage(`{"type":"object","maximum":9007199254740992}`)}
	right := ResponsesTool{Type: "function", Name: "large", Parameters: json.RawMessage(`{"type":"object","maximum":9007199254740993}`)}
	assert.False(t, responsesToolDefinitionsEqual(left, right))
}

func TestResponsesToolRegistry_RejectsExcessiveToolCount(t *testing.T) {
	tools := make([]ResponsesTool, maxResponsesToolCount+1)
	for i := range tools {
		tools[i] = ResponsesTool{Type: "function", Name: "tool"}
	}
	_, err := BuildResponsesToolRegistry(&ResponsesRequest{Input: json.RawMessage(`"hi"`), Tools: tools})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tool count exceeds")
}

func TestResponsesToolRegistry_ReplacesRepeatedToolSearchOutputByCallID(t *testing.T) {
	req := &ResponsesRequest{Input: json.RawMessage(`[
		{"type":"tool_search_call","execution":"client","call_id":"call_search","arguments":{"goal":"tools"}},
		{"type":"tool_search_output","execution":"client","call_id":"call_search","tools":[
			{"type":"function","name":"old_tool","parameters":{"type":"object"}}
		]},
		{"type":"tool_search_output","execution":"client","call_id":"call_search","tools":[
			{"type":"function","name":"new_tool","parameters":{"type":"object"}}
		]}
	]`)}
	registry, err := BuildResponsesToolRegistry(req)
	require.NoError(t, err)
	tools := registry.Tools()
	require.Len(t, tools, 1)
	assert.Equal(t, "new_tool", tools[0].Name)
	assert.NotContains(t, registry.NamespaceToolNames(), "old_tool")
	assert.Equal(t, NamespacedToolName{Namespace: "new_tool", Name: "new_tool"}, registry.NamespaceToolNames()["new_tool"])

	chatReq, err := ResponsesToChatCompletionsRequestWithRegistry(req, registry, DefaultChatCompletionsCapabilities())
	require.NoError(t, err)
	var searchResults int
	for _, message := range chatReq.Messages {
		if message.Role == "tool" && message.ToolCallID == "call_search" {
			searchResults++
		}
	}
	assert.Equal(t, 1, searchResults, "repeated tool_search_output must not create duplicate Chat tool results")
}

func TestResponsesToolRegistry_RemovesToolsFromUpdatedToolSearchOutput(t *testing.T) {
	registry, err := BuildResponsesToolRegistry(&ResponsesRequest{Input: json.RawMessage(`[
		{"type":"tool_search_call","execution":"client","call_id":"call_search","arguments":{"goal":"tools"}},
		{"type":"tool_search_output","execution":"client","call_id":"call_search","tools":[
			{"type":"function","name":"temporary_tool","parameters":{"type":"object"}}
		]},
		{"type":"tool_search_output","execution":"client","call_id":"call_search","tools":[]}
	]`)})
	require.NoError(t, err)
	assert.Empty(t, registry.Tools())
	assert.Empty(t, registry.NamespaceToolNames())
}

func TestResponsesToolRegistry_RejectsConflictingResponseIdentity(t *testing.T) {
	_, err := BuildResponsesToolRegistry(&ResponsesRequest{Input: json.RawMessage(`[
		{"type":"tool_search_call","execution":"client","call_id":"call_search","arguments":{"goal":"tools"}},
		{"type":"tool_search_output","execution":"client","call_id":"call_search","tools":[
			{"type":"function","name":"foo","parameters":{"type":"object"}},
			{"type":"namespace","name":"foo","tools":[
				{"type":"function","name":"foo","parameters":{"type":"object"}}
			]}
		]}
	]`)})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `identity "foo"/"foo" maps to conflicting Chat tool names`)
}

func TestResponsesToolRegistry_RejectsTopLevelAndDynamicFunctionIdentityCollision(t *testing.T) {
	_, err := BuildResponsesToolRegistry(&ResponsesRequest{
		Tools: []ResponsesTool{{Type: "function", Name: "foo", Parameters: json.RawMessage(`{"type":"object"}`)}},
		Input: json.RawMessage(`[
			{"type":"tool_search_call","execution":"client","call_id":"call_search","arguments":{"goal":"tools"}},
			{"type":"tool_search_output","execution":"client","call_id":"call_search","tools":[
				{"type":"function","name":"foo","parameters":{"type":"object"}}
			]}
		]`),
	})
	require.Error(t, err)
	var capabilityErr *ChatCompletionsCapabilityError
	require.ErrorAs(t, err, &capabilityErr)
	assert.Equal(t, "chat_tool_identity", capabilityErr.Feature)
	assert.Contains(t, err.Error(), `same Chat tool name "foo"`)
}

func TestResponsesToolRegistry_UsesHistoricalIdentityBeforeRepeatedOutputReplacement(t *testing.T) {
	req := &ResponsesRequest{Input: json.RawMessage(`[
		{"type":"tool_search_call","execution":"client","call_id":"call_search","arguments":{"goal":"tools"}},
		{"type":"tool_search_output","execution":"client","call_id":"call_search","tools":[
			{"type":"function","name":"old_tool","parameters":{"type":"object"}}
		]},
		{"type":"function_call","namespace":"old_tool","name":"old_tool","call_id":"call_old","arguments":"{}"},
		{"type":"function_call_output","call_id":"call_old","output":"old result"},
		{"type":"tool_search_output","execution":"client","call_id":"call_search","tools":[
			{"type":"function","name":"new_tool","parameters":{"type":"object"}}
		]},
		{"type":"function_call","namespace":"new_tool","name":"new_tool","call_id":"call_new","arguments":"{}"},
		{"type":"function_call_output","call_id":"call_new","output":"new result"}
	]`)}
	registry, err := BuildResponsesToolRegistry(req)
	require.NoError(t, err)
	chatReq, err := ResponsesToChatCompletionsRequestWithRegistry(req, registry, DefaultChatCompletionsCapabilities())
	require.NoError(t, err)

	var callNames []string
	for _, message := range chatReq.Messages {
		for _, call := range message.ToolCalls {
			callNames = append(callNames, call.Function.Name)
		}
	}
	assert.Equal(t, []string{"tool_search", "old_tool", "new_tool"}, callNames)
	require.Len(t, chatReq.Tools, 1)
	assert.Equal(t, "new_tool", chatReq.Tools[0].Function.Name)
}

func TestResponsesToolRegistry_HistoricalIdentityLookupUsesReplayCache(t *testing.T) {
	registry, err := BuildResponsesToolRegistry(&ResponsesRequest{
		Tools: []ResponsesTool{{
			Type:  "namespace",
			Name:  "crm",
			Tools: []ResponsesTool{{Type: "function", Name: "get_customer"}},
		}},
		Input: json.RawMessage(`[
			{"type":"function_call","namespace":"crm","name":"get_customer","call_id":"call_customer","arguments":"{}"}
		]`),
	})
	require.NoError(t, err)

	// Historical names are resolved while replaying input. Clearing the source
	// registrations proves lookup is constant-time and does not rescan tools.
	registry.registrations = nil
	name, ok, err := registry.chatNameForResponseToolAt(0, "crm", "get_customer")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "crm__get_customer", name)
}

func TestResponsesToChatCompletionsRequest_RejectsUnknownToolSearchExecutionAsInvalidRequest(t *testing.T) {
	tests := []struct {
		name string
		req  *ResponsesRequest
	}{
		{
			name: "declaration",
			req: &ResponsesRequest{
				Input: json.RawMessage(`"find tools"`),
				Tools: []ResponsesTool{{Type: "tool_search", Execution: "bogus"}},
			},
		},
		{
			name: "call history",
			req: &ResponsesRequest{Input: json.RawMessage(`[
				{"type":"tool_search_call","execution":"bogus","call_id":"call_search","arguments":{"goal":"tools"}}
			]`)},
		},
		{
			name: "output history",
			req: &ResponsesRequest{Input: json.RawMessage(`[
				{"type":"tool_search_output","execution":"bogus","call_id":"call_search","tools":[]}
			]`)},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ResponsesToChatCompletionsRequest(tc.req)
			require.Error(t, err)
			var capabilityErr *ChatCompletionsCapabilityError
			assert.NotErrorAs(t, err, &capabilityErr)
			assert.Contains(t, err.Error(), "execution must be client or server")
		})
	}
}

func TestResponsesToChatCompletionsRequest_RejectsHostedToolsAsCapabilityMismatch(t *testing.T) {
	for _, toolType := range []string{"web_search", "image_generation", "file_search"} {
		t.Run(toolType, func(t *testing.T) {
			_, err := ResponsesToChatCompletionsRequest(&ResponsesRequest{
				Input: json.RawMessage(`"use hosted tool"`),
				Tools: []ResponsesTool{{Type: toolType}},
			})
			require.Error(t, err)
			var capabilityErr *ChatCompletionsCapabilityError
			require.ErrorAs(t, err, &capabilityErr)
			assert.Equal(t, "responses_hosted_tool", capabilityErr.Feature)
		})
	}
}

func TestResponsesToolRegistry_RejectsOversizedDefinition(t *testing.T) {
	tool := ResponsesTool{Type: "function", Name: "large", rawDefinition: make(json.RawMessage, maxResponsesToolDefinitionBytes+1)}
	_, err := BuildResponsesToolRegistry(&ResponsesRequest{Input: json.RawMessage(`"hi"`), Tools: []ResponsesTool{tool}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tool definition exceeds")
}

func TestResponsesToolRegistry_RejectsOversizedDefinitionTotal(t *testing.T) {
	raw := make(json.RawMessage, maxResponsesToolDefinitionBytes)
	tools := make([]ResponsesTool, maxResponsesToolTotalBytes/maxResponsesToolDefinitionBytes+1)
	for i := range tools {
		tools[i] = ResponsesTool{Type: "function", Name: "large", rawDefinition: raw}
	}
	_, err := BuildResponsesToolRegistry(&ResponsesRequest{Input: json.RawMessage(`"hi"`), Tools: tools})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tool definitions exceed")
}

func TestResponsesToolRegistry_RejectsExcessiveNamespaceDepth(t *testing.T) {
	tool := ResponsesTool{Type: "function", Name: "leaf"}
	for i := 0; i <= maxResponsesToolNamespaceDepth; i++ {
		tool = ResponsesTool{Type: "namespace", Name: "ns", Tools: []ResponsesTool{tool}}
	}
	_, err := BuildResponsesToolRegistry(&ResponsesRequest{Input: json.RawMessage(`"hi"`), Tools: []ResponsesTool{tool}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "namespace depth exceeds")
}

func TestValidateResponsesToolPayloadRejectsBeforeToolUnmarshal(t *testing.T) {
	body := []byte(`{"model":"test","input":"hi","tools":[{"type":"function","name":"large","description":"` +
		strings.Repeat("x", maxResponsesToolDefinitionBytes) + `"}]}`)
	err := ValidateResponsesToolPayload(body)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tool definition exceeds")
}

func TestValidateResponsesToolPayloadRejectsDuplicateRelevantKeys(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "duplicate root tools",
			body: `{"model":"test","input":"hi","tools":[],"tools":[{"type":"function","name":"hidden"}]}`,
		},
		{
			name: "case-insensitive duplicate root tools",
			body: `{"model":"test","input":"hi","tools":[],"Tools":[{"type":"function","name":"hidden"}]}`,
		},
		{
			name: "duplicate carrier tools",
			body: `{"model":"test","input":[{"type":"additional_tools","tools":[],"tools":[{"type":"function","name":"hidden"}]}]}`,
		},
		{
			name: "duplicate tool execution",
			body: `{"model":"test","input":"hi","tools":[{"type":"tool_search","execution":"client","execution":"bogus"}]}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateResponsesToolPayload([]byte(tc.body))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "duplicate key")
		})
	}
}

func TestValidateResponsesToolPayloadCountsAllowedToolReferences(t *testing.T) {
	entryPadding := strings.Repeat("x", maxResponsesToolDefinitionBytes-256)
	entry := `{"type":"function","name":"foo","unknown_padding":"` + entryPadding + `"}`
	require.LessOrEqual(t, len(entry), maxResponsesToolDefinitionBytes)

	var body strings.Builder
	_, _ = body.WriteString(`{"model":"test","input":"hi","tools":[{"type":"function","name":"foo"}],"tool_choice":{"type":"allowed_tools","mode":"auto","tools":[`)
	for i := 0; i < maxResponsesToolTotalBytes/maxResponsesToolDefinitionBytes+1; i++ {
		if i > 0 {
			_ = body.WriteByte(',')
		}
		_, _ = body.WriteString(entry)
	}
	_, _ = body.WriteString(`]}}`)

	err := ValidateResponsesToolPayload([]byte(body.String()))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tool definitions exceed")
}

func TestValidateResponsesToolPayloadRejectsExcessiveRootFields(t *testing.T) {
	var body strings.Builder
	_, _ = body.WriteString(`{"model":"test","input":"hi"`)
	for i := 0; i < maxResponsesJSONObjectFieldCount; i++ {
		_, _ = body.WriteString(`,"unknown_` + strconv.Itoa(i) + `":null`)
	}
	_ = body.WriteByte('}')

	err := ValidateResponsesToolPayload([]byte(body.String()))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Responses request field count exceeds")
}

func TestValidateResponsesToolPayloadRejectsExcessiveInputItemFields(t *testing.T) {
	var body strings.Builder
	_, _ = body.WriteString(`{"model":"test","input":[{"type":"message"`)
	for i := 0; i < maxResponsesJSONObjectFieldCount; i++ {
		_, _ = body.WriteString(`,"unknown_` + strconv.Itoa(i) + `":null`)
	}
	_, _ = body.WriteString(`}]}`)

	err := ValidateResponsesToolPayload([]byte(body.String()))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Responses input item 0 field count exceeds")
}

func TestValidateResponsesToolPayloadRejectsExcessiveContentPartFields(t *testing.T) {
	var body strings.Builder
	_, _ = body.WriteString(`{"model":"test","input":[{"type":"message","role":"user","content":[{"type":"input_text"`)
	for i := 0; i < maxResponsesJSONObjectFieldCount; i++ {
		_, _ = body.WriteString(`,"unknown_` + strconv.Itoa(i) + `":null`)
	}
	_, _ = body.WriteString(`}]}]}`)

	err := ValidateResponsesToolPayload([]byte(body.String()))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Responses input item 0 content part 0 field count exceeds")
}

func TestValidateResponsesToolPayloadRejectsExcessiveReasoningSummaryFields(t *testing.T) {
	var body strings.Builder
	_, _ = body.WriteString(`{"model":"test","input":[{"type":"reasoning","summary":[{"type":"summary_text"`)
	for i := 0; i < maxResponsesJSONObjectFieldCount; i++ {
		_, _ = body.WriteString(`,"unknown_` + strconv.Itoa(i) + `":null`)
	}
	_, _ = body.WriteString(`}]}]}`)

	err := ValidateResponsesToolPayload([]byte(body.String()))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Responses input item 0 summary part 0 field count exceeds")
}

func TestValidateResponsesToolPayloadRejectsExcessiveNestedImageURLFields(t *testing.T) {
	var body strings.Builder
	_, _ = body.WriteString(`{"model":"test","input":[{"type":"message","role":"user","content":[{"type":"input_image","image_url":{"url":"data:image/png;base64,AA=="`)
	for i := 0; i < maxResponsesJSONObjectFieldCount; i++ {
		_, _ = body.WriteString(`,"unknown_` + strconv.Itoa(i) + `":null`)
	}
	_, _ = body.WriteString(`}}]}]}`)

	err := ValidateResponsesToolPayload([]byte(body.String()))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "image_url field count exceeds")
}

func TestValidateResponsesToolPayloadRejectsExcessiveContentParts(t *testing.T) {
	var body strings.Builder
	_, _ = body.WriteString(`{"model":"test","input":[{"type":"message","role":"user","content":[`)
	for i := 0; i <= maxResponsesContentPartCount; i++ {
		if i > 0 {
			_ = body.WriteByte(',')
		}
		_, _ = body.WriteString(`{"type":"input_text","text":"x"}`)
	}
	_, _ = body.WriteString(`]}]}`)

	err := ValidateResponsesToolPayload([]byte(body.String()))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "content part count exceeds")
}

func TestResponsesContentConversionIgnoresUnknownPartFields(t *testing.T) {
	var raw strings.Builder
	_, _ = raw.WriteString(`{"type":"input_text","text":"hello"`)
	for i := 0; i < maxResponsesJSONObjectFieldCount*2; i++ {
		_, _ = raw.WriteString(`,"unknown_` + strconv.Itoa(i) + `":null`)
	}
	_ = raw.WriteByte('}')

	content, err := responsesContentToChatContent(json.RawMessage(raw.String()), "user")
	require.NoError(t, err)
	assert.JSONEq(t, `"hello"`, string(content))
}

func TestValidateResponsesToolPayloadRejectsDuplicateUnknownKeys(t *testing.T) {
	err := ValidateResponsesToolPayload([]byte(`{"model":"test","input":"hi","extension":1,"Extension":2}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate key")
}

func TestResponsesInputItemCountIsBoundedBeforeConversion(t *testing.T) {
	var input strings.Builder
	_ = input.WriteByte('[')
	for i := 0; i <= maxResponsesInputItemCount; i++ {
		if i > 0 {
			_ = input.WriteByte(',')
		}
		_, _ = input.WriteString(`null`)
	}
	_ = input.WriteByte(']')

	t.Run("raw payload validation", func(t *testing.T) {
		body := `{"model":"test","input":` + input.String() + `}`
		err := ValidateResponsesToolPayload([]byte(body))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "responses input item count exceeds")
	})

	t.Run("registry direct call", func(t *testing.T) {
		_, err := BuildResponsesToolRegistry(&ResponsesRequest{Input: json.RawMessage(input.String())})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "responses input item count exceeds")
	})
}
