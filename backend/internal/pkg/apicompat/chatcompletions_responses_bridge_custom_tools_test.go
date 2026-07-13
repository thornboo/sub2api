package apicompat

// custom/freeform 工具（如 Codex 0.14x 的 exec）在 responses→chat 桥上的双向转换。
// 背景：Codex 的核心命令执行工具 exec 是 type=custom（输入为自由文本），此前被
// responsesToolsToChatTools 丢弃，导致模型工具列表中没有 exec、无法执行任何命令。

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponsesToChatCompletionsRequest_CustomToolBecomesFunctionTool(t *testing.T) {
	req := &ResponsesRequest{
		Model: "glm-5.2",
		Input: json.RawMessage(`"run dir"`),
		Tools: []ResponsesTool{
			{Type: "custom", Name: "exec", Description: "Run JavaScript code"},
			{Type: "function", Name: "wait", Parameters: json.RawMessage(`{"type":"object","properties":{}}`)},
		},
	}

	out, err := ResponsesToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Len(t, out.Tools, 2)

	assert.Equal(t, "function", out.Tools[0].Type)
	assert.Equal(t, "exec", out.Tools[0].Function.Name)
	assert.Equal(t, "Run JavaScript code", out.Tools[0].Function.Description)
	assert.JSONEq(t, customToolInputSchema, string(out.Tools[0].Function.Parameters))

	assert.Equal(t, "wait", out.Tools[1].Function.Name)
}

func TestResponsesToChatCompletionsRequest_AdditionalToolsItem(t *testing.T) {
	req := &ResponsesRequest{
		Model: "gpt-test",
		Input: json.RawMessage(`[
			{"type":"additional_tools","role":"developer","tools":[
				{"type":"custom","name":"exec","description":"Run PowerShell"},
				{"type":"function","name":"wait","parameters":{"type":"object","properties":{}}},
				{"type":"namespace","name":"collaboration","tools":[
					{"type":"function","name":"send_message","parameters":{"type":"object","properties":{}}}
				]}
			]},
			{"type":"message","role":"user","content":[{"type":"input_text","text":"run Get-Location"}]}
		]`),
		ToolChoice: json.RawMessage(`"auto"`),
	}

	effective, err := ResponsesRequestTools(req)
	require.NoError(t, err)
	require.Len(t, effective, 3)
	assert.True(t, CustomToolNames(effective)["exec"])
	assert.Equal(t, NamespacedToolName{Namespace: "collaboration", Name: "send_message"}, NamespaceToolNames(effective)["collaboration__send_message"])

	out, err := ResponsesToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Len(t, out.Tools, 3)
	assert.Equal(t, "exec", out.Tools[0].Function.Name)
	assert.Equal(t, "wait", out.Tools[1].Function.Name)
	assert.Equal(t, "collaboration__send_message", out.Tools[2].Function.Name)
	assert.JSONEq(t, `"auto"`, string(out.ToolChoice))

	require.Len(t, out.Messages, 1, "additional_tools must not become a chat message")
	assert.Equal(t, "user", out.Messages[0].Role)
}

func TestResponsesRequestTools_SkipsStringInputItems(t *testing.T) {
	req := &ResponsesRequest{
		Input: json.RawMessage(`["plain input",{"type":"additional_tools","tools":[{"type":"custom","name":"exec"}]}]`),
	}

	tools, err := ResponsesRequestTools(req)
	require.NoError(t, err)
	require.Len(t, tools, 1)
	assert.Equal(t, "exec", tools[0].Name)
}

func TestResponsesToChatCompletionsRequest_RejectsHostedToolsInsteadOfDroppingThem(t *testing.T) {
	req := &ResponsesRequest{
		Model: "glm-5.2",
		Input: json.RawMessage(`"hi"`),
		Tools: []ResponsesTool{
			{Type: "web_search"},
			{Type: "image_generation"},
		},
		ToolChoice: json.RawMessage(`"auto"`),
	}

	_, err := ResponsesToChatCompletionsRequest(req)
	require.Error(t, err)
	var capabilityErr *ChatCompletionsCapabilityError
	require.ErrorAs(t, err, &capabilityErr)
	assert.Equal(t, "responses_hosted_tool", capabilityErr.Feature)
}

func TestResponsesToChatCompletionsRequest_CustomToolChoiceMapsToFunctionChoice(t *testing.T) {
	req := &ResponsesRequest{
		Model:      "glm-5.2",
		Input:      json.RawMessage(`"run dir"`),
		Tools:      []ResponsesTool{{Type: "custom", Name: "exec"}},
		ToolChoice: json.RawMessage(`{"type":"custom","name":"exec"}`),
	}

	out, err := ResponsesToChatCompletionsRequest(req)
	require.NoError(t, err)

	assert.JSONEq(t, `{"type":"function","function":{"name":"exec"}}`, string(out.ToolChoice))
}

func TestResponsesInputToChatMessages_CustomToolCallHistory(t *testing.T) {
	input := json.RawMessage(`[
		{"role":"user","content":"list files"},
		{"type":"custom_tool_call","call_id":"call_1","name":"exec","input":"dir"},
		{"type":"custom_tool_call_output","call_id":"call_1","output":"main.go"}
	]`)

	messages, err := responsesInputToChatMessages("", input)
	require.NoError(t, err)
	require.Len(t, messages, 3)

	assert.Equal(t, []string{"user", "assistant", "tool"}, chatMessageRoles(messages))

	require.Len(t, messages[1].ToolCalls, 1)
	toolCall := messages[1].ToolCalls[0]
	assert.Equal(t, "call_1", toolCall.ID)
	assert.Equal(t, "exec", toolCall.Function.Name)
	assert.JSONEq(t, `{"input":"dir"}`, toolCall.Function.Arguments)

	assert.Equal(t, "call_1", messages[2].ToolCallID)
	assert.JSONEq(t, `"main.go"`, string(messages[2].Content))
}

func TestChatCompletionsResponseToResponses_CustomToolCallOutputItem(t *testing.T) {
	resp := &ChatCompletionsResponse{
		ID: "cc-1",
		Choices: []ChatChoice{{
			Message: ChatMessage{
				Role: "assistant",
				ToolCalls: []ChatToolCall{
					{ID: "call_1", Function: ChatFunctionCall{Name: "exec", Arguments: `{"input": "dir"}`}},
					{ID: "call_2", Function: ChatFunctionCall{Name: "wait", Arguments: `{"cell_id": 3}`}},
				},
			},
		}},
	}

	out := ChatCompletionsResponseToResponses(resp, "glm-5.2", map[string]bool{"exec": true}, false, nil)
	require.Len(t, out.Output, 2)

	assert.Equal(t, "custom_tool_call", out.Output[0].Type)
	assert.Equal(t, "call_1", out.Output[0].CallID)
	assert.Equal(t, "exec", out.Output[0].Name)
	assert.Equal(t, "dir", out.Output[0].Input)
	assert.Empty(t, out.Output[0].Arguments)

	assert.Equal(t, "function_call", out.Output[1].Type)
	assert.Equal(t, "wait", out.Output[1].Name)
	assert.Equal(t, `{"cell_id": 3}`, out.Output[1].Arguments)
}

func TestExtractCustomToolCallInput_FallsBackToRawArguments(t *testing.T) {
	assert.Equal(t, "dir", extractCustomToolCallInput(`{"input": "dir"}`))
	assert.Equal(t, "console.log(1)", extractCustomToolCallInput(`console.log(1)`))
	assert.Equal(t, `{"other": "x"}`, extractCustomToolCallInput(`{"other": "x"}`))
	assert.Equal(t, "", extractCustomToolCallInput(`{}`))
	assert.Equal(t, "", extractCustomToolCallInput(`null`))
	assert.Equal(t, `[]`, extractCustomToolCallInput(`[]`))
	assert.Equal(t, "", extractCustomToolCallInput(""))
}

func TestExtractCustomToolCallInput_IgnoresLargeUnknownFieldSet(t *testing.T) {
	var arguments strings.Builder
	_ = arguments.WriteByte('{')
	for i := 0; i < 4096; i++ {
		if i > 0 {
			_ = arguments.WriteByte(',')
		}
		_, _ = arguments.WriteString(`"unknown_` + strconv.Itoa(i) + `":null`)
	}
	_, _ = arguments.WriteString(`,"input":"payload"}`)

	assert.Equal(t, "payload", extractCustomToolCallInput(arguments.String()))
}

func TestChatCompletionsChunkToResponsesEvents_CustomToolCallStream(t *testing.T) {
	state := NewChatCompletionsToResponsesStreamState("glm-5.2")
	state.CustomTools = map[string]bool{"exec": true}

	idx := 0
	chunk := &ChatCompletionsChunk{
		ID: "cc-1",
		Choices: []ChatChunkChoice{{
			Delta: ChatDelta{
				ToolCalls: []ChatToolCall{{
					Index:    &idx,
					ID:       "call_1",
					Function: ChatFunctionCall{Name: "exec", Arguments: `{"input": "dir"}`},
				}},
			},
		}},
	}

	events := ChatCompletionsChunkToResponsesEvents(chunk, state)
	events = append(events, FinalizeChatCompletionsResponsesStream(state)...)

	var added, inputDone, itemDone *ResponsesStreamEvent
	for i := range events {
		evt := &events[i]
		switch evt.Type {
		case "response.output_item.added":
			if evt.Item != nil && evt.Item.Type != "message" && evt.Item.Type != "reasoning" {
				added = evt
			}
		case "response.custom_tool_call_input.done":
			inputDone = evt
		case "response.output_item.done":
			if evt.Item != nil && evt.Item.Type == "custom_tool_call" {
				itemDone = evt
			}
		case "response.function_call_arguments.delta", "response.function_call_arguments.done":
			t.Fatalf("custom 工具调用不应产出 function_call 参数事件: %s", evt.Type)
		}
	}

	require.NotNil(t, added, "缺少 custom_tool_call 的 output_item.added")
	assert.Equal(t, "custom_tool_call", added.Item.Type)
	assert.Equal(t, "exec", added.Item.Name)

	require.NotNil(t, inputDone, "缺少 response.custom_tool_call_input.done")
	assert.Equal(t, "dir", inputDone.Input)
	assert.Equal(t, "call_1", inputDone.CallID)

	require.NotNil(t, itemDone, "缺少 custom_tool_call 的 output_item.done")
	assert.Equal(t, added.Item.ID, itemDone.Item.ID)
	assert.Equal(t, "call_1", itemDone.Item.CallID)
	assert.Equal(t, "exec", itemDone.Item.Name)
	assert.Equal(t, "dir", itemDone.Item.Input)
	assert.Empty(t, itemDone.Item.Arguments)

	// response.completed 的 output 数组同样携带 custom_tool_call 项。
	final := events[len(events)-1]
	require.Equal(t, "response.completed", final.Type)
	require.NotNil(t, final.Response)
	foundCustom := false
	for _, item := range final.Response.Output {
		if item.Type == "custom_tool_call" {
			foundCustom = true
			assert.Equal(t, added.Item.ID, item.ID)
			assert.Equal(t, "exec", item.Name)
			assert.Equal(t, "dir", item.Input)
		}
	}
	assert.True(t, foundCustom, "response.completed 缺少 custom_tool_call 输出项")
}

func TestResponsesToChatCompletionsRequest_ToolSearchToolBecomesProxyFunction(t *testing.T) {
	req := &ResponsesRequest{
		Model: "glm-5.2",
		Input: json.RawMessage(`"hi"`),
		Tools: []ResponsesTool{{
			Type:        "tool_search",
			Execution:   "client",
			Description: "Find project tools",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"goal":{"type":"string"}},"required":["goal"]}`),
		}},
	}

	out, err := ResponsesToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Len(t, out.Tools, 1)

	assert.Equal(t, "function", out.Tools[0].Type)
	assert.Equal(t, "tool_search", out.Tools[0].Function.Name)
	assert.Equal(t, "Find project tools", out.Tools[0].Function.Description)
	assert.JSONEq(t, `{"type":"object","properties":{"goal":{"type":"string"}},"required":["goal"]}`, string(out.Tools[0].Function.Parameters))
}

func TestResponsesToChatCompletionsRequest_RejectsServerToolSearch(t *testing.T) {
	_, err := ResponsesToChatCompletionsRequest(&ResponsesRequest{
		Model: "glm-5.2",
		Input: json.RawMessage(`"find gmail tools"`),
		Tools: []ResponsesTool{{Type: "tool_search", Execution: "server"}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "execution=server")
}

// codex 只在 ResponseItem 为 tool_search_call 变体且 execution=client 时执行
// tool search；同名 function_call 会命中 ToolSearchHandler 后因 payload 不匹配
// 触发 FunctionCallError::Fatal，直接中止整个 turn，因此回程必须还原项类型。
func TestChatCompletionsResponseToResponses_ToolSearchCallOutputItem(t *testing.T) {
	resp := &ChatCompletionsResponse{
		ID: "cc-1",
		Choices: []ChatChoice{{
			Message: ChatMessage{
				Role: "assistant",
				ToolCalls: []ChatToolCall{
					{ID: "call_s", Function: ChatFunctionCall{Name: "tool_search", Arguments: `{"query":"gmail","limit":2}`}},
				},
			},
		}},
	}

	out := ChatCompletionsResponseToResponses(resp, "glm-5.2", nil, true, nil)
	require.Len(t, out.Output, 1)

	item := out.Output[0]
	assert.Equal(t, "tool_search_call", item.Type)
	assert.Equal(t, "call_s", item.CallID)

	// 线上形态：execution 必须为 "client"（codex 的必填字段，非 client 被忽略），
	// arguments 必须是 JSON 对象而非字符串（codex 按对象解析 query/limit）。
	b, err := json.Marshal(item)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(b, &m))
	assert.Equal(t, "client", m["execution"])
	args, ok := m["arguments"].(map[string]any)
	require.True(t, ok, "arguments 必须序列化为 JSON 对象")
	assert.Equal(t, "gmail", args["query"])
}

func TestChatCompletionsResponseToResponses_ToolSearchNotDeclaredKeepsFunctionCall(t *testing.T) {
	resp := &ChatCompletionsResponse{
		Choices: []ChatChoice{{
			Message: ChatMessage{
				Role: "assistant",
				ToolCalls: []ChatToolCall{
					{ID: "call_s", Function: ChatFunctionCall{Name: "tool_search", Arguments: `{"query":"gmail"}`}},
				},
			},
		}},
	}

	// 客户端未声明 type=tool_search 时，同名普通 function 工具不受影响。
	out := ChatCompletionsResponseToResponses(resp, "glm-5.2", nil, false, nil)
	require.Len(t, out.Output, 1)
	assert.Equal(t, "function_call", out.Output[0].Type)
}

func TestChatCompletionsChunkToResponsesEvents_ToolSearchCallStream(t *testing.T) {
	state := NewChatCompletionsToResponsesStreamState("glm-5.2")
	state.ToolSearchDeclared = true

	idx := 0
	chunk := &ChatCompletionsChunk{
		ID: "cc-1",
		Choices: []ChatChunkChoice{{
			Delta: ChatDelta{
				ToolCalls: []ChatToolCall{{
					Index:    &idx,
					ID:       "call_s",
					Function: ChatFunctionCall{Name: "tool_search", Arguments: `{"query":"gmail"}`},
				}},
			},
		}},
	}

	events := ChatCompletionsChunkToResponsesEvents(chunk, state)
	events = append(events, FinalizeChatCompletionsResponsesStream(state)...)

	var added, itemDone *ResponsesStreamEvent
	for i := range events {
		evt := &events[i]
		switch evt.Type {
		case "response.output_item.added":
			if evt.Item != nil && evt.Item.Type != "message" && evt.Item.Type != "reasoning" {
				added = evt
			}
		case "response.output_item.done":
			if evt.Item != nil && evt.Item.Type == "tool_search_call" {
				itemDone = evt
			}
		case "response.function_call_arguments.delta", "response.function_call_arguments.done",
			"response.custom_tool_call_input.delta", "response.custom_tool_call_input.done":
			t.Fatalf("tool_search 调用不应产出 %s", evt.Type)
		}
	}

	require.NotNil(t, added, "缺少 tool_search_call 的 output_item.added")
	assert.Equal(t, "tool_search_call", added.Item.Type)

	require.NotNil(t, itemDone, "缺少 tool_search_call 的 output_item.done")
	assert.Equal(t, added.Item.ID, itemDone.Item.ID)
	assert.Equal(t, "call_s", itemDone.Item.CallID)

	// SSE 线上形态经 responsesItemWire 白名单重组，必须单独断言。
	sse, err := ResponsesEventToSSE(*itemDone)
	require.NoError(t, err)
	assert.Contains(t, sse, `"execution":"client"`)
	assert.Contains(t, sse, `"arguments":{"query":"gmail"}`)
	assert.Contains(t, sse, `"call_id":"call_s"`)

	// response.completed 的 output 数组同样携带 tool_search_call 项。
	final := events[len(events)-1]
	require.Equal(t, "response.completed", final.Type)
	require.NotNil(t, final.Response)
	found := false
	for _, item := range final.Response.Output {
		if item.Type == "tool_search_call" {
			found = true
			assert.Equal(t, added.Item.ID, item.ID)
			assert.Equal(t, "call_s", item.CallID)
		}
	}
	assert.True(t, found, "response.completed 缺少 tool_search_call 输出项")
}

func TestHasToolSearchTool(t *testing.T) {
	assert.True(t, HasToolSearchTool([]ResponsesTool{{Type: "tool_search", Execution: "client"}}))
	assert.False(t, HasToolSearchTool([]ResponsesTool{{Type: "tool_search"}}), "type-only tool_search is hosted, not client executed")
	assert.False(t, HasToolSearchTool([]ResponsesTool{{Type: "function", Name: "tool_search"}}))
	assert.False(t, HasToolSearchTool(nil))
}

func TestResponsesToChatCompletionsRequest_NamespaceToolFlattensChildren(t *testing.T) {
	req := &ResponsesRequest{
		Model: "glm-5.2",
		Input: json.RawMessage(`"hi"`),
		Tools: []ResponsesTool{{
			Type: "namespace",
			Name: "gmail",
			Tools: []ResponsesTool{
				{Type: "function", Name: "send", Description: "Send mail", Parameters: json.RawMessage(`{"type":"object","properties":{}}`)},
				{Type: "custom", Name: "ignored_child"},
			},
		}},
	}

	out, err := ResponsesToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Len(t, out.Tools, 1, "namespace 子工具中仅 function 类型被摊平")

	assert.Equal(t, "gmail__send", out.Tools[0].Function.Name)
	assert.Equal(t, "Send mail", out.Tools[0].Function.Description)
}

func TestResponsesRequestTools_MergesResponsesLiteAdditionalTools(t *testing.T) {
	req := &ResponsesRequest{
		Model: "glm-5.2",
		Input: json.RawMessage(`[
			{"role":"user","content":"open the page"},
			{"type":"additional_tools","role":"developer","tools":[
				{"type":"namespace","name":"browser","tools":[{"type":"function","name":"open","parameters":{"type":"object"}}]},
				"exec",
				{"type":"tool_search","execution":"client"}
			]}
		]`),
		Tools: []ResponsesTool{{Type: "function", Name: "wait"}},
	}

	tools, err := ResponsesRequestTools(req)
	require.NoError(t, err)
	require.Len(t, tools, 4)
	assert.Equal(t, "wait", tools[0].Name)
	assert.Equal(t, "namespace", tools[1].Type)
	assert.Equal(t, "browser", tools[1].Name)
	assert.Equal(t, "custom", tools[2].Type)
	assert.Equal(t, "exec", tools[2].Name)
	assert.Equal(t, "tool_search", tools[3].Type)

	out, err := ResponsesToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Len(t, out.Tools, 4)
	assert.Equal(t, "wait", out.Tools[0].Function.Name)
	assert.Equal(t, "browser__open", out.Tools[1].Function.Name)
	assert.Equal(t, "exec", out.Tools[2].Function.Name)
	assert.Equal(t, "tool_search", out.Tools[3].Function.Name)
	require.Len(t, out.Messages, 1, "additional_tools carrier must not become a chat message")
	assert.Equal(t, "user", out.Messages[0].Role)
}

func TestResponsesToolsParsing_StringToolBecomesCustom(t *testing.T) {
	var req ResponsesRequest
	require.NoError(t, json.Unmarshal([]byte(`{"model":"glm-5.2","input":"hi","tools":["exec",{"type":"function","name":"wait"}]}`), &req))

	require.Len(t, req.Tools, 2)
	assert.Equal(t, "custom", req.Tools[0].Type)
	assert.Equal(t, "exec", req.Tools[0].Name)
	assert.Equal(t, "function", req.Tools[1].Type)

	assert.True(t, CustomToolNames(req.Tools)["exec"])
}

func TestFlattenNamespaceToolName_CapsAt64WithHashSuffix(t *testing.T) {
	assert.Equal(t, "gmail__send", flattenNamespaceToolName("gmail", "send"))

	long := flattenNamespaceToolName("very_long_namespace_prefix_for_testing_purposes", "and_a_rather_long_tool_name_too")
	assert.LessOrEqual(t, len(long), 64)
	assert.Contains(t, long, "__")
	// 同输入结果稳定
	assert.Equal(t, long, flattenNamespaceToolName("very_long_namespace_prefix_for_testing_purposes", "and_a_rather_long_tool_name_too"))
}

func TestResponsesInputToChatMessages_ToolSearchCallHistory(t *testing.T) {
	input := json.RawMessage(`[
		{"role":"user","content":"find tools"},
		{"type":"tool_search_call","execution":"client","call_id":"call_s","arguments":{"query":"gmail"}},
		{"type":"tool_search_output","execution":"client","call_id":"call_s","status":"completed","tools":[
			{"type":"namespace","name":"gmail","tools":[{"type":"function","name":"send","parameters":{"type":"object"}}]}
		]}
	]`)

	messages, err := responsesInputToChatMessages("", input)
	require.NoError(t, err)
	require.Len(t, messages, 3)

	require.Len(t, messages[1].ToolCalls, 1)
	assert.Equal(t, "tool_search", messages[1].ToolCalls[0].Function.Name)
	assert.JSONEq(t, `{"query":"gmail"}`, messages[1].ToolCalls[0].Function.Arguments)

	assert.Equal(t, "tool", messages[2].Role)
	assert.Equal(t, "call_s", messages[2].ToolCallID)
	var loadedTools string
	require.NoError(t, json.Unmarshal(messages[2].Content, &loadedTools))
	assert.JSONEq(t, `[{"type":"namespace","name":"gmail","tools":[{"type":"function","name":"send","parameters":{"type":"object"}}]}]`, loadedTools)
}

func TestResponsesToChatCompletionsRequest_ToolSearchOutputLoadsToolsForNextTurn(t *testing.T) {
	req := &ResponsesRequest{
		Model: "glm-5.2",
		Input: json.RawMessage(`[
			{"role":"user","content":"find the CRM tools"},
			{"type":"tool_search_call","execution":"client","call_id":"call_search","arguments":{"goal":"find orders"}},
			{"type":"tool_search_output","execution":"client","call_id":"call_search","status":"completed","tools":[
				{"type":"function","name":"get_shipping_eta","parameters":{"type":"object"}},
				{"type":"namespace","name":"crm","tools":[{"type":"function","name":"list_open_orders","parameters":{"type":"object"}}]}
			]}
		]`),
	}

	requestTools, err := ResponsesRequestTools(req)
	require.NoError(t, err)
	require.Len(t, requestTools, 2)
	assert.Equal(t, "get_shipping_eta", requestTools[0].Name)
	assert.Equal(t, "crm", requestTools[1].Name)
	assert.False(t, HasToolSearchTool(requestTools), "official second turn relies on tool_search_output.tools without redeclaring tool_search")

	out, err := ResponsesToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Len(t, out.Tools, 2)
	assert.Equal(t, "get_shipping_eta", out.Tools[0].Function.Name)
	assert.Equal(t, "crm__list_open_orders", out.Tools[1].Function.Name)
	require.Len(t, out.Messages, 3)
	assert.Equal(t, "call_search", out.Messages[2].ToolCallID)

	chatResponse := &ChatCompletionsResponse{
		ID:    "chatcmpl_1",
		Model: "glm-5.2",
		Choices: []ChatChoice{{Message: ChatMessage{
			Role: "assistant",
			ToolCalls: []ChatToolCall{{
				ID:       "call_orders",
				Type:     "function",
				Function: ChatFunctionCall{Name: "crm__list_open_orders", Arguments: `{}`},
			}},
		}}},
	}
	converted := ChatCompletionsResponseToResponses(chatResponse, req.Model, CustomToolNames(requestTools), HasToolSearchTool(requestTools), NamespaceToolNames(requestTools))
	require.Len(t, converted.Output, 1)
	assert.Equal(t, "function_call", converted.Output[0].Type)
	assert.Equal(t, "crm", converted.Output[0].Namespace)
	assert.Equal(t, "list_open_orders", converted.Output[0].Name)
}

func TestResponsesToChatCompletionsRequest_DedupesEquivalentToolsAndRejectsConflictingDefinitions(t *testing.T) {
	out, err := ResponsesToChatCompletionsRequest(&ResponsesRequest{
		Model: "glm-5.2",
		Input: json.RawMessage(`"hi"`),
		Tools: []ResponsesTool{
			{Type: "function", Name: "wait", Parameters: json.RawMessage(`{"type":"object","properties":{"id":{"type":"string"}}}`)},
			{Type: "function", Name: "wait", Parameters: json.RawMessage(`{"properties":{"id":{"type":"string"}},"type":"object"}`)},
		},
	})
	require.NoError(t, err)
	require.Len(t, out.Tools, 1, "semantically equivalent duplicate tools should be emitted once")

	_, err = ResponsesToChatCompletionsRequest(&ResponsesRequest{
		Model: "glm-5.2",
		Input: json.RawMessage(`"hi"`),
		Tools: []ResponsesTool{
			{Type: "function", Name: "wait", Parameters: json.RawMessage(`{"type":"object","properties":{"id":{"type":"string"}}}`)},
			{Type: "function", Name: "wait", Parameters: json.RawMessage(`{"type":"object","properties":{"id":{"type":"integer"}}}`)},
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conflicting definitions")
	assert.Contains(t, err.Error(), "wait")
}

func TestResponsesToChatCompletionsRequest_CustomGrammarParticipatesInDefinitionConflicts(t *testing.T) {
	var equivalent ResponsesRequest
	require.NoError(t, json.Unmarshal([]byte(`{
		"model":"glm-5.2",
		"input":"hi",
		"tools":[
			{
				"type":"custom",
				"name":"apply_patch",
				"description":"Apply patch",
				"format":{"type":"grammar","syntax":"lark","definition":"start: /foo/"}
			},
			{
				"description":"Apply patch",
				"name":"apply_patch",
				"format":{"definition":"start: /foo/","syntax":"lark","type":"grammar"},
				"type":"custom"
			}
		]
	}`), &equivalent))
	registry, err := BuildResponsesToolRegistry(&equivalent)
	require.NoError(t, err)
	capabilities := DefaultChatCompletionsCapabilities()
	capabilities.AllowLossyCustomToolGrammar = true
	out, err := ResponsesToChatCompletionsRequestWithRegistry(&equivalent, registry, capabilities)
	require.NoError(t, err)
	require.Len(t, out.Tools, 1, "equivalent custom grammar definitions should be emitted once")

	var conflicting ResponsesRequest
	require.NoError(t, json.Unmarshal([]byte(`{
		"model":"glm-5.2",
		"input":"hi",
		"tools":[
			{
				"type":"custom",
				"name":"apply_patch",
				"description":"Apply patch",
				"format":{"type":"grammar","syntax":"lark","definition":"start: /foo/"}
			},
			{
				"type":"custom",
				"name":"apply_patch",
				"description":"Apply patch",
				"format":{"type":"grammar","syntax":"lark","definition":"start: /bar/"}
			}
		]
	}`), &conflicting))
	registry, err = BuildResponsesToolRegistry(&conflicting)
	require.NoError(t, err)
	_, err = ResponsesToChatCompletionsRequestWithRegistry(&conflicting, registry, capabilities)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conflicting definitions")
	assert.Contains(t, err.Error(), "apply_patch")
}

func TestResponsesInputToChatMessages_NamespacedFunctionCallHistory(t *testing.T) {
	input := json.RawMessage(`[
		{"type":"function_call","call_id":"call_n","name":"send","namespace":"gmail","arguments":"{\"to\":\"a\"}"},
		{"type":"function_call_output","call_id":"call_n","output":"ok"}
	]`)

	messages, err := responsesInputToChatMessages("", input)
	require.NoError(t, err)
	require.Len(t, messages, 2)

	require.Len(t, messages[0].ToolCalls, 1)
	assert.Equal(t, "gmail__send", messages[0].ToolCalls[0].Function.Name)
}

func TestChatCompletionsChunkToResponsesEvents_CustomToolNameArrivesLate(t *testing.T) {
	state := NewChatCompletionsToResponsesStreamState("glm-5.2")
	state.CustomTools = map[string]bool{"exec": true}

	idx := 0
	chunk1 := &ChatCompletionsChunk{Choices: []ChatChunkChoice{{Delta: ChatDelta{
		ToolCalls: []ChatToolCall{{Index: &idx, ID: "call_1", Function: ChatFunctionCall{Arguments: `{"inp`}}},
	}}}}
	chunk2 := &ChatCompletionsChunk{Choices: []ChatChunkChoice{{Delta: ChatDelta{
		ToolCalls: []ChatToolCall{{Index: &idx, Function: ChatFunctionCall{Name: "exec", Arguments: `ut": "dir"}`}}},
	}}}}

	var events []ResponsesStreamEvent
	events = append(events, ChatCompletionsChunkToResponsesEvents(chunk1, state)...)
	events = append(events, ChatCompletionsChunkToResponsesEvents(chunk2, state)...)
	assert.Empty(t, state.ToolCalls[idx].Function.Arguments, "分片期间不得通过 string += 重复复制完整参数")
	assert.Equal(t, `{"input": "dir"}`, state.toolArgumentsFor(idx))
	events = append(events, FinalizeChatCompletionsResponsesStream(state)...)

	addedCount := 0
	for _, evt := range events {
		switch evt.Type {
		case "response.output_item.added":
			if evt.Item != nil && evt.Item.Type != "reasoning" && evt.Item.Type != "message" {
				addedCount++
				assert.Equal(t, "custom_tool_call", evt.Item.Type, "迟到的名字命中 custom 工具时按 custom_tool_call 宣告")
				assert.Equal(t, "exec", evt.Item.Name)
			}
		case "response.function_call_arguments.delta", "response.function_call_arguments.done":
			t.Fatalf("custom 调用不应产出 function 参数事件: %s", evt.Type)
		case "response.custom_tool_call_input.done":
			assert.Equal(t, "dir", evt.Input)
		}
	}
	assert.Equal(t, 1, addedCount, "工具调用只宣告一次")
}

func TestChatCompletionsChunkToResponsesEvents_FunctionToolNameArrivesLate(t *testing.T) {
	state := NewChatCompletionsToResponsesStreamState("glm-5.2")
	state.CustomTools = map[string]bool{"exec": true}

	idx := 0
	chunk1 := &ChatCompletionsChunk{Choices: []ChatChunkChoice{{Delta: ChatDelta{
		ToolCalls: []ChatToolCall{{Index: &idx, ID: "call_9", Function: ChatFunctionCall{Arguments: `{"cell`}}},
	}}}}
	chunk2 := &ChatCompletionsChunk{Choices: []ChatChunkChoice{{Delta: ChatDelta{
		ToolCalls: []ChatToolCall{{Index: &idx, Function: ChatFunctionCall{Name: "wait", Arguments: `_id": 3}`}}},
	}}}}

	var events []ResponsesStreamEvent
	events = append(events, ChatCompletionsChunkToResponsesEvents(chunk1, state)...)
	events = append(events, ChatCompletionsChunkToResponsesEvents(chunk2, state)...)
	assert.Empty(t, state.ToolCalls[idx].Function.Arguments, "分片期间不得通过 string += 重复复制完整参数")
	assert.Equal(t, `{"cell_id": 3}`, state.toolArgumentsFor(idx))
	events = append(events, FinalizeChatCompletionsResponsesStream(state)...)

	deltas := ""
	argsDone := ""
	for _, evt := range events {
		switch evt.Type {
		case "response.function_call_arguments.delta":
			deltas += evt.Delta
		case "response.function_call_arguments.done":
			argsDone = evt.Arguments
		case "response.custom_tool_call_input.done":
			t.Fatal("function 调用不应产出 custom 事件")
		}
	}
	assert.Equal(t, `{"cell_id": 3}`, deltas, "宣告前累积的参数需在宣告时补发")
	assert.Equal(t, `{"cell_id": 3}`, argsDone)
}

func TestChatCompletionsChunkToResponsesEvents_RejectsToolArgumentLimits(t *testing.T) {
	tests := []struct {
		name      string
		toolName  string
		configure func(*ChatCompletionsToResponsesStreamState)
	}{
		{
			name:     "ordinary function",
			toolName: "wait",
		},
		{
			name:     "custom tool",
			toolName: "exec",
			configure: func(state *ChatCompletionsToResponsesStreamState) {
				state.CustomTools = map[string]bool{"exec": true}
			},
		},
		{
			name:     "tool search",
			toolName: toolSearchProxyName,
			configure: func(state *ChatCompletionsToResponsesStreamState) {
				state.ToolSearchDeclared = true
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state := NewChatCompletionsToResponsesStreamState("glm-5.2")
			state.maxToolArgumentBytes = 8
			state.maxTotalToolArgumentBytes = 16
			if tc.configure != nil {
				tc.configure(state)
			}
			idx := 0
			events := ChatCompletionsChunkToResponsesEvents(&ChatCompletionsChunk{
				Choices: []ChatChunkChoice{{Delta: ChatDelta{ToolCalls: []ChatToolCall{{
					Index: &idx,
					ID:    "call_limit",
					Function: ChatFunctionCall{
						Name:      tc.toolName,
						Arguments: "123456789",
					},
				}}}}},
			}, state)

			require.Error(t, state.StreamError())
			assert.Contains(t, state.StreamError().Error(), "exceed 8 bytes")
			assert.True(t, hasResponseStreamEventType(events, "response.failed"))
			assert.False(t, hasResponseStreamEventType(events, "response.output_item.done"))
			assert.Empty(t, FinalizeChatCompletionsResponsesStream(state), "超限后不得正常收尾")
		})
	}
}

func TestChatCompletionsChunkToResponsesEvents_RejectsTotalToolArgumentLimit(t *testing.T) {
	state := NewChatCompletionsToResponsesStreamState("glm-5.2")
	state.maxToolArgumentBytes = 8
	state.maxTotalToolArgumentBytes = 10
	first, second := 0, 1
	events := ChatCompletionsChunkToResponsesEvents(&ChatCompletionsChunk{
		Choices: []ChatChunkChoice{{Delta: ChatDelta{ToolCalls: []ChatToolCall{
			{Index: &first, ID: "call_1", Function: ChatFunctionCall{Name: "first", Arguments: "123456"}},
			{Index: &second, ID: "call_2", Function: ChatFunctionCall{Name: "second", Arguments: "abcdef"}},
		}}}},
	}, state)

	require.Error(t, state.StreamError())
	assert.Contains(t, state.StreamError().Error(), "exceed 10 total bytes")
	assert.Equal(t, "123456", state.toolArgumentsFor(first))
	assert.Empty(t, state.toolArgumentsFor(second), "超限分片不得进入累计 buffer")
	assert.True(t, hasResponseStreamEventType(events, "response.failed"))
	assert.False(t, hasResponseStreamEventType(events, "response.output_item.done"))
}

func hasResponseStreamEventType(events []ResponsesStreamEvent, eventType string) bool {
	for _, event := range events {
		if event.Type == eventType {
			return true
		}
	}
	return false
}

// 序列化层（MarshalJSON → responsesItemWire）单独走白名单重组，事件结构体上的字段
// 齐全不代表落到 SSE 线上的 JSON 齐全，必须在 wire 层再断言一次。
func TestResponsesEventToSSE_CustomToolCallItemCarriesAllFields(t *testing.T) {
	evt := ResponsesStreamEvent{
		Type:        "response.output_item.done",
		OutputIndex: 1,
		Item: &ResponsesOutput{
			Type:   "custom_tool_call",
			ID:     "item_1",
			CallID: "call_1",
			Name:   "exec",
			Input:  "dir",
			Status: "completed",
		},
	}

	sse, err := ResponsesEventToSSE(evt)
	require.NoError(t, err)

	assert.Contains(t, sse, `"call_id":"call_1"`)
	assert.Contains(t, sse, `"name":"exec"`)
	assert.Contains(t, sse, `"input":"dir"`)
	assert.Contains(t, sse, `"type":"custom_tool_call"`)
}

func TestNamespaceToolNames_MapsFlattenedNames(t *testing.T) {
	tools := []ResponsesTool{
		{Type: "namespace", Name: "gmail", Tools: []ResponsesTool{
			{Type: "function", Name: "send"},
			{Type: "custom", Name: "skip_me"},
		}},
		{Type: "namespace", Name: "crm", Children: []ResponsesTool{
			{Type: "function", Name: "query"},
		}},
		{Type: "function", Name: "wait"},
	}

	m := NamespaceToolNames(tools)
	require.Len(t, m, 2)
	assert.Equal(t, NamespacedToolName{Namespace: "gmail", Name: "send"}, m["gmail__send"])
	assert.Equal(t, NamespacedToolName{Namespace: "crm", Name: "query"}, m["crm__query"])

	// 摊平名超长时截断加哈希，无法按字符串切分还原，必须经映射反查。
	longNS := "very_long_namespace_prefix_for_testing_purposes"
	longChild := "and_a_rather_long_tool_name_too"
	m2 := NamespaceToolNames([]ResponsesTool{{
		Type: "namespace", Name: longNS,
		Tools: []ResponsesTool{{Type: "function", Name: longChild}},
	}})
	assert.Equal(t, NamespacedToolName{Namespace: longNS, Name: longChild},
		m2[flattenNamespaceToolName(longNS, longChild)])

	assert.Nil(t, NamespaceToolNames(nil))
}

// 内置 tool_search 降级后的代理 function 与客户端声明的同名工具无法区分：回程会把
// 普通工具的调用劫持成 tool_search_call，必须显式拒绝（代理不能改名，codex 的模型
// 侧按 tool_search 这个名字调用）。
func TestResponsesToChatCompletionsRequest_RejectsToolSearchNameConflict(t *testing.T) {
	// 与顶层 function 工具同名。
	_, err := ResponsesToChatCompletionsRequest(&ResponsesRequest{
		Model: "glm-5.2",
		Input: json.RawMessage(`"hi"`),
		Tools: []ResponsesTool{
			{Type: "tool_search", Execution: "client"},
			{Type: "function", Name: "tool_search"},
		},
	})
	require.Error(t, err, "与内置 tool_search 代理撞名的 function 工具必须拒绝")
	assert.Contains(t, err.Error(), "tool_search")

	// 与顶层 custom 工具同名。
	_, err = ResponsesToChatCompletionsRequest(&ResponsesRequest{
		Model: "glm-5.2",
		Input: json.RawMessage(`"hi"`),
		Tools: []ResponsesTool{
			{Type: "custom", Name: "tool_search"},
			{Type: "tool_search", Execution: "client"},
		},
	})
	require.Error(t, err, "与内置 tool_search 代理撞名的 custom 工具必须拒绝")

	// 重复声明 type=tool_search 去重后只产出一个代理，不拒绝。
	out, err := ResponsesToChatCompletionsRequest(&ResponsesRequest{
		Model: "glm-5.2",
		Input: json.RawMessage(`"hi"`),
		Tools: []ResponsesTool{{Type: "tool_search", Execution: "client"}, {Type: "tool_search", Execution: "client"}},
	})
	require.NoError(t, err)
	require.Len(t, out.Tools, 1)
	assert.Equal(t, "tool_search", out.Tools[0].Function.Name)

	_, err = ResponsesToChatCompletionsRequest(&ResponsesRequest{
		Model: "glm-5.2",
		Input: json.RawMessage(`"hi"`),
		Tools: []ResponsesTool{
			{Type: "tool_search", Execution: "client", Description: "find project tools"},
			{Type: "tool_search", Execution: "client", Description: "find tenant tools"},
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conflicting definitions")
}

// tool_choice 指向无法转换的托管工具或不存在的名字时必须显式拒绝；静默丢弃会把
// 强制调用退化成 auto。字符串形式与指向幸存工具的选择保持转发。
func TestResponsesToChatCompletionsRequest_RejectsUnrepresentableToolChoice(t *testing.T) {
	_, err := ResponsesToChatCompletionsRequest(&ResponsesRequest{
		Model: "glm-5.2",
		Input: json.RawMessage(`"hi"`),
		Tools: []ResponsesTool{
			{Type: "function", Name: "wait", Parameters: json.RawMessage(`{"type":"object","properties":{}}`)},
			{Type: "web_search"},
		},
		ToolChoice: json.RawMessage(`{"type":"web_search"}`),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "web_search")

	_, err = ResponsesToChatCompletionsRequest(&ResponsesRequest{
		Model:      "glm-5.2",
		Input:      json.RawMessage(`"hi"`),
		Tools:      []ResponsesTool{{Type: "function", Name: "wait"}},
		ToolChoice: json.RawMessage(`{"type":"function","name":"missing"}`),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing")

	// 字符串形式与指向幸存工具的选择保持原有转发行为。
	out, err := ResponsesToChatCompletionsRequest(&ResponsesRequest{
		Model:      "glm-5.2",
		Input:      json.RawMessage(`"hi"`),
		Tools:      []ResponsesTool{{Type: "function", Name: "wait"}},
		ToolChoice: json.RawMessage(`"auto"`),
	})
	require.NoError(t, err)
	assert.JSONEq(t, `"auto"`, string(out.ToolChoice))

	out, err = ResponsesToChatCompletionsRequest(&ResponsesRequest{
		Model:      "glm-5.2",
		Input:      json.RawMessage(`"hi"`),
		Tools:      []ResponsesTool{{Type: "function", Name: "wait"}},
		ToolChoice: json.RawMessage(`{"type":"function","name":"wait"}`),
	})
	require.NoError(t, err)
	assert.JSONEq(t, `{"type":"function","function":{"name":"wait"}}`, string(out.ToolChoice))
}

func TestResponsesToChatCompletionsRequest_MapsAllowedToolsChoice(t *testing.T) {
	out, err := ResponsesToChatCompletionsRequest(&ResponsesRequest{
		Model: "glm-5.2",
		Input: json.RawMessage(`"hi"`),
		Tools: []ResponsesTool{
			{Type: "function", Name: "wait"},
			{Type: "custom", Name: "exec"},
			{Type: "namespace", Name: "browser", Tools: []ResponsesTool{{Type: "function", Name: "open"}}},
		},
		ToolChoice: json.RawMessage(`{
			"type":"allowed_tools",
			"mode":"required",
			"tools":[
				{"type":"function","name":"wait"},
				{"type":"custom","name":"exec"},
				{"type":"namespace","name":"browser"}
			]
		}`),
	})
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"type":"allowed_tools",
		"allowed_tools":{
			"mode":"required",
			"tools":[
				{"type":"function","function":{"name":"wait"}},
				{"type":"function","function":{"name":"exec"}},
				{"type":"function","function":{"name":"browser__open"}}
			]
		}
	}`, string(out.ToolChoice))
}

func TestResponsesToChatCompletionsRequest_RejectsUnrepresentableAllowedToolsChoice(t *testing.T) {
	_, err := ResponsesToChatCompletionsRequest(&ResponsesRequest{
		Model: "glm-5.2",
		Input: json.RawMessage(`"hi"`),
		Tools: []ResponsesTool{{Type: "function", Name: "wait"}, {Type: "web_search"}},
		ToolChoice: json.RawMessage(`{
			"type":"allowed_tools",
			"mode":"required",
			"tools":[{"type":"function","name":"wait"},{"type":"web_search"}]
		}`),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "web_search")
}

// tool_search 工具没有被丢弃而是降级为同名 function 代理，强制选择它的 tool_choice
// 必须同步降级为指向代理的 function 选择，不能静默丢弃（丢弃会把强制搜索退化为
// 自动选择，模型可以不执行搜索）。
func TestResponsesToChatCompletionsRequest_ToolSearchToolChoiceMapsToProxy(t *testing.T) {
	out, err := ResponsesToChatCompletionsRequest(&ResponsesRequest{
		Model:      "glm-5.2",
		Input:      json.RawMessage(`"hi"`),
		Tools:      []ResponsesTool{{Type: "tool_search", Execution: "client"}},
		ToolChoice: json.RawMessage(`{"type":"tool_search"}`),
	})
	require.NoError(t, err)
	assert.JSONEq(t, `{"type":"function","function":{"name":"tool_search"}}`, string(out.ToolChoice))

	// 未声明 type=tool_search 时强制选择它没有可指向的代理，必须显式拒绝。
	_, err = ResponsesToChatCompletionsRequest(&ResponsesRequest{
		Model:      "glm-5.2",
		Input:      json.RawMessage(`"hi"`),
		Tools:      []ResponsesTool{{Type: "function", Name: "wait"}},
		ToolChoice: json.RawMessage(`{"type":"tool_search"}`),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tool_search")
}

func TestResponsesToChatCompletionsRequest_NamespaceToolChoiceMapsSingleChild(t *testing.T) {
	out, err := ResponsesToChatCompletionsRequest(&ResponsesRequest{
		Model: "glm-5.2",
		Input: json.RawMessage(`[
			{"role":"user","content":"take a screenshot"},
			{"type":"additional_tools","tools":[
				{"type":"namespace","name":"browser","tools":[{"type":"function","name":"screenshot"}]}
			]}
		]`),
		ToolChoice: json.RawMessage(`{"type":"namespace","name":"browser"}`),
	})
	require.NoError(t, err)
	assert.JSONEq(t, `{"type":"function","function":{"name":"browser__screenshot"}}`, string(out.ToolChoice))
}

func TestResponsesToChatCompletionsRequest_MapsMultiChildNamespaceToolChoice(t *testing.T) {
	out, err := ResponsesToChatCompletionsRequest(&ResponsesRequest{
		Model: "glm-5.2",
		Input: json.RawMessage(`"hi"`),
		Tools: []ResponsesTool{{
			Type: "namespace",
			Name: "browser",
			Tools: []ResponsesTool{
				{Type: "function", Name: "open"},
				{Type: "function", Name: "screenshot"},
			},
		}},
		ToolChoice: json.RawMessage(`{"type":"namespace","name":"browser"}`),
	})
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"type":"allowed_tools",
		"allowed_tools":{
			"mode":"required",
			"tools":[
				{"type":"function","function":{"name":"browser__open"}},
				{"type":"function","function":{"name":"browser__screenshot"}}
			]
		}
	}`, string(out.ToolChoice))
}

// 客户端请求在原生 Responses API 上合法（namespace 子工具按 namespace+name 路由），
// 是摊平转换让名字产生歧义；歧义无法消除时必须返回 transport capability mismatch
// 供 handler 换到原生 Responses 账号，而不是静默降级或归因成客户端请求错误。
func TestResponsesToChatCompletionsRequest_RejectsAmbiguousFlattenedNames(t *testing.T) {
	// 摊平名与顶层 function 工具撞名。
	_, err := ResponsesToChatCompletionsRequest(&ResponsesRequest{
		Model: "glm-5.2",
		Input: json.RawMessage(`"hi"`),
		Tools: []ResponsesTool{
			{Type: "function", Name: "gmail__send"},
			{Type: "namespace", Name: "gmail", Tools: []ResponsesTool{{Type: "function", Name: "send"}}},
		},
	})
	require.Error(t, err, "与顶层工具撞名的摊平必须拒绝")
	var capabilityErr *ChatCompletionsCapabilityError
	require.ErrorAs(t, err, &capabilityErr)
	assert.Equal(t, "chat_tool_identity", capabilityErr.Feature)
	assert.Contains(t, err.Error(), "gmail__send")

	// 不同 namespace 组合产生相同摊平名。
	_, err = ResponsesToChatCompletionsRequest(&ResponsesRequest{
		Model: "glm-5.2",
		Input: json.RawMessage(`"hi"`),
		Tools: []ResponsesTool{
			{Type: "namespace", Name: "a", Tools: []ResponsesTool{{Type: "function", Name: "b__c"}}},
			{Type: "namespace", Name: "a__b", Tools: []ResponsesTool{{Type: "function", Name: "c"}}},
		},
	})
	require.Error(t, err, "跨 namespace 撞名的摊平必须拒绝")
	require.ErrorAs(t, err, &capabilityErr)
	assert.Equal(t, "chat_tool_identity", capabilityErr.Feature)
	assert.Contains(t, err.Error(), "a__b__c")
}

func TestResponsesToChatCompletionsRequest_RejectsFunctionCustomNameConflict(t *testing.T) {
	_, err := ResponsesToChatCompletionsRequest(&ResponsesRequest{
		Model: "glm-5.2",
		Input: json.RawMessage(`"hi"`),
		Tools: []ResponsesTool{
			{Type: "function", Name: "exec"},
			{Type: "custom", Name: "exec"},
		},
	})
	require.Error(t, err)
	var capabilityErr *ChatCompletionsCapabilityError
	require.ErrorAs(t, err, &capabilityErr)
	assert.Equal(t, "chat_tool_identity", capabilityErr.Feature)
	assert.Contains(t, err.Error(), "function")
	assert.Contains(t, err.Error(), "custom")
	assert.Contains(t, err.Error(), "exec")
}

func TestResponsesToChatCompletionsRequest_RejectsToolChoiceSourceTypeMismatch(t *testing.T) {
	tests := []struct {
		name       string
		tools      []ResponsesTool
		toolChoice json.RawMessage
	}{
		{
			name:       "forced function cannot select custom",
			tools:      []ResponsesTool{{Type: "custom", Name: "exec"}},
			toolChoice: json.RawMessage(`{"type":"function","name":"exec"}`),
		},
		{
			name:       "forced custom cannot select function",
			tools:      []ResponsesTool{{Type: "function", Name: "wait"}},
			toolChoice: json.RawMessage(`{"type":"custom","name":"wait"}`),
		},
		{
			name:  "allowed custom cannot select function",
			tools: []ResponsesTool{{Type: "function", Name: "wait"}},
			toolChoice: json.RawMessage(`{
				"type":"allowed_tools",
				"mode":"required",
				"tools":[{"type":"custom","name":"wait"}]
			}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ResponsesToChatCompletionsRequest(&ResponsesRequest{
				Model:      "glm-5.2",
				Input:      json.RawMessage(`"hi"`),
				Tools:      tt.tools,
				ToolChoice: tt.toolChoice,
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "source type")
		})
	}
}

// 完全相同的 (namespace, 子工具) 重复声明不构成歧义：去重后正常转换，不拒绝。
func TestResponsesToChatCompletionsRequest_DedupesIdenticalNamespaceChildren(t *testing.T) {
	out, err := ResponsesToChatCompletionsRequest(&ResponsesRequest{
		Model: "glm-5.2",
		Input: json.RawMessage(`"hi"`),
		Tools: []ResponsesTool{
			{Type: "namespace", Name: "gmail", Tools: []ResponsesTool{
				{Type: "function", Name: "send"},
				{Type: "function", Name: "send"},
			}},
		},
	})
	require.NoError(t, err)
	require.Len(t, out.Tools, 1, "重复声明的同一子工具只声明一次")
	assert.Equal(t, "gmail__send", out.Tools[0].Function.Name)

	_, err = ResponsesToChatCompletionsRequest(&ResponsesRequest{
		Model: "glm-5.2",
		Input: json.RawMessage(`"hi"`),
		Tools: []ResponsesTool{
			{Type: "namespace", Name: "gmail", Tools: []ResponsesTool{{Type: "function", Name: "send", Parameters: json.RawMessage(`{"type":"object"}`)}}},
			{Type: "namespace", Name: "gmail", Tools: []ResponsesTool{{Type: "function", Name: "send", Parameters: json.RawMessage(`{"type":"string"}`)}}},
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conflicting definitions")
}

// codex 按 namespace+name 路由 namespace 子工具的调用：回程必须把摊平名还原为
// 裸子工具名并带独立 namespace 字段，平铺名的 function_call 会被 codex 判为
// unsupported call 拒绝执行。
func TestChatCompletionsResponseToResponses_NamespacedToolCallRestored(t *testing.T) {
	resp := &ChatCompletionsResponse{
		ID: "cc-1",
		Choices: []ChatChoice{{
			Message: ChatMessage{
				Role: "assistant",
				ToolCalls: []ChatToolCall{
					{ID: "call_n", Function: ChatFunctionCall{Name: "mcp__svc__echo", Arguments: `{"text":"hi"}`}},
					{ID: "call_9", Function: ChatFunctionCall{Name: "wait", Arguments: `{"cell_id": 3}`}},
				},
			},
		}},
	}
	nsTools := map[string]NamespacedToolName{
		"mcp__svc__echo": {Namespace: "mcp__svc", Name: "echo"},
	}

	out := ChatCompletionsResponseToResponses(resp, "glm-5.2", nil, false, nsTools)
	require.Len(t, out.Output, 2)

	item := out.Output[0]
	assert.Equal(t, "function_call", item.Type)
	assert.Equal(t, "echo", item.Name)
	assert.Equal(t, "mcp__svc", item.Namespace)
	assert.Equal(t, "call_n", item.CallID)
	assert.Equal(t, `{"text":"hi"}`, item.Arguments)

	// 非流式响应体走 ResponsesOutput.MarshalJSON，namespace 必须落到线上 JSON。
	b, err := json.Marshal(item)
	require.NoError(t, err)
	assert.Contains(t, string(b), `"namespace":"mcp__svc"`)
	assert.Contains(t, string(b), `"name":"echo"`)

	// 未命中映射的普通 function 调用不受影响，且不携带 namespace 字段。
	assert.Equal(t, "wait", out.Output[1].Name)
	assert.Empty(t, out.Output[1].Namespace)
	b2, err := json.Marshal(out.Output[1])
	require.NoError(t, err)
	assert.NotContains(t, string(b2), `"namespace"`)
}

func TestChatCompletionsChunkToResponsesEvents_NamespacedToolCallStream(t *testing.T) {
	state := NewChatCompletionsToResponsesStreamState("glm-5.2")
	state.NamespaceTools = map[string]NamespacedToolName{
		"mcp__svc__echo": {Namespace: "mcp__svc", Name: "echo"},
	}

	idx := 0
	chunk := &ChatCompletionsChunk{
		ID: "cc-1",
		Choices: []ChatChunkChoice{{
			Delta: ChatDelta{
				ToolCalls: []ChatToolCall{{
					Index:    &idx,
					ID:       "call_n",
					Function: ChatFunctionCall{Name: "mcp__svc__echo", Arguments: `{"text":"hi"}`},
				}},
			},
		}},
	}

	events := ChatCompletionsChunkToResponsesEvents(chunk, state)
	events = append(events, FinalizeChatCompletionsResponsesStream(state)...)

	var added, argumentsDelta, itemDone *ResponsesStreamEvent
	for i := range events {
		evt := &events[i]
		switch evt.Type {
		case "response.output_item.added":
			if evt.Item != nil && evt.Item.Type != "message" && evt.Item.Type != "reasoning" {
				added = evt
			}
		case "response.output_item.done":
			if evt.Item != nil && evt.Item.Type == "function_call" {
				itemDone = evt
			}
		case "response.function_call_arguments.delta":
			argumentsDelta = evt
		case "response.custom_tool_call_input.delta", "response.custom_tool_call_input.done":
			t.Fatalf("namespace 子工具调用不应产出 custom 事件: %s", evt.Type)
		}
	}

	require.NotNil(t, added, "缺少 namespace 调用的 output_item.added")
	assert.Equal(t, "function_call", added.Item.Type)
	assert.Equal(t, "echo", added.Item.Name)
	assert.Equal(t, "mcp__svc", added.Item.Namespace)

	require.NotNil(t, itemDone, "缺少 namespace 调用的 output_item.done")
	assert.Equal(t, "call_n", itemDone.Item.CallID)
	assert.Equal(t, "echo", itemDone.Item.Name)
	assert.Equal(t, "mcp__svc", itemDone.Item.Namespace)
	assert.Equal(t, `{"text":"hi"}`, itemDone.Item.Arguments)

	require.NotNil(t, argumentsDelta, "缺少 namespace 调用的 arguments delta")
	assert.Equal(t, "echo", argumentsDelta.Name, "同一调用的 delta 必须使用与 added/done 一致的还原名")

	// SSE 线上形态经 responsesItemWire 白名单重组，必须单独断言 namespace 落线。
	sse, err := ResponsesEventToSSE(*itemDone)
	require.NoError(t, err)
	assert.Contains(t, sse, `"namespace":"mcp__svc"`)
	assert.Contains(t, sse, `"name":"echo"`)
	assert.Contains(t, sse, `"call_id":"call_n"`)

	// response.completed 的 output 数组同样携带还原后的 namespace 调用项。
	final := events[len(events)-1]
	require.Equal(t, "response.completed", final.Type)
	require.NotNil(t, final.Response)
	found := false
	for _, item := range final.Response.Output {
		if item.Type == "function_call" {
			found = true
			assert.Equal(t, "echo", item.Name)
			assert.Equal(t, "mcp__svc", item.Namespace)
		}
	}
	assert.True(t, found, "response.completed 缺少还原后的 namespace 调用项")
}

func TestChatCompletionsChunkToResponsesEvents_NamespacedToolNameArrivesLate(t *testing.T) {
	state := NewChatCompletionsToResponsesStreamState("glm-5.2")
	state.NamespaceTools = map[string]NamespacedToolName{
		"mcp__svc__echo": {Namespace: "mcp__svc", Name: "echo"},
	}

	idx := 0
	chunk1 := &ChatCompletionsChunk{Choices: []ChatChunkChoice{{Delta: ChatDelta{
		ToolCalls: []ChatToolCall{{Index: &idx, ID: "call_n", Function: ChatFunctionCall{Arguments: `{"te`}}},
	}}}}
	chunk2 := &ChatCompletionsChunk{Choices: []ChatChunkChoice{{Delta: ChatDelta{
		ToolCalls: []ChatToolCall{{Index: &idx, Function: ChatFunctionCall{Name: "mcp__svc__echo", Arguments: `xt":"hi"}`}}},
	}}}}

	var events []ResponsesStreamEvent
	events = append(events, ChatCompletionsChunkToResponsesEvents(chunk1, state)...)
	events = append(events, ChatCompletionsChunkToResponsesEvents(chunk2, state)...)
	events = append(events, FinalizeChatCompletionsResponsesStream(state)...)

	addedCount := 0
	deltas := ""
	for _, evt := range events {
		switch evt.Type {
		case "response.output_item.added":
			if evt.Item != nil && evt.Item.Type != "reasoning" && evt.Item.Type != "message" {
				addedCount++
				assert.Equal(t, "echo", evt.Item.Name, "迟到的名字命中 namespace 映射时按还原名宣告")
				assert.Equal(t, "mcp__svc", evt.Item.Namespace)
			}
		case "response.function_call_arguments.delta":
			assert.Equal(t, "echo", evt.Name, "迟到宣告补发的 delta 必须使用还原名")
			deltas += evt.Delta
		}
	}
	assert.Equal(t, 1, addedCount, "工具调用只宣告一次")
	assert.Equal(t, `{"text":"hi"}`, deltas, "宣告前累积的参数需在宣告时补发")
}

func TestChatCompletionsChunkToResponsesEvents_FunctionToolStreamUnaffected(t *testing.T) {
	state := NewChatCompletionsToResponsesStreamState("glm-5.2")
	state.CustomTools = map[string]bool{"exec": true}

	idx := 0
	chunk := &ChatCompletionsChunk{
		Choices: []ChatChunkChoice{{
			Delta: ChatDelta{
				ToolCalls: []ChatToolCall{{
					Index:    &idx,
					ID:       "call_9",
					Function: ChatFunctionCall{Name: "wait", Arguments: `{"cell_id": 3}`},
				}},
			},
		}},
	}

	events := ChatCompletionsChunkToResponsesEvents(chunk, state)
	events = append(events, FinalizeChatCompletionsResponsesStream(state)...)

	sawArgsDelta := false
	for _, evt := range events {
		if evt.Type == "response.function_call_arguments.delta" {
			sawArgsDelta = true
		}
		if evt.Type == "response.custom_tool_call_input.done" {
			t.Fatal("function 工具不应产出 custom_tool_call 事件")
		}
	}
	assert.True(t, sawArgsDelta, "function 工具应保持原有参数增量事件")
}
