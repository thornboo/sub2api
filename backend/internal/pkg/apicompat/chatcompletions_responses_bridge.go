package apicompat

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

// ResponsesToChatCompletionsRequest converts a Responses API request into a
// Chat Completions request for upstreams that only implement
// /v1/chat/completions.
func ResponsesToChatCompletionsRequest(req *ResponsesRequest) (*ChatCompletionsRequest, error) {
	registry, err := BuildResponsesToolRegistry(req)
	if err != nil {
		return nil, err
	}
	return ResponsesToChatCompletionsRequestWithRegistry(req, registry, DefaultChatCompletionsCapabilities())
}

// ResponsesToChatCompletionsRequestWithRegistry converts a request using the
// already parsed request-local tool registry and explicit upstream capabilities.
func ResponsesToChatCompletionsRequestWithRegistry(req *ResponsesRequest, registry *ResponsesToolRegistry, capabilities ChatCompletionsCapabilities) (*ChatCompletionsRequest, error) {
	if req == nil {
		return nil, fmt.Errorf("responses request is nil")
	}
	if registry == nil {
		return nil, fmt.Errorf("responses tool registry is nil")
	}

	messages, err := responsesInputToChatMessagesWithRegistry(req.Instructions, registry, capabilities)
	if err != nil {
		return nil, err
	}
	requestTools := registry.Tools()

	out := &ChatCompletionsRequest{
		Model:               req.Model,
		Messages:            messages,
		MaxCompletionTokens: req.MaxOutputTokens,
		Temperature:         req.Temperature,
		TopP:                req.TopP,
		Stream:              req.Stream,
		ServiceTier:         req.ServiceTier,
		ParallelToolCalls:   req.ParallelToolCalls,
	}
	if req.Reasoning != nil {
		out.ReasoningEffort = req.Reasoning.Effort
	}
	if len(requestTools) > 0 {
		tools, err := responsesToolsToChatTools(requestTools, capabilities)
		if err != nil {
			return nil, err
		}
		out.Tools = tools
	}
	if len(req.ToolChoice) > 0 {
		declared := make(map[string]bool, len(out.Tools))
		for _, tool := range out.Tools {
			if tool.Function != nil {
				declared[tool.Function.Name] = true
			}
		}
		tc, err := responsesToolChoiceToChatToolChoice(req.ToolChoice, declared, responsesToolSourceTypes(requestTools), requestTools, capabilities)
		if err != nil {
			return nil, err
		}
		if len(tc) > 0 {
			if len(out.Tools) == 0 {
				return nil, fmt.Errorf("tool_choice cannot be forwarded because no Responses tools are representable by Chat Completions")
			}
			out.ToolChoice = tc
		}
	}
	if req.Text != nil {
		out.ResponseFormat = responsesTextFormatToChatResponseFormat(req.Text.Format)
	}

	return out, nil
}

// ResponsesRequestTools returns the tools callable at the end of the request's
// input history. BuildResponsesToolRegistry retains the source and loading state
// needed to keep top-level deferred tools hidden until additional_tools or a
// client tool_search_output makes them available.
func ResponsesRequestTools(req *ResponsesRequest) ([]ResponsesTool, error) {
	registry, err := BuildResponsesToolRegistry(req)
	if err != nil {
		return nil, err
	}
	return registry.Tools(), nil
}

// CustomToolNames 收集 Responses 请求中 custom/freeform 工具的名字。chat 桥回程时
// 需要据此把模型对这些工具的调用还原为 custom_tool_call 项（codex 只按该类型路由）。
func CustomToolNames(tools []ResponsesTool) map[string]bool {
	var out map[string]bool
	for _, tool := range tools {
		if tool.Type == "custom" && tool.Name != "" {
			if out == nil {
				out = make(map[string]bool)
			}
			out[tool.Name] = true
		}
	}
	return out
}

// NamespacedToolName 记录 namespace 子工具的原始归属（命名空间 + 裸子工具名）。
type NamespacedToolName struct {
	Namespace string
	Name      string
}

// NamespaceToolNames 收集 Responses 请求中 namespace 子工具的摊平名 →（namespace,
// 子工具名）映射。chat 桥回程时需据此把模型对摊平工具的调用还原为带 namespace 字段
// 的 function_call 项：codex 按 namespace+name 路由，平铺名会被判为 unsupported
// call；摊平名超长时带截断哈希（见 flattenNamespaceToolName），无法按字符串切分还原。
// 摊平名撞名的请求已在转换阶段被显式拒绝（见 namespaceChildrenToChatTools），
// 此处映射不存在歧义。
func NamespaceToolNames(tools []ResponsesTool) map[string]NamespacedToolName {
	var out map[string]NamespacedToolName
	for _, tool := range tools {
		if tool.Type != "namespace" || tool.Name == "" {
			continue
		}
		children := tool.Tools
		if len(children) == 0 {
			children = tool.Children
		}
		for _, child := range children {
			if child.Type != "function" || child.Name == "" {
				continue
			}
			if out == nil {
				out = make(map[string]NamespacedToolName)
			}
			out[flattenNamespaceToolName(tool.Name, child.Name)] = NamespacedToolName{
				Namespace: tool.Name,
				Name:      child.Name,
			}
		}
	}
	return out
}

// HasToolSearchTool reports whether the supplied callable set explicitly
// declares client-executed tool search. Type-only tool_search is hosted by
// protocol default and must not be silently reinterpreted as client execution.
func HasToolSearchTool(tools []ResponsesTool) bool {
	for _, tool := range tools {
		if tool.Type == "tool_search" && tool.Execution == "client" {
			return true
		}
	}
	return false
}

func responsesToolSourceTypes(tools []ResponsesTool) map[string]string {
	out := make(map[string]string)
	for _, tool := range tools {
		switch tool.Type {
		case "function", "custom":
			if tool.Name != "" {
				out[tool.Name] = tool.Type
			}
		case "tool_search":
			out[toolSearchProxyName] = "tool_search"
		case "namespace":
			children := tool.Tools
			if len(children) == 0 {
				children = tool.Children
			}
			for _, child := range children {
				if child.Type == "function" && child.Name != "" {
					out[flattenNamespaceToolName(tool.Name, child.Name)] = "namespace"
				}
			}
		}
	}
	return out
}

// responsesInputToChatMessages converts a Responses request's instructions +
// input[] into Chat Completions messages. It is a three-stage pipeline:
//
//	parse   — instructions become a system message; input[] is split into items
//	build   — buildChatMessagesFromItems walks items, attaching reasoning to the
//	          assistant message that produced a tool call, merging parallel tool
//	          calls into one assistant message, and skipping item types that have
//	          no Chat equivalent
//	normalize — normalizeChatMessages enforces the invariants DeepSeek requires
//
// The build + normalize split keeps every protocol rule in one place rather than
// scattered across per-item cases, and makes unknown future codex item types
// fail safe instead of leaking into the upstream request.
func responsesInputToChatMessages(instructions string, inputRaw json.RawMessage) ([]ChatMessage, error) {
	registry, err := BuildResponsesToolRegistry(&ResponsesRequest{Input: inputRaw})
	if err != nil {
		return nil, err
	}
	return responsesInputToChatMessagesWithRegistry(instructions, registry, DefaultChatCompletionsCapabilities())
}

func responsesInputToChatMessagesWithRegistry(instructions string, registry *ResponsesToolRegistry, capabilities ChatCompletionsCapabilities) ([]ChatMessage, error) {
	var messages []ChatMessage
	if strings.TrimSpace(instructions) != "" {
		content, _ := json.Marshal(instructions)
		messages = append(messages, ChatMessage{Role: "system", Content: content})
	}
	if registry == nil {
		return nil, fmt.Errorf("responses tool registry is nil")
	}
	if registry.inputIsText {
		content, _ := json.Marshal(registry.inputText)
		messages = append(messages, ChatMessage{Role: "user", Content: content})
		return messages, nil
	}
	if len(registry.inputItems) == 0 {
		return messages, nil
	}

	built, err := buildChatMessagesFromItems(messages, registry.inputItems, registry, capabilities)
	if err != nil {
		return nil, err
	}
	return normalizeChatMessages(built), nil
}

// buildChatMessagesFromItems walks the Responses input items and appends the
// corresponding Chat messages.
func buildChatMessagesFromItems(messages []ChatMessage, rawItems []json.RawMessage, registry *ResponsesToolRegistry, capabilities ChatCompletionsCapabilities) ([]ChatMessage, error) {
	// pendingReasoning holds the reasoning text from a reasoning item until the
	// assistant message it belongs to is emitted. DeepSeek's thinking mode
	// requires the reasoning_content that produced a tool call to be passed back
	// on that assistant message; dropping it yields a 400. It only survives
	// across an assistant message (so a following tool call in the same turn
	// still receives it); any other role ends the thinking span.
	var pendingReasoning string

	for inputItemIndex, raw := range rawItems {
		raw = bytesTrimSpace(raw)
		if len(raw) == 0 || string(raw) == "null" {
			continue
		}

		var item map[string]json.RawMessage
		if err := json.Unmarshal(raw, &item); err != nil {
			var text string
			if textErr := json.Unmarshal(raw, &text); textErr == nil {
				content, _ := json.Marshal(text)
				messages = append(messages, ChatMessage{Role: "user", Content: content})
				pendingReasoning = ""
				continue
			}
			return nil, fmt.Errorf("parse responses input item: %w", err)
		}

		role := chatCompletionsBridgeRole(rawString(item["role"]))
		itemType := rawString(item["type"])
		switch itemType {
		case "reasoning":
			if txt := extractResponsesReasoningText(item); txt != "" {
				pendingReasoning = txt
			}
			continue
		case "function_call":
			arguments := rawString(item["arguments"])
			if strings.TrimSpace(arguments) == "" {
				arguments = "{}"
			}
			name := rawString(item["name"])
			// namespace 子工具的历史调用带 namespace 字段，需与请求方向的摊平
			// 命名（namespaceChildrenToChatTools）保持一致。
			if ns := rawString(item["namespace"]); ns != "" {
				registeredName, ok, err := registry.chatNameForResponseToolAt(inputItemIndex, ns, name)
				if err != nil {
					return nil, err
				}
				if ok {
					name = registeredName
				} else {
					name = flattenNamespaceToolName(ns, name)
				}
			}
			toolCall := ChatToolCall{
				ID:   rawString(item["call_id"]),
				Type: "function",
				Function: ChatFunctionCall{
					Name:      name,
					Arguments: arguments,
				},
			}
			messages = appendAssistantToolCall(messages, toolCall, pendingReasoning)
			pendingReasoning = ""
			continue
		case "tool_search_call":
			execution := rawString(item["execution"])
			if err := validateClientToolSearchExecution(execution, capabilities); err != nil {
				return nil, err
			}
			callID := rawString(item["call_id"])
			if callID == "" {
				return nil, fmt.Errorf("client tool_search_call is missing call_id")
			}
			// tool_search 调用的 arguments 是 JSON 对象（如 {"query": ...}），
			// 原文即为降级 function 调用的 arguments 字符串。
			arguments := strings.TrimSpace(string(bytesTrimSpace(item["arguments"])))
			if s := rawString(item["arguments"]); s != "" {
				arguments = s
			}
			if arguments == "" || arguments == "null" {
				arguments = "{}"
			}
			toolCall := ChatToolCall{
				ID:   callID,
				Type: "function",
				Function: ChatFunctionCall{
					Name:      toolSearchProxyName,
					Arguments: arguments,
				},
			}
			messages = appendAssistantToolCall(messages, toolCall, pendingReasoning)
			pendingReasoning = ""
			continue
		case "custom_tool_call":
			// custom/freeform 工具的历史调用：input 自由文本包进降级 function 工具
			// 的 {"input": ...} 参数，与请求方向的工具降级（customToolInputSchema）
			// 保持一致，模型才能把历史与当前工具定义对上。
			arguments, _ := json.Marshal(map[string]string{"input": rawString(item["input"])})
			toolCall := ChatToolCall{
				ID:   rawString(item["call_id"]),
				Type: "function",
				Function: ChatFunctionCall{
					Name:      rawString(item["name"]),
					Arguments: string(arguments),
				},
			}
			messages = appendAssistantToolCall(messages, toolCall, pendingReasoning)
			pendingReasoning = ""
			continue
		case "tool_search_output":
			execution := rawString(item["execution"])
			if err := validateClientToolSearchExecution(execution, capabilities); err != nil {
				return nil, err
			}
			callID := rawString(item["call_id"])
			if callID == "" {
				return nil, fmt.Errorf("tool_search_output without call_id cannot be represented as a Chat Completions tool result")
			}
			toolsRaw := bytesTrimSpace(item["tools"])
			if len(toolsRaw) == 0 || string(toolsRaw) == "null" {
				return nil, fmt.Errorf("tool_search_output %q is missing tools", callID)
			}
			// A later copy with the same call_id updates the current tool registry,
			// but it is not a second Chat tool result for the original call. Keep the
			// first result in history and suppress repeated copies so Chat receives a
			// valid one-call/one-result sequence.
			if registry.isRepeatedToolSearchOutputItem(inputItemIndex) {
				continue
			}
			// Chat Completions requires a tool-role result for every preceding
			// assistant tool call. Preserve the exact loaded tool definitions as
			// JSON text while ResponsesRequestTools exposes them as callable tools.
			content, _ := json.Marshal(string(toolsRaw))
			messages = append(messages, ChatMessage{
				Role:       "tool",
				ToolCallID: callID,
				Content:    content,
			})
			pendingReasoning = ""
			continue
		case "function_call_output", "custom_tool_call_output":
			outputRaw := bytesTrimSpace(item["output"])
			outputText := rawString(outputRaw)
			if outputText == "" && len(outputRaw) > 0 && string(outputRaw) != "null" && string(outputRaw) != `""` {
				// 对象/数组形式的工具输出整体字符串化。
				outputText = string(outputRaw)
			}
			content, _ := json.Marshal(outputText)
			messages = append(messages, ChatMessage{
				Role:       "tool",
				ToolCallID: rawString(item["call_id"]),
				Content:    content,
			})
			pendingReasoning = ""
			continue
		case "input_text", "text":
			content, _ := json.Marshal(rawString(item["text"]))
			messages = append(messages, ChatMessage{Role: "user", Content: content})
			pendingReasoning = ""
			continue
		case "input_image":
			content, err := chatContentFromResponsesPart(responsesBridgeContentPart{
				Type:     itemType,
				Text:     rawString(item["text"]),
				ImageURL: item["image_url"],
			})
			if err != nil {
				return nil, err
			}
			messages = append(messages, ChatMessage{Role: "user", Content: content})
			pendingReasoning = ""
			continue
		}

		// Only genuine message items become chat messages. Codex emits other
		// Responses item types with no Chat equivalent (web_search_call,
		// local_shell_call, file_search_call, ...). Converting them via the
		// generic path would insert a spurious message between an assistant
		// tool_calls message and its tool reply, which DeepSeek rejects
		// ("insufficient tool messages following tool_calls message"). Skip them.
		if itemType != "" && itemType != "message" {
			pendingReasoning = ""
			continue
		}

		content := item["content"]
		if len(bytesTrimSpace(content)) == 0 {
			if text := rawString(item["text"]); text != "" {
				content, _ = json.Marshal(text)
			}
		}
		chatContent, err := responsesContentToChatContent(content, role)
		if err != nil {
			return nil, err
		}
		messages = append(messages, ChatMessage{Role: role, Content: chatContent})
		// Reasoning only survives across an assistant text message.
		if role != "assistant" {
			pendingReasoning = ""
		}
	}

	return messages, nil
}

// appendAssistantToolCall merges a tool call into the chat message list.
// Parallel tool calls arrive as consecutive *_call items and must share one
// assistant message; the matching tool replies then follow it. Merge into the
// immediately preceding assistant message.
func appendAssistantToolCall(messages []ChatMessage, toolCall ChatToolCall, pendingReasoning string) []ChatMessage {
	if n := len(messages); n > 0 && messages[n-1].Role == "assistant" {
		messages[n-1].ToolCalls = append(messages[n-1].ToolCalls, toolCall)
		if messages[n-1].ReasoningContent == "" {
			messages[n-1].ReasoningContent = pendingReasoning
		}
		return messages
	}
	return append(messages, ChatMessage{
		Role:             "assistant",
		ToolCalls:        []ChatToolCall{toolCall},
		ReasoningContent: pendingReasoning,
	})
}

// normalizeChatMessages is the single place that enforces the tool-call
// invariant the DeepSeek / OpenAI Chat Completions schema requires: an assistant
// message with tool_calls must be immediately followed by one tool message per
// tool_call_id, in order, with nothing in between.
//
// Codex histories violate this in several ways that the builder alone can't fix:
//   - a non-tool message lands between an assistant tool_calls message and its
//     tool replies (e.g. an "Approved command prefix saved" system notice codex
//     injects mid tool-execution);
//   - a parallel tool_call's sibling output never arrives, or a call is left
//     dangling by a mid-execution reconnect (unanswered tool_call);
//   - a tool reply has no announcing assistant tool_call (orphan).
//
// It rebuilds the sequence so each assistant's answered tool_calls are followed
// directly by their replies (in call order); unanswered tool_calls are dropped
// (and an assistant left with neither tool_calls nor content is dropped); orphan
// tool replies and intervening messages are emitted in their natural position
// but never between an assistant tool_calls message and its replies.
func normalizeChatMessages(messages []ChatMessage) []ChatMessage {
	// Index every tool reply by its tool_call_id (last wins on duplicates).
	replies := make(map[string]ChatMessage)
	for _, m := range messages {
		if m.Role == "tool" && m.ToolCallID != "" {
			replies[m.ToolCallID] = m
		}
	}

	out := make([]ChatMessage, 0, len(messages))
	for _, m := range messages {
		switch {
		case m.Role == "tool":
			// A bare tool message with no tool_call_id is a direct Chat
			// Completions passthrough; keep it in place. A tool reply whose id is
			// announced by an assistant is emitted right after that assistant
			// (skip the standalone occurrence). Any other tool reply is an orphan
			// and is dropped.
			if m.ToolCallID == "" {
				out = append(out, m)
			}
			continue
		case len(m.ToolCalls) > 0:
			kept := make([]ChatToolCall, 0, len(m.ToolCalls))
			for _, tc := range m.ToolCalls {
				if tc.ID == "" {
					continue
				}
				if _, ok := replies[tc.ID]; ok {
					kept = append(kept, tc)
				}
			}
			if len(kept) == 0 {
				// No answered tool_calls left: keep as a plain message if it has
				// content, otherwise drop it entirely.
				if isBlankChatContent(m.Content) {
					continue
				}
				m.ToolCalls = nil
				out = append(out, m)
				continue
			}
			m.ToolCalls = kept
			out = append(out, m)
			for _, tc := range kept {
				out = append(out, replies[tc.ID])
			}
		default:
			out = append(out, m)
		}
	}
	return out
}

// isBlankChatContent reports whether a chat message content holds no usable text.
func isBlankChatContent(raw json.RawMessage) bool {
	raw = bytesTrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" || string(raw) == `""` {
		return true
	}
	return chatMessageContentText(raw) == ""
}

// extractResponsesReasoningText pulls the reasoning text out of a Responses
// reasoning item. The Chat→Responses bridge writes the upstream reasoning_content
// verbatim into the summary_text parts (see closeChatReasoningItem), so codex
// round-trips it there; prefer summary[].text and fall back to content.
func extractResponsesReasoningText(item map[string]json.RawMessage) string {
	type reasoningPart struct {
		Text string `json:"text"`
	}
	var parts []string
	collect := func(raw json.RawMessage) {
		raw = bytesTrimSpace(raw)
		if len(raw) == 0 || string(raw) == "null" {
			return
		}
		var arr []reasoningPart
		if err := json.Unmarshal(raw, &arr); err == nil {
			for _, p := range arr {
				if p.Text != "" {
					parts = append(parts, p.Text)
				}
			}
			return
		}
		if t := rawString(raw); t != "" {
			parts = append(parts, t)
		}
	}
	collect(item["summary"])
	if len(parts) == 0 {
		collect(item["content"])
	}
	return strings.Join(parts, "\n")
}

func chatCompletionsBridgeRole(role string) string {
	trimmed := strings.TrimSpace(role)
	if trimmed == "" {
		return "user"
	}
	if strings.EqualFold(trimmed, "developer") {
		return "system"
	}
	return role
}

func responsesContentToChatContent(raw json.RawMessage, role string) (json.RawMessage, error) {
	raw = bytesTrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		empty, _ := json.Marshal("")
		return empty, nil
	}

	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return raw, nil
	}

	var rawParts []json.RawMessage
	if err := json.Unmarshal(raw, &rawParts); err == nil {
		return responsesContentPartsToChatContent(rawParts, role)
	}

	var part responsesBridgeContentPart
	if err := json.Unmarshal(raw, &part); err == nil {
		return chatContentFromResponsesPart(part)
	}

	return raw, nil
}

func responsesContentPartsToChatContent(rawParts []json.RawMessage, role string) (json.RawMessage, error) {
	var textParts []string
	var chatParts []ChatContentPart
	hasNonText := false

	for _, rawPart := range rawParts {
		var part responsesBridgeContentPart
		if err := json.Unmarshal(rawPart, &part); err != nil {
			continue
		}
		switch part.Type {
		case "input_text", "output_text", "text", "":
			if part.Text == "" {
				continue
			}
			textParts = append(textParts, part.Text)
			chatParts = append(chatParts, ChatContentPart{Type: "text", Text: part.Text})
		case "input_image", "image_url":
			imageURL := responseBridgeImageURL(part.ImageURL)
			if imageURL == "" {
				continue
			}
			hasNonText = true
			chatParts = append(chatParts, ChatContentPart{
				Type:     "image_url",
				ImageURL: &ChatImageURL{URL: imageURL},
			})
		}
	}

	if !hasNonText {
		joined, _ := json.Marshal(strings.Join(textParts, "\n\n"))
		return joined, nil
	}
	if role != "user" {
		joined, _ := json.Marshal(strings.Join(textParts, "\n\n"))
		return joined, nil
	}
	if len(chatParts) == 0 {
		empty, _ := json.Marshal("")
		return empty, nil
	}
	return json.Marshal(chatParts)
}

type responsesBridgeContentPart struct {
	Type     string          `json:"type"`
	Text     string          `json:"text"`
	ImageURL json.RawMessage `json:"image_url"`
}

func chatContentFromResponsesPart(part responsesBridgeContentPart) (json.RawMessage, error) {
	switch part.Type {
	case "input_image", "image_url":
		return json.Marshal([]ChatContentPart{{
			Type:     "image_url",
			ImageURL: &ChatImageURL{URL: responseBridgeImageURL(part.ImageURL)},
		}})
	default:
		return json.Marshal(part.Text)
	}
}

func responseBridgeImageURL(raw json.RawMessage) string {
	if imageURL := rawString(raw); imageURL != "" {
		return imageURL
	}
	return rawNestedString(raw, "url")
}

// customToolInputSchema 是 custom/freeform 工具降级为 function 工具时的参数 schema。
// chat 协议无法表达 custom 工具的自由文本输入（及其 grammar 约束），退化为单一
// input 字符串参数；回程时再从 arguments 的 input 字段还原（见
// extractCustomToolCallInput）。
const customToolInputSchema = `{"type":"object","properties":{"input":{"type":"string","description":"The raw input for this tool, passed through verbatim."}},"required":["input"]}`

func responsesToolsToChatTools(tools []ResponsesTool, capabilities ChatCompletionsCapabilities) ([]ChatTool, error) {
	// 顶层 function/custom 工具名集合：namespace 子工具摊平后与其撞名时，chat
	// 上游无法按 namespace 区分调用归属。这类请求在原生 Responses 上游是合法的
	// （按 namespace+name 路由），歧义由摊平转换制造且无法消除，必须显式拒绝，
	// 不能静默降级（重复声明发给上游、回程还原到错误工具）。
	topLevel := make(map[string]string)
	for _, tool := range tools {
		if (tool.Type == "function" || tool.Type == "custom") && tool.Name != "" {
			if previousType, exists := topLevel[tool.Name]; exists && previousType != tool.Type {
				return nil, newChatCompletionsCapabilityError("chat_tool_identity", fmt.Sprintf("declared function and custom tools share the name %q; this upstream cannot disambiguate their response type, rename one of the tools", tool.Name))
			}
			topLevel[tool.Name] = tool.Type
		}
	}
	flatOwner := make(map[string]NamespacedToolName)
	flatDefinitions := make(map[string]ResponsesTool)
	convertedTopLevel := make(map[string]ResponsesTool)
	toolSearchDeclared := false
	var toolSearchDefinition ResponsesTool
	out := make([]ChatTool, 0, len(tools))
	for _, tool := range tools {
		switch tool.Type {
		case "function":
			if previous, exists := convertedTopLevel[tool.Name]; exists {
				if responsesToolDefinitionsEqual(previous, tool) {
					continue
				}
				return nil, fmt.Errorf("function tool %q has conflicting definitions across Responses tool carriers", tool.Name)
			}
			convertedTopLevel[tool.Name] = tool
			out = append(out, ChatTool{
				Type: "function",
				Function: &ChatFunction{
					Name:        tool.Name,
					Description: tool.Description,
					Parameters:  tool.Parameters,
					Strict:      tool.Strict,
				},
			})
		case "custom":
			if len(bytesTrimSpace(tool.Format)) > 0 && !capabilities.AllowLossyCustomToolGrammar {
				return nil, newChatCompletionsCapabilityError("chat_custom_tool_grammar", fmt.Sprintf("custom tool %q uses a grammar/format that cannot be preserved by this Chat Completions fallback", tool.Name))
			}
			if previous, exists := convertedTopLevel[tool.Name]; exists {
				if responsesToolDefinitionsEqual(previous, tool) {
					continue
				}
				return nil, fmt.Errorf("custom tool %q has conflicting definitions across Responses tool carriers", tool.Name)
			}
			convertedTopLevel[tool.Name] = tool
			// codex 0.14x 的核心执行工具 exec 即为 custom 类型；丢弃它会让模型
			// 无法执行任何命令，必须降级为 function 工具透传。
			out = append(out, ChatTool{
				Type: "function",
				Function: &ChatFunction{
					Name:        tool.Name,
					Description: tool.Description,
					Parameters:  json.RawMessage(customToolInputSchema),
				},
			})
		case "tool_search":
			if err := validateClientToolSearchExecution(tool.Execution, capabilities); err != nil {
				return nil, err
			}
			// 代理不能改名（codex 的模型侧按 tool_search 这个名字调用），与客户端
			// 声明的同名工具无法区分——回程会把普通工具的调用劫持成 tool_search_call，
			// 必须显式拒绝；重复声明 type=tool_search 去重即可。
			if _, exists := topLevel[toolSearchProxyName]; exists {
				return nil, newChatCompletionsCapabilityError("chat_tool_identity", fmt.Sprintf("built-in tool_search conflicts with a declared tool named %q; this upstream cannot disambiguate them, rename the tool", toolSearchProxyName))
			}
			if toolSearchDeclared {
				if responsesToolDefinitionsEqual(toolSearchDefinition, tool) {
					continue
				}
				return nil, fmt.Errorf("tool_search has conflicting definitions across Responses tool carriers")
			}
			toolSearchDeclared = true
			toolSearchDefinition = tool
			out = append(out, toolSearchProxyChatTool(tool))
		case "namespace":
			flattened, err := namespaceChildrenToChatTools(tool, topLevel, flatOwner, flatDefinitions)
			if err != nil {
				return nil, err
			}
			out = append(out, flattened...)
		default:
			if tool.Type == "" {
				return nil, fmt.Errorf("responses tool type is required")
			}
			return nil, newChatCompletionsCapabilityError("responses_hosted_tool", fmt.Sprintf("Responses tool type %q cannot be represented by a Chat Completions upstream", tool.Type))
		}
	}
	return out, nil
}

// toolSearchProxyName 是 tool_search 服务端工具降级后的 function 工具名。模型对
// 它的调用以同名 function_call 原样回传，由 codex 端路由。
const toolSearchProxyName = "tool_search"

const toolSearchProxySchema = `{"type":"object","properties":{"query":{"type":"string","description":"Search query for tools or connectors to load."},"limit":{"type":"integer","description":"Maximum number of tool groups to return."}},"required":["query"]}`

func toolSearchProxyChatTool(tool ResponsesTool) ChatTool {
	description := tool.Description
	if description == "" {
		description = "Search and load Codex tools, plugins, connectors, and MCP namespaces for the current task."
	}
	parameters := tool.Parameters
	if len(bytesTrimSpace(parameters)) == 0 {
		parameters = json.RawMessage(toolSearchProxySchema)
	}
	return ChatTool{
		Type: "function",
		Function: &ChatFunction{
			Name:        toolSearchProxyName,
			Description: description,
			Parameters:  parameters,
		},
	}
}

// namespaceChildrenToChatTools 将 namespace 工具的子 function 工具摊平为顶层
// function 工具，名字加 "<namespace>__" 前缀。摊平名与顶层工具或其他 namespace
// 撞名时返回错误（歧义不可消除，显式拒绝）；同一 (namespace, 子工具) 的重复声明
// 去重后不算冲突。
func namespaceChildrenToChatTools(tool ResponsesTool, topLevel map[string]string, flatOwner map[string]NamespacedToolName, flatDefinitions map[string]ResponsesTool) ([]ChatTool, error) {
	if tool.Name == "" {
		return nil, nil
	}
	children := tool.Tools
	if len(children) == 0 {
		children = tool.Children
	}
	var out []ChatTool
	for _, child := range children {
		if child.Type != "function" || child.Name == "" {
			continue
		}
		flat := flattenNamespaceToolName(tool.Name, child.Name)
		entry := NamespacedToolName{Namespace: tool.Name, Name: child.Name}
		if _, exists := topLevel[flat]; exists {
			return nil, newChatCompletionsCapabilityError("chat_tool_identity", fmt.Sprintf("namespace tool %q/%q flattens to %q which conflicts with a top-level tool of the same name; this upstream cannot disambiguate them, rename one of the tools", tool.Name, child.Name, flat))
		}
		if prev, ok := flatOwner[flat]; ok {
			if prev == entry {
				if responsesToolDefinitionsEqual(flatDefinitions[flat], child) {
					continue
				}
				return nil, fmt.Errorf("namespace tool %q/%q has conflicting definitions across Responses tool carriers", tool.Name, child.Name)
			}
			return nil, newChatCompletionsCapabilityError("chat_tool_identity", fmt.Sprintf("namespace tools %q/%q and %q/%q both flatten to %q; this upstream cannot disambiguate them, rename one of the tools", prev.Namespace, prev.Name, tool.Name, child.Name, flat))
		}
		flatOwner[flat] = entry
		flatDefinitions[flat] = child
		out = append(out, ChatTool{
			Type: "function",
			Function: &ChatFunction{
				Name:        flat,
				Description: child.Description,
				Parameters:  child.Parameters,
				Strict:      child.Strict,
			},
		})
	}
	return out, nil
}

func responsesToolDefinitionsEqual(left, right ResponsesTool) bool {
	leftJSON, leftErr := responsesToolDefinitionJSON(left)
	rightJSON, rightErr := responsesToolDefinitionJSON(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	leftValue, leftErr := decodeToolDefinitionForEquality(leftJSON)
	rightValue, rightErr := decodeToolDefinitionForEquality(rightJSON)
	if leftErr != nil || rightErr != nil {
		return false
	}
	return reflect.DeepEqual(leftValue, rightValue)
}

func decodeToolDefinitionForEquality(raw []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return value, nil
}

func responsesToolDefinitionJSON(tool ResponsesTool) ([]byte, error) {
	if raw := bytesTrimSpace(tool.rawDefinition); len(raw) > 0 {
		return raw, nil
	}
	return json.Marshal(tool)
}

// chatToolNameMaxLen 是 Chat Completions function 工具名的通用长度上限。
const chatToolNameMaxLen = 64

// flattenNamespaceToolName 生成 namespace 子工具的摊平名；超长时截断并追加
// sha256 短哈希保证唯一性。
func flattenNamespaceToolName(namespace, name string) string {
	full := namespace + "__" + name
	if len(full) <= chatToolNameMaxLen {
		return full
	}
	sum := sha256.Sum256([]byte(full))
	suffix := "__" + hex.EncodeToString(sum[:4])
	prefixLen := chatToolNameMaxLen - len(suffix)
	var prefix strings.Builder
	for _, ch := range full {
		if prefix.Len()+len(string(ch)) > prefixLen {
			break
		}
		_, _ = prefix.WriteRune(ch)
	}
	return prefix.String() + suffix
}

// responsesToolChoiceToChatToolChoice 把 Responses 的 tool_choice 转为 chat 形态。
// declared 是转换后实际声明的 chat 工具名集合。强制选择无法精确表示时必须返回错误，
// 不能静默丢弃后退化为 auto；只有在没有可转换工具时，auto/none 可以安全省略。
func responsesToolChoiceToChatToolChoice(raw json.RawMessage, declared map[string]bool, sourceTypes map[string]string, tools []ResponsesTool, capabilities ChatCompletionsCapabilities) (json.RawMessage, error) {
	raw = bytesTrimSpace(raw)
	var mode string
	if err := json.Unmarshal(raw, &mode); err == nil {
		switch mode {
		case "auto", "none":
			if len(declared) == 0 {
				return nil, nil
			}
			return raw, nil
		case "required":
			if len(declared) == 0 {
				return nil, fmt.Errorf("tool_choice %q requires a tool, but no Responses tools are representable by Chat Completions", mode)
			}
			return raw, nil
		default:
			return nil, fmt.Errorf("unsupported Responses tool_choice %q", mode)
		}
	}

	var choice map[string]json.RawMessage
	if err := json.Unmarshal(raw, &choice); err != nil {
		return nil, fmt.Errorf("parse Responses tool_choice: %w", err)
	}
	choiceType := rawString(choice["type"])
	if choiceType == "allowed_tools" {
		return responsesAllowedToolsChoiceToChat(choice, declared, sourceTypes, tools, capabilities)
	}

	var name string
	var requiredSourceType string
	switch choiceType {
	case "tool_search":
		// tool_search 未被丢弃而是降级为同名 function 代理（见
		// responsesToolsToChatTools），强制选择它同样降级为 function 选择，
		// 静默丢弃会把强制搜索退化为自动选择。
		name = toolSearchProxyName
		requiredSourceType = "tool_search"
	case "function", "custom":
		// custom 工具已降级为 function 工具，指向它的 tool_choice 同样按 function 转换。
		name = rawString(choice["name"])
		if name == "" {
			name = rawNestedString(choice["function"], "name")
		}
		if name == "" {
			return nil, fmt.Errorf("%s tool_choice is missing name", choiceType)
		}
		requiredSourceType = choiceType
	case "namespace":
		namespace := rawString(choice["name"])
		if namespace == "" {
			namespace = rawString(choice["namespace"])
		}
		if namespace == "" {
			return nil, fmt.Errorf("namespace tool_choice is missing name")
		}
		targets := namespaceToolChoiceTargets(tools, namespace, declared)
		switch len(targets) {
		case 0:
			return nil, fmt.Errorf("namespace tool_choice %q has no convertible child tools", namespace)
		case 1:
			name = targets[0]
		default:
			return chatAllowedToolsChoice("required", targets, capabilities)
		}
	default:
		if choiceType == "" {
			return nil, fmt.Errorf("responses tool_choice object is missing type")
		}
		return nil, fmt.Errorf("responses tool_choice type %q cannot be represented by Chat Completions", choiceType)
	}
	if !declared[name] {
		return nil, fmt.Errorf("responses tool_choice %q does not reference a tool representable by Chat Completions", name)
	}
	if requiredSourceType != "" && sourceTypes[name] != requiredSourceType {
		return nil, fmt.Errorf("responses %s tool_choice %q does not match declared source type %q", choiceType, name, sourceTypes[name])
	}
	out, err := json.Marshal(map[string]any{
		"type": "function",
		"function": map[string]string{
			"name": name,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal Chat Completions tool_choice: %w", err)
	}
	return out, nil
}

type responsesAllowedToolRef struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

func responsesAllowedToolsChoiceToChat(choice map[string]json.RawMessage, declared map[string]bool, sourceTypes map[string]string, requestTools []ResponsesTool, capabilities ChatCompletionsCapabilities) (json.RawMessage, error) {
	mode := rawString(choice["mode"])
	if mode != "auto" && mode != "required" {
		return nil, fmt.Errorf("allowed_tools tool_choice mode must be auto or required")
	}
	allowedRaw := bytesTrimSpace(choice["tools"])
	if len(allowedRaw) > maxResponsesToolTotalBytes {
		return nil, fmt.Errorf("allowed_tools definitions exceed %d bytes", maxResponsesToolTotalBytes)
	}
	seen := make(map[string]bool)
	chatAllowedNames := make([]string, 0)
	appendName := func(name string) error {
		if name == "" || !declared[name] {
			return fmt.Errorf("allowed_tools tool_choice %q does not reference a tool representable by Chat Completions", name)
		}
		if seen[name] {
			return nil
		}
		seen[name] = true
		chatAllowedNames = append(chatAllowedNames, name)
		return nil
	}

	appendTool := func(tool responsesAllowedToolRef) error {
		switch tool.Type {
		case "function", "custom":
			if sourceTypes[tool.Name] != tool.Type {
				return fmt.Errorf("allowed_tools entry %q of type %q does not match declared source type %q", tool.Name, tool.Type, sourceTypes[tool.Name])
			}
			if err := appendName(tool.Name); err != nil {
				return err
			}
		case "tool_search":
			if sourceTypes[toolSearchProxyName] != "tool_search" {
				return fmt.Errorf("allowed_tools tool_search entry does not match a declared source tool")
			}
			if err := appendName(toolSearchProxyName); err != nil {
				return err
			}
		case "namespace":
			targets := namespaceToolChoiceTargets(requestTools, tool.Name, declared)
			if len(targets) == 0 {
				return fmt.Errorf("allowed_tools namespace %q has no convertible child tools", tool.Name)
			}
			for _, target := range targets {
				if err := appendName(target); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("allowed_tools entry type %q cannot be represented by Chat Completions", tool.Type)
		}
		return nil
	}

	decoder := json.NewDecoder(bytes.NewReader(allowedRaw))
	token, err := decoder.Token()
	if err != nil {
		return nil, fmt.Errorf("parse allowed_tools tool_choice tools: %w", err)
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '[' {
		return nil, fmt.Errorf("allowed_tools tool_choice tools must be an array")
	}
	allowedCount := 0
	for decoder.More() {
		allowedCount++
		if allowedCount > maxResponsesToolCount {
			return nil, fmt.Errorf("allowed_tools count exceeds %d", maxResponsesToolCount)
		}
		var tool responsesAllowedToolRef
		if err := decoder.Decode(&tool); err != nil {
			return nil, fmt.Errorf("parse allowed_tools tool_choice tool: %w", err)
		}
		if err := appendTool(tool); err != nil {
			return nil, err
		}
	}
	if _, err := decoder.Token(); err != nil {
		return nil, fmt.Errorf("parse allowed_tools tool_choice tools: %w", err)
	}
	if allowedCount == 0 {
		return nil, fmt.Errorf("allowed_tools tool_choice must contain at least one tool")
	}

	return chatAllowedToolsChoice(mode, chatAllowedNames, capabilities)
}

func chatAllowedToolsChoice(mode string, names []string, capabilities ChatCompletionsCapabilities) (json.RawMessage, error) {
	if !capabilities.SupportsAllowedTools {
		return nil, newChatCompletionsCapabilityError("chat_allowed_tools", "this Chat Completions account has not declared support for tool_choice.allowed_tools")
	}
	chatAllowed := make([]map[string]any, 0, len(names))
	for _, name := range names {
		chatAllowed = append(chatAllowed, map[string]any{
			"type": "function",
			"function": map[string]string{
				"name": name,
			},
		})
	}
	out, err := json.Marshal(map[string]any{
		"type": "allowed_tools",
		"allowed_tools": map[string]any{
			"mode":  mode,
			"tools": chatAllowed,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal Chat Completions allowed_tools choice: %w", err)
	}
	return out, nil
}

func namespaceToolChoiceTargets(tools []ResponsesTool, namespace string, declared map[string]bool) []string {
	seen := make(map[string]bool)
	var out []string
	for _, tool := range tools {
		if tool.Type != "namespace" || tool.Name != namespace {
			continue
		}
		children := tool.Tools
		if len(children) == 0 {
			children = tool.Children
		}
		for _, child := range children {
			if child.Type != "function" || child.Name == "" {
				continue
			}
			flat := flattenNamespaceToolName(namespace, child.Name)
			if !declared[flat] || seen[flat] {
				continue
			}
			seen[flat] = true
			out = append(out, flat)
		}
	}
	return out
}

// extractCustomToolCallInput 从降级 function 调用的 arguments 中还原 custom 工具的
// 自由文本输入：优先取 {"input": "..."} 的 input 字段；模型未按 schema 输出时原样
// 回传，交由客户端校验、模型重试。
func extractCustomToolCallInput(arguments string) string {
	trimmed := strings.TrimSpace(arguments)
	if trimmed == "" {
		return ""
	}
	if !gjson.Valid(trimmed) {
		return trimmed
	}
	root := gjson.Parse(trimmed)
	if !root.IsObject() {
		if root.Type == gjson.Null {
			return ""
		}
		return trimmed
	}
	if input := root.Get("input"); input.Exists() {
		if input.Type == gjson.String {
			return input.String()
		}
		return trimmed
	}
	hasFields := false
	root.ForEach(func(_, _ gjson.Result) bool {
		hasFields = true
		return false
	})
	if !hasFields {
		return ""
	}
	return trimmed
}

// ChatCompletionsResponseToResponses converts a non-streaming Chat Completions
// response into a Responses API response. customTools 是客户端请求中 custom 工具
// 的名字集合（见 CustomToolNames），命中的调用会还原为 custom_tool_call 项；
// toolSearch 表示客户端声明了 tool_search 工具（见 HasToolSearchTool），代理工具
// 的调用会还原为 tool_search_call 项；namespaceTools 是 namespace 子工具的摊平名
// 映射（见 NamespaceToolNames），命中的调用还原为带 namespace 字段的 function_call 项。
func ChatCompletionsResponseToResponses(resp *ChatCompletionsResponse, model string, customTools map[string]bool, toolSearch bool, namespaceTools map[string]NamespacedToolName) *ResponsesResponse {
	id := ""
	if resp != nil {
		id = resp.ID
	}
	if id == "" {
		id = generateResponsesID()
	}

	out := &ResponsesResponse{
		ID:     id,
		Object: "response",
		Model:  model,
		Status: "completed",
	}
	if resp == nil {
		out.Output = []ResponsesOutput{emptyResponsesMessageOutput()}
		return out
	}
	if out.Model == "" {
		out.Model = resp.Model
	}

	if len(resp.Choices) > 0 {
		choice := resp.Choices[0]
		out.Output = chatMessageToResponsesOutput(choice.Message, customTools, toolSearch, namespaceTools)
		if choice.FinishReason == "length" {
			out.Status = "incomplete"
			out.IncompleteDetails = &ResponsesIncompleteDetails{Reason: "max_output_tokens"}
		}
	}
	if len(out.Output) == 0 {
		out.Output = []ResponsesOutput{emptyResponsesMessageOutput()}
	}
	if resp.Usage != nil {
		out.Usage = ChatUsageToResponsesUsage(resp.Usage)
	}
	return out
}

func chatMessageToResponsesOutput(message ChatMessage, customTools map[string]bool, toolSearch bool, namespaceTools map[string]NamespacedToolName) []ResponsesOutput {
	var outputs []ResponsesOutput
	if message.ReasoningContent != "" {
		outputs = append(outputs, ResponsesOutput{
			Type: "reasoning",
			ID:   generateItemID(),
			Summary: []ResponsesSummary{{
				Type: "summary_text",
				Text: message.ReasoningContent,
			}},
		})
	}

	text := chatMessageContentText(message.Content)
	if text == "" && strings.TrimSpace(message.ReasoningContent) != "" && len(message.ToolCalls) == 0 {
		text = message.ReasoningContent
	}
	if text != "" || len(message.ToolCalls) == 0 {
		outputs = append(outputs, ResponsesOutput{
			Type: "message",
			ID:   generateItemID(),
			Role: "assistant",
			Content: []ResponsesContentPart{{
				Type: "output_text",
				Text: text,
			}},
			Status: "completed",
		})
	}

	for _, toolCall := range message.ToolCalls {
		arguments := toolCall.Function.Arguments
		if strings.TrimSpace(arguments) == "" {
			arguments = "{}"
		}
		if customTools[toolCall.Function.Name] {
			outputs = append(outputs, ResponsesOutput{
				Type:   "custom_tool_call",
				ID:     generateItemID(),
				CallID: toolCall.ID,
				Name:   toolCall.Function.Name,
				Input:  extractCustomToolCallInput(arguments),
				Status: "completed",
			})
			continue
		}
		if toolSearch && toolCall.Function.Name == toolSearchProxyName {
			outputs = append(outputs, ResponsesOutput{
				Type:      "tool_search_call",
				ID:        generateItemID(),
				CallID:    toolCall.ID,
				Arguments: arguments,
				Status:    "completed",
			})
			continue
		}
		if ns, ok := namespaceTools[toolCall.Function.Name]; ok {
			outputs = append(outputs, ResponsesOutput{
				Type:      "function_call",
				ID:        generateItemID(),
				CallID:    toolCall.ID,
				Name:      ns.Name,
				Namespace: ns.Namespace,
				Arguments: arguments,
				Status:    "completed",
			})
			continue
		}
		outputs = append(outputs, ResponsesOutput{
			Type:      "function_call",
			ID:        generateItemID(),
			CallID:    toolCall.ID,
			Name:      toolCall.Function.Name,
			Arguments: arguments,
			Status:    "completed",
		})
	}

	return outputs
}

// toolSearchCallArgumentsJSON 把降级 function 调用累积的 arguments 字符串还原为
// tool_search_call 线上要求的 JSON 对象；模型未按 schema 输出（非法 JSON）时按
// 字符串值兜底，交由 codex 解析报错后让模型重试。
func toolSearchCallArgumentsJSON(arguments string) json.RawMessage {
	trimmed := strings.TrimSpace(arguments)
	if trimmed == "" {
		return json.RawMessage(`{}`)
	}
	if json.Valid([]byte(trimmed)) {
		return json.RawMessage(trimmed)
	}
	fallback, _ := json.Marshal(arguments)
	return fallback
}

func emptyResponsesMessageOutput() ResponsesOutput {
	return ResponsesOutput{
		Type:    "message",
		ID:      generateItemID(),
		Role:    "assistant",
		Content: []ResponsesContentPart{{Type: "output_text", Text: ""}},
		Status:  "completed",
	}
}

func chatMessageContentText(raw json.RawMessage) string {
	raw = bytesTrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	var parts []ChatContentPart
	if err := json.Unmarshal(raw, &parts); err == nil {
		var texts []string
		for _, part := range parts {
			if part.Type == "text" && part.Text != "" {
				texts = append(texts, part.Text)
			}
		}
		return strings.Join(texts, "\n\n")
	}
	return ""
}

// ChatUsageToResponsesUsage converts Chat Completions token usage to Responses
// usage shape.
func ChatUsageToResponsesUsage(usage *ChatUsage) *ResponsesUsage {
	if usage == nil {
		return nil
	}
	out := &ResponsesUsage{
		InputTokens:  usage.PromptTokens,
		OutputTokens: usage.CompletionTokens,
		TotalTokens:  usage.TotalTokens,
	}
	if out.TotalTokens == 0 {
		out.TotalTokens = out.InputTokens + out.OutputTokens
	}
	if usage.PromptTokensDetails != nil && (usage.PromptTokensDetails.CachedTokens > 0 ||
		usage.PromptTokensDetails.CacheCreationTokens > 0 || usage.PromptTokensDetails.CacheWriteTokens > 0) {
		out.InputTokensDetails = &ResponsesInputTokensDetails{
			CachedTokens:        usage.PromptTokensDetails.CachedTokens,
			CacheCreationTokens: usage.PromptTokensDetails.CacheCreationTokens,
			CacheWriteTokens:    usage.PromptTokensDetails.CacheWriteTokens,
		}
		if usage.PromptTokensDetails.CacheWriteTokens > 0 {
			out.CacheCreationInputTokens = usage.PromptTokensDetails.CacheWriteTokens
		} else {
			out.CacheCreationInputTokens = usage.PromptTokensDetails.CacheCreationTokens
		}
	}
	return out
}

// ChatCompletionsToResponsesStreamState tracks state while converting Chat
// Completions SSE chunks into Responses SSE events.
type ChatCompletionsToResponsesStreamState struct {
	ResponseID     string
	Model          string
	Created        int64
	SequenceNumber int
	CreatedSent    bool
	CompletedSent  bool

	// nextOutputIndex assigns sequential output_index values to items as they
	// are opened (reasoning, message, tool calls), so the streamed indices match
	// the order of items in the final response.output array.
	nextOutputIndex int

	// Reasoning item lifecycle. DeepSeek-style upstreams stream all
	// reasoning_content before any content, so reasoning is modeled as its own
	// "reasoning" output item that must be opened (output_item.added) before any
	// reasoning delta and closed before the message/tool items open.
	ReasoningItemID string
	ReasoningIndex  int
	ReasoningOpen   bool
	ReasoningDone   bool

	// Message item + output_text content-part lifecycle.
	MessageItemID string
	MessageIndex  int
	TextPartOpen  bool

	Text      strings.Builder
	Reasoning strings.Builder

	// Tool-call lifecycle, keyed by the upstream tool_call index.
	ToolCalls                 map[int]*ChatToolCall
	ToolItemIDs               map[int]string
	ToolOutputIndex           map[int]int
	toolArguments             map[int]*strings.Builder
	toolArgumentBytes         int
	maxToolArgumentBytes      int
	maxTotalToolArgumentBytes int
	streamError               error

	// CustomTools 是客户端请求中 custom/freeform 工具的名字集合（见
	// CustomToolNames）。命中的调用按 custom_tool_call 生命周期下发，codex 才能
	// 路由回它注册的 custom 工具。
	CustomTools map[string]bool

	// ToolSearchDeclared 表示客户端请求声明了 tool_search 工具（见
	// HasToolSearchTool）。命中的代理调用按 tool_search_call 项还原，codex 只按
	// 该项类型（且 execution=client）执行 tool search。
	ToolSearchDeclared bool

	// NamespaceTools 是 namespace 子工具的摊平名 → 原始归属映射（见
	// NamespaceToolNames）。命中的调用还原为带 namespace 字段的 function_call 项，
	// codex 按 namespace+name 路由。
	NamespaceTools map[string]NamespacedToolName

	// toolIsCustom 记录每个工具调用宣告时的类型判定，保证 added/done 事件的
	// 项类型一致。
	toolIsCustom map[int]bool

	// toolIsToolSearch 记录工具调用是否判定为 tool_search 代理调用。
	toolIsToolSearch map[int]bool

	// toolNamespace 记录工具调用宣告时命中的 namespace 归属（见 NamespaceTools）。
	toolNamespace map[int]NamespacedToolName

	// toolAnnounced 记录 output_item.added 是否已发出。存在 custom 工具且名字
	// 尚未到达时延迟宣告，待名字可判定类型后再补发（见 announceChatToolItem）。
	toolAnnounced map[int]bool

	FinishReason string
	Usage        *ResponsesUsage
}

const (
	defaultMaxChatStreamToolArgumentBytes      = 16 << 20
	defaultMaxChatStreamTotalToolArgumentBytes = 32 << 20
)

// NewChatCompletionsToResponsesStreamState returns an initialized stream state.
func NewChatCompletionsToResponsesStreamState(model string) *ChatCompletionsToResponsesStreamState {
	return &ChatCompletionsToResponsesStreamState{
		ResponseID:                generateResponsesID(),
		Model:                     model,
		Created:                   time.Now().Unix(),
		ToolCalls:                 make(map[int]*ChatToolCall),
		ToolItemIDs:               make(map[int]string),
		ToolOutputIndex:           make(map[int]int),
		toolArguments:             make(map[int]*strings.Builder),
		maxToolArgumentBytes:      defaultMaxChatStreamToolArgumentBytes,
		maxTotalToolArgumentBytes: defaultMaxChatStreamTotalToolArgumentBytes,
		toolIsCustom:              make(map[int]bool),
		toolIsToolSearch:          make(map[int]bool),
		toolNamespace:             make(map[int]NamespacedToolName),
		toolAnnounced:             make(map[int]bool),
	}
}

// StreamError reports a terminal conversion error detected while consuming an
// upstream Chat Completions stream. Callers must stop reading and skip normal
// finalization when it is non-nil.
func (state *ChatCompletionsToResponsesStreamState) StreamError() error {
	if state == nil {
		return nil
	}
	return state.streamError
}

func (state *ChatCompletionsToResponsesStreamState) appendToolArguments(index int, delta string) error {
	if delta == "" {
		return nil
	}
	buffer := state.toolArguments[index]
	currentBytes := 0
	if buffer != nil {
		currentBytes = buffer.Len()
	}
	if len(delta) > state.maxToolArgumentBytes-currentBytes {
		return fmt.Errorf("chat completions upstream tool arguments exceed %d bytes for one call", state.maxToolArgumentBytes)
	}
	if len(delta) > state.maxTotalToolArgumentBytes-state.toolArgumentBytes {
		return fmt.Errorf("chat completions upstream tool arguments exceed %d total bytes", state.maxTotalToolArgumentBytes)
	}
	if buffer == nil {
		buffer = &strings.Builder{}
		buffer.Grow(min(state.maxToolArgumentBytes, max(256, len(delta))))
		state.toolArguments[index] = buffer
	}
	_, _ = buffer.WriteString(delta)
	state.toolArgumentBytes += len(delta)
	return nil
}

func (state *ChatCompletionsToResponsesStreamState) toolArgumentsFor(index int) string {
	if state == nil || state.toolArguments[index] == nil {
		return ""
	}
	return state.toolArguments[index].String()
}

func failChatCompletionsResponsesStream(state *ChatCompletionsToResponsesStreamState, err error) []ResponsesStreamEvent {
	if state == nil || err == nil || state.CompletedSent {
		return nil
	}
	state.streamError = err
	state.CompletedSent = true
	events := ensureChatToResponsesCreated(state)
	events = append(events, chatToResponsesEvent(state, "response.failed", &ResponsesStreamEvent{
		Response: &ResponsesResponse{
			ID:     state.ResponseID,
			Object: "response",
			Model:  state.Model,
			Status: "failed",
			Output: []ResponsesOutput{},
			Usage:  state.Usage,
			Error: &ResponsesError{
				Code:    "upstream_response_too_large",
				Message: err.Error(),
			},
		},
	}))
	return events
}

func (state *ChatCompletionsToResponsesStreamState) allocOutputIndex() int {
	idx := state.nextOutputIndex
	state.nextOutputIndex++
	return idx
}

// ChatCompletionsChunkToResponsesEvents converts one Chat Completions stream
// chunk into zero or more Responses stream events.
func ChatCompletionsChunkToResponsesEvents(
	chunk *ChatCompletionsChunk,
	state *ChatCompletionsToResponsesStreamState,
) []ResponsesStreamEvent {
	if chunk == nil || state == nil {
		return nil
	}
	if state.streamError != nil {
		return nil
	}
	if chunk.ID != "" {
		state.ResponseID = chunk.ID
	}
	if state.Model == "" && chunk.Model != "" {
		state.Model = chunk.Model
	}
	if chunk.Usage != nil {
		state.Usage = ChatUsageToResponsesUsage(chunk.Usage)
	}

	var events []ResponsesStreamEvent
	events = append(events, ensureChatToResponsesCreated(state)...)

	for _, choice := range chunk.Choices {
		// Reasoning is emitted as its own output item and must be opened
		// (output_item.added + reasoning_summary_part.added) before the first
		// delta, otherwise a strict client discards the delta. The leading
		// empty-string reasoning delta upstreams send is filtered out.
		if choice.Delta.ReasoningContent != nil && *choice.Delta.ReasoningContent != "" {
			events = append(events, ensureChatReasoningItem(state)...)
			_, _ = state.Reasoning.WriteString(*choice.Delta.ReasoningContent)
			events = append(events, chatToResponsesEvent(state, "response.reasoning_summary_text.delta", &ResponsesStreamEvent{
				OutputIndex:  state.ReasoningIndex,
				SummaryIndex: 0,
				Delta:        *choice.Delta.ReasoningContent,
				ItemID:       state.ReasoningItemID,
			}))
		}
		if choice.Delta.Content != nil && *choice.Delta.Content != "" {
			// First real content closes the reasoning item, then opens the
			// message item and its output_text content part.
			events = append(events, closeChatReasoningItem(state)...)
			events = append(events, ensureChatToResponsesMessageItem(state)...)
			events = append(events, ensureChatToResponsesTextPart(state)...)
			_, _ = state.Text.WriteString(*choice.Delta.Content)
			events = append(events, chatToResponsesEvent(state, "response.output_text.delta", &ResponsesStreamEvent{
				OutputIndex:  state.MessageIndex,
				ContentIndex: 0,
				Delta:        *choice.Delta.Content,
				ItemID:       state.MessageItemID,
			}))
		}
		for _, toolCall := range choice.Delta.ToolCalls {
			idx := 0
			if toolCall.Index != nil {
				idx = *toolCall.Index
			}
			stored, ok := state.ToolCalls[idx]
			if !ok {
				// A tool call closes any open reasoning item first.
				events = append(events, closeChatReasoningItem(state)...)
				copyCall := toolCall
				if copyCall.ID == "" {
					copyCall.ID = generateItemID()
				}
				copyCall.Type = "function"
				// Arguments are accumulated by the shared block below so the
				// emitted delta and the stored value stay in sync. Some upstreams
				// (e.g. GLM/Zhipu) pack id+name+arguments into the first tool_call
				// chunk; without this reset the first chunk's arguments would be
				// counted twice (once from this copy, once from the += below),
				// producing a doubled, invalid JSON like {"a":1}{"a":1}.
				copyCall.Function.Arguments = ""
				state.ToolCalls[idx] = &copyCall
				stored = &copyCall
				state.ToolItemIDs[idx] = generateItemID()
				state.ToolOutputIndex[idx] = state.allocOutputIndex()
			} else {
				if toolCall.ID != "" {
					stored.ID = toolCall.ID
				}
				if toolCall.Function.Name != "" {
					stored.Function.Name = toolCall.Function.Name
				}
			}
			wasAnnounced := state.toolAnnounced[idx]
			if err := state.appendToolArguments(idx, toolCall.Function.Arguments); err != nil {
				events = append(events, failChatCompletionsResponsesStream(state, err)...)
				return events
			}
			events = append(events, announceChatToolItem(state, idx, stored, false)...)
			if toolCall.Function.Arguments != "" {
				// 未宣告（名字未到）时仅累积，宣告时统一补发；custom 调用的
				// arguments 是包裹 input 的 JSON 片段，无法增量还原为自由文本
				// 输入，缓冲整份 arguments 收尾时一次性下发（见 closeChatToolItems）；
				// tool_search 调用同样收尾时随 output_item.done 全量下发。
				if wasAnnounced && state.toolAnnounced[idx] && !state.toolIsCustom[idx] && !state.toolIsToolSearch[idx] {
					name := stored.Function.Name
					if ns, ok := state.toolNamespace[idx]; ok {
						name = ns.Name
					}
					events = append(events, chatToResponsesEvent(state, "response.function_call_arguments.delta", &ResponsesStreamEvent{
						OutputIndex: state.ToolOutputIndex[idx],
						ItemID:      state.ToolItemIDs[idx],
						Delta:       toolCall.Function.Arguments,
						CallID:      stored.ID,
						Name:        name,
					}))
				}
			}
		}
		if choice.FinishReason != nil && *choice.FinishReason != "" {
			state.FinishReason = *choice.FinishReason
		}
	}

	return events
}

// FinalizeChatCompletionsResponsesStream emits terminal Responses events.
func FinalizeChatCompletionsResponsesStream(state *ChatCompletionsToResponsesStreamState) []ResponsesStreamEvent {
	if state == nil || state.CompletedSent {
		return nil
	}
	var events []ResponsesStreamEvent
	events = append(events, ensureChatToResponsesCreated(state)...)

	// Close a reasoning item that never transitioned to content (reasoning-only
	// or empty completion).
	events = append(events, closeChatReasoningItem(state)...)
	events = append(events, synthesizeChatReasoningFallbackMessage(state)...)

	if state.MessageItemID != "" {
		if state.TextPartOpen {
			events = append(events, chatToResponsesEvent(state, "response.output_text.done", &ResponsesStreamEvent{
				OutputIndex:  state.MessageIndex,
				ContentIndex: 0,
				Text:         state.Text.String(),
				ItemID:       state.MessageItemID,
			}))
			events = append(events, chatToResponsesEvent(state, "response.content_part.done", &ResponsesStreamEvent{
				OutputIndex:  state.MessageIndex,
				ContentIndex: 0,
				ItemID:       state.MessageItemID,
				Part:         &ResponsesContentPart{Type: "output_text", Text: state.Text.String()},
			}))
		}
		events = append(events, chatToResponsesEvent(state, "response.output_item.done", &ResponsesStreamEvent{
			OutputIndex: state.MessageIndex,
			Item: &ResponsesOutput{
				Type:    "message",
				ID:      state.MessageItemID,
				Role:    "assistant",
				Content: []ResponsesContentPart{{Type: "output_text", Text: state.Text.String()}},
				Status:  "completed",
			},
		}))
	}

	// Close every function_call item opened during the stream. Codex finalizes a
	// tool call only after function_call_arguments.done + output_item.done for
	// that item; without them the call never completes and the session wedges.
	// Mirrors cc-switch's finalize_tools.
	events = append(events, closeChatToolItems(state)...)

	status := "completed"
	var incompleteDetails *ResponsesIncompleteDetails
	if state.FinishReason == "length" {
		status = "incomplete"
		incompleteDetails = &ResponsesIncompleteDetails{Reason: "max_output_tokens"}
	}

	state.CompletedSent = true
	events = append(events, chatToResponsesEvent(state, "response.completed", &ResponsesStreamEvent{
		Response: &ResponsesResponse{
			ID:                state.ResponseID,
			Object:            "response",
			Model:             state.Model,
			Status:            status,
			Output:            state.chatOutput(),
			Usage:             state.Usage,
			IncompleteDetails: incompleteDetails,
		},
	}))
	return events
}

func ensureChatToResponsesCreated(state *ChatCompletionsToResponsesStreamState) []ResponsesStreamEvent {
	if state.CreatedSent {
		return nil
	}
	state.CreatedSent = true
	return []ResponsesStreamEvent{chatToResponsesEvent(state, "response.created", &ResponsesStreamEvent{
		Response: &ResponsesResponse{
			ID:     state.ResponseID,
			Object: "response",
			Model:  state.Model,
			Status: "in_progress",
			Output: []ResponsesOutput{},
		},
	})}
}

// ensureChatReasoningItem opens the reasoning output item (output_item.added +
// reasoning_summary_part.added) before the first reasoning delta. Codex renders
// streaming reasoning only when this summary-part lifecycle is present.
func ensureChatReasoningItem(state *ChatCompletionsToResponsesStreamState) []ResponsesStreamEvent {
	if state.ReasoningOpen || state.ReasoningDone {
		return nil
	}
	state.ReasoningOpen = true
	state.ReasoningItemID = generateItemID()
	state.ReasoningIndex = state.allocOutputIndex()
	return []ResponsesStreamEvent{
		chatToResponsesEvent(state, "response.output_item.added", &ResponsesStreamEvent{
			OutputIndex: state.ReasoningIndex,
			Item:        &ResponsesOutput{Type: "reasoning", ID: state.ReasoningItemID, Status: "in_progress"},
		}),
		chatToResponsesEvent(state, "response.reasoning_summary_part.added", &ResponsesStreamEvent{
			OutputIndex:  state.ReasoningIndex,
			SummaryIndex: 0,
			ItemID:       state.ReasoningItemID,
			Part:         &ResponsesContentPart{Type: "summary_text"},
		}),
	}
}

// closeChatReasoningItem emits the reasoning item's terminal events
// (reasoning_summary_text.done + reasoning_summary_part.done + output_item.done).
func closeChatReasoningItem(state *ChatCompletionsToResponsesStreamState) []ResponsesStreamEvent {
	if !state.ReasoningOpen {
		return nil
	}
	state.ReasoningOpen = false
	state.ReasoningDone = true
	reasoning := state.Reasoning.String()
	return []ResponsesStreamEvent{
		chatToResponsesEvent(state, "response.reasoning_summary_text.done", &ResponsesStreamEvent{
			OutputIndex:  state.ReasoningIndex,
			SummaryIndex: 0,
			Text:         reasoning,
			ItemID:       state.ReasoningItemID,
		}),
		chatToResponsesEvent(state, "response.reasoning_summary_part.done", &ResponsesStreamEvent{
			OutputIndex:  state.ReasoningIndex,
			SummaryIndex: 0,
			ItemID:       state.ReasoningItemID,
			Part:         &ResponsesContentPart{Type: "summary_text", Text: reasoning},
		}),
		chatToResponsesEvent(state, "response.output_item.done", &ResponsesStreamEvent{
			OutputIndex: state.ReasoningIndex,
			Item: &ResponsesOutput{
				Type:    "reasoning",
				ID:      state.ReasoningItemID,
				Status:  "completed",
				Summary: []ResponsesSummary{{Type: "summary_text", Text: reasoning}},
			},
		}),
	}
}

func synthesizeChatReasoningFallbackMessage(state *ChatCompletionsToResponsesStreamState) []ResponsesStreamEvent {
	if state == nil ||
		state.MessageItemID != "" ||
		state.Text.Len() > 0 ||
		state.Reasoning.Len() == 0 ||
		len(state.ToolCalls) > 0 {
		return nil
	}

	text := state.Reasoning.String()
	if strings.TrimSpace(text) == "" {
		return nil
	}

	var events []ResponsesStreamEvent
	events = append(events, ensureChatToResponsesMessageItem(state)...)
	events = append(events, ensureChatToResponsesTextPart(state)...)
	_, _ = state.Text.WriteString(text)
	events = append(events, chatToResponsesEvent(state, "response.output_text.delta", &ResponsesStreamEvent{
		OutputIndex:  state.MessageIndex,
		ContentIndex: 0,
		Delta:        text,
		ItemID:       state.MessageItemID,
	}))
	return events
}

func ensureChatToResponsesMessageItem(state *ChatCompletionsToResponsesStreamState) []ResponsesStreamEvent {
	if state.MessageItemID != "" {
		return nil
	}
	state.MessageItemID = generateItemID()
	state.MessageIndex = state.allocOutputIndex()
	return []ResponsesStreamEvent{chatToResponsesEvent(state, "response.output_item.added", &ResponsesStreamEvent{
		OutputIndex: state.MessageIndex,
		Item: &ResponsesOutput{
			Type:    "message",
			ID:      state.MessageItemID,
			Role:    "assistant",
			Status:  "in_progress",
			Content: []ResponsesContentPart{{Type: "output_text"}},
		},
	})}
}

func ensureChatToResponsesTextPart(state *ChatCompletionsToResponsesStreamState) []ResponsesStreamEvent {
	if state.TextPartOpen {
		return nil
	}
	state.TextPartOpen = true
	return []ResponsesStreamEvent{chatToResponsesEvent(state, "response.content_part.added", &ResponsesStreamEvent{
		OutputIndex:  state.MessageIndex,
		ContentIndex: 0,
		ItemID:       state.MessageItemID,
		Part:         &ResponsesContentPart{Type: "output_text", Text: ""},
	})}
}

// announceChatToolItem 在类型可判定时发出工具调用的 output_item.added。custom
// 工具的判定依赖名字：名字未到且请求里存在 custom 工具时延迟宣告，避免 added/done
// 的项类型不一致；force 用于流收尾，名字始终未到时按 function_call 兜底。
func announceChatToolItem(
	state *ChatCompletionsToResponsesStreamState,
	idx int,
	stored *ChatToolCall,
	force bool,
) []ResponsesStreamEvent {
	if state.toolAnnounced[idx] {
		return nil
	}
	if !force && stored.Function.Name == "" && (len(state.CustomTools) > 0 || state.ToolSearchDeclared || len(state.NamespaceTools) > 0) {
		return nil
	}
	state.toolAnnounced[idx] = true
	isCustom := state.CustomTools[stored.Function.Name]
	isToolSearch := !isCustom && state.ToolSearchDeclared && stored.Function.Name == toolSearchProxyName
	state.toolIsCustom[idx] = isCustom
	state.toolIsToolSearch[idx] = isToolSearch
	itemType := "function_call"
	if isCustom {
		itemType = "custom_tool_call"
	}
	if isToolSearch {
		itemType = "tool_search_call"
	}
	// namespace 子工具的调用仍按 function_call 生命周期下发，但 added/done 项要
	// 还原为裸子工具名 + namespace 字段（codex 按 namespace+name 路由）。
	itemName, itemNamespace := stored.Function.Name, ""
	if ns, ok := state.NamespaceTools[stored.Function.Name]; ok && !isCustom && !isToolSearch {
		state.toolNamespace[idx] = ns
		itemName, itemNamespace = ns.Name, ns.Namespace
	}
	events := []ResponsesStreamEvent{chatToResponsesEvent(state, "response.output_item.added", &ResponsesStreamEvent{
		OutputIndex: state.ToolOutputIndex[idx],
		Item: &ResponsesOutput{
			Type:      itemType,
			ID:        state.ToolItemIDs[idx],
			CallID:    stored.ID,
			Name:      itemName,
			Namespace: itemNamespace,
			Status:    "in_progress",
		},
	})}
	// 迟到宣告时补发已累积的参数增量（custom/tool_search 的输入收尾统一下发，不补发）。
	arguments := state.toolArgumentsFor(idx)
	if !isCustom && !isToolSearch && arguments != "" {
		name := stored.Function.Name
		if ns, ok := state.toolNamespace[idx]; ok {
			name = ns.Name
		}
		events = append(events, chatToResponsesEvent(state, "response.function_call_arguments.delta", &ResponsesStreamEvent{
			OutputIndex: state.ToolOutputIndex[idx],
			ItemID:      state.ToolItemIDs[idx],
			Delta:       arguments,
			CallID:      stored.ID,
			Name:        name,
		}))
	}
	return events
}

// closeChatToolItems emits function_call_arguments.done + output_item.done for
// every tool call opened during the stream, carrying the full call_id/name/
// arguments so codex can deserialize and execute the call. Mirrors cc-switch's
// finalize_tools.
func closeChatToolItems(state *ChatCompletionsToResponsesStreamState) []ResponsesStreamEvent {
	if len(state.ToolCalls) == 0 {
		return nil
	}
	var events []ResponsesStreamEvent
	for i := 0; i < len(state.ToolCalls); i++ {
		toolCall, ok := state.ToolCalls[i]
		if !ok || toolCall == nil {
			continue
		}
		itemID, opened := state.ToolItemIDs[i]
		if !opened {
			continue
		}
		// 名字始终未到导致尚未宣告的调用，收尾前按最终名字兜底宣告。
		events = append(events, announceChatToolItem(state, i, toolCall, true)...)
		arguments := state.toolArgumentsFor(i)
		if strings.TrimSpace(arguments) == "" {
			arguments = "{}"
		}
		toolCall.Function.Arguments = arguments
		outputIndex := state.ToolOutputIndex[i]
		if state.toolIsCustom[i] {
			// custom 调用按 custom_tool_call 生命周期收尾：input 在此处一次性下发
			// （流中不产出增量，见 ChatCompletionsChunkToResponsesEvents）。
			input := extractCustomToolCallInput(arguments)
			if input != "" {
				events = append(events, chatToResponsesEvent(state, "response.custom_tool_call_input.delta", &ResponsesStreamEvent{
					OutputIndex: outputIndex,
					ItemID:      itemID,
					Delta:       input,
				}))
			}
			events = append(events,
				chatToResponsesEvent(state, "response.custom_tool_call_input.done", &ResponsesStreamEvent{
					OutputIndex: outputIndex,
					ItemID:      itemID,
					CallID:      toolCall.ID,
					Name:        toolCall.Function.Name,
					Input:       input,
				}),
				chatToResponsesEvent(state, "response.output_item.done", &ResponsesStreamEvent{
					OutputIndex: outputIndex,
					Item: &ResponsesOutput{
						Type:   "custom_tool_call",
						ID:     itemID,
						CallID: toolCall.ID,
						Name:   toolCall.Function.Name,
						Input:  input,
						Status: "completed",
					},
				}),
			)
			continue
		}
		if state.toolIsToolSearch[i] {
			// tool_search 调用按 tool_search_call 项收尾：codex 从 output_item.done
			// 物化该调用（无参数增量事件），arguments 全量随项下发。
			events = append(events, chatToResponsesEvent(state, "response.output_item.done", &ResponsesStreamEvent{
				OutputIndex: outputIndex,
				Item: &ResponsesOutput{
					Type:      "tool_search_call",
					ID:        itemID,
					CallID:    toolCall.ID,
					Arguments: arguments,
					Status:    "completed",
				},
			}))
			continue
		}
		// namespace 子工具调用在宣告时已记录归属，收尾项同样带还原名与 namespace。
		name, namespace := toolCall.Function.Name, ""
		if ns, ok := state.toolNamespace[i]; ok {
			name, namespace = ns.Name, ns.Namespace
		}
		events = append(events,
			chatToResponsesEvent(state, "response.function_call_arguments.done", &ResponsesStreamEvent{
				OutputIndex: outputIndex,
				ItemID:      itemID,
				CallID:      toolCall.ID,
				Name:        name,
				Arguments:   arguments,
			}),
			chatToResponsesEvent(state, "response.output_item.done", &ResponsesStreamEvent{
				OutputIndex: outputIndex,
				Item: &ResponsesOutput{
					Type:      "function_call",
					ID:        itemID,
					CallID:    toolCall.ID,
					Name:      name,
					Namespace: namespace,
					Arguments: arguments,
					Status:    "completed",
				},
			}),
		)
	}
	return events
}

func (state *ChatCompletionsToResponsesStreamState) chatOutput() []ResponsesOutput {
	var outputs []ResponsesOutput
	if state.Reasoning.Len() > 0 {
		outputs = append(outputs, ResponsesOutput{
			Type: "reasoning",
			ID:   nonEmpty(state.ReasoningItemID, generateItemID()),
			Summary: []ResponsesSummary{{
				Type: "summary_text",
				Text: state.Reasoning.String(),
			}},
		})
	}
	if state.MessageItemID != "" || len(state.ToolCalls) == 0 {
		outputs = append(outputs, ResponsesOutput{
			Type: "message",
			ID:   nonEmpty(state.MessageItemID, generateItemID()),
			Role: "assistant",
			Content: []ResponsesContentPart{{
				Type: "output_text",
				Text: state.Text.String(),
			}},
			Status: "completed",
		})
	}
	for i := 0; i < len(state.ToolCalls); i++ {
		toolCall, ok := state.ToolCalls[i]
		if !ok || toolCall == nil {
			continue
		}
		arguments := toolCall.Function.Arguments
		if strings.TrimSpace(arguments) == "" {
			arguments = "{}"
		}
		if state.toolIsCustom[i] {
			outputs = append(outputs, ResponsesOutput{
				Type:   "custom_tool_call",
				ID:     nonEmpty(state.ToolItemIDs[i], generateItemID()),
				CallID: toolCall.ID,
				Name:   toolCall.Function.Name,
				Input:  extractCustomToolCallInput(arguments),
				Status: "completed",
			})
			continue
		}
		if state.toolIsToolSearch[i] {
			outputs = append(outputs, ResponsesOutput{
				Type:      "tool_search_call",
				ID:        nonEmpty(state.ToolItemIDs[i], generateItemID()),
				CallID:    toolCall.ID,
				Arguments: arguments,
				Status:    "completed",
			})
			continue
		}
		name, namespace := toolCall.Function.Name, ""
		if ns, ok := state.toolNamespace[i]; ok {
			name, namespace = ns.Name, ns.Namespace
		}
		outputs = append(outputs, ResponsesOutput{
			Type:      "function_call",
			ID:        nonEmpty(state.ToolItemIDs[i], generateItemID()),
			CallID:    toolCall.ID,
			Name:      name,
			Namespace: namespace,
			Arguments: arguments,
			Status:    "completed",
		})
	}
	return outputs
}

func chatToResponsesEvent(
	state *ChatCompletionsToResponsesStreamState,
	eventType string,
	template *ResponsesStreamEvent,
) ResponsesStreamEvent {
	seq := state.SequenceNumber
	state.SequenceNumber++
	evt := *template
	evt.Type = eventType
	evt.SequenceNumber = seq
	return evt
}

func rawString(raw json.RawMessage) string {
	raw = bytesTrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return ""
}

func rawNestedString(raw json.RawMessage, key string) string {
	switch key {
	case "url":
		var value struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal(raw, &value); err == nil {
			return value.URL
		}
	case "name":
		var value struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(raw, &value); err == nil {
			return value.Name
		}
	}
	return ""
}

func bytesTrimSpace(raw json.RawMessage) json.RawMessage {
	return json.RawMessage(strings.TrimSpace(string(raw)))
}

func nonEmpty(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
