package apicompat

import (
	"encoding/json"
	"fmt"
)

// ChatCompletionsCapabilities describes optional Chat Completions features that
// cannot be assumed for every OpenAI-compatible upstream.
type ChatCompletionsCapabilities struct {
	SupportsAllowedTools          bool
	AllowImplicitClientToolSearch bool
	AllowLossyCustomToolGrammar   bool
}

// DefaultChatCompletionsCapabilities targets the current OpenAI Chat API while
// keeping Tool Search execution semantics strict. Account-backed third-party
// fallbacks should derive their capabilities from the selected account instead.
func DefaultChatCompletionsCapabilities() ChatCompletionsCapabilities {
	return ChatCompletionsCapabilities{SupportsAllowedTools: true}
}

// ChatCompletionsCapabilityError means the Responses request is valid but the
// selected Chat Completions transport cannot preserve the requested behavior.
// Service code should treat it as an account capability mismatch, not as a
// malformed client request.
type ChatCompletionsCapabilityError struct {
	Feature string
	Message string
}

func (e *ChatCompletionsCapabilityError) Error() string {
	if e == nil {
		return "chat completions capability mismatch"
	}
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("chat completions upstream does not support %s", e.Feature)
}

func newChatCompletionsCapabilityError(feature, message string) error {
	return &ChatCompletionsCapabilityError{Feature: feature, Message: message}
}

type responsesToolSource uint8

const (
	responsesToolSourceTopLevel responsesToolSource = iota
	responsesToolSourceAdditionalTools
	responsesToolSourceToolSearchOutput
)

type registeredResponsesTool struct {
	tool                   ResponsesTool
	source                 responsesToolSource
	active                 bool
	inputItemIndex         int
	inactiveFromInputIndex int
}

type historicalResponseToolName struct {
	identity NamespacedToolName
	chatName string
}

// ResponsesToolRegistry is the request-local, immutable view of tool state.
// It retains carrier provenance while exposing only tools callable at the end
// of the supplied input history. In particular, top-level defer_loading tools
// stay hidden until an additional_tools or tool_search_output carrier loads
// them.
type ResponsesToolRegistry struct {
	registrations                 []registeredResponsesTool
	inputText                     string
	inputIsText                   bool
	inputItems                    []json.RawMessage
	repeatedToolSearchOutputItems map[int]bool
	historicalToolNames           map[int]historicalResponseToolName

	customTools    map[string]bool
	namespaceTools map[string]NamespacedToolName
	responseNames  map[NamespacedToolName]string
}

const (
	maxResponsesToolCount           = 1024
	maxResponsesToolDefinitionBytes = 1 << 20
	maxResponsesToolTotalBytes      = 16 << 20
	maxResponsesToolNamespaceDepth  = 16
	maxResponsesInputItemCount      = 16 << 10
)

// BuildResponsesToolRegistry parses a Responses request exactly once and
// replays its tool carriers in input order.
func BuildResponsesToolRegistry(req *ResponsesRequest) (*ResponsesToolRegistry, error) {
	if req == nil {
		return nil, fmt.Errorf("responses request is nil")
	}

	registry := &ResponsesToolRegistry{
		customTools:                   make(map[string]bool),
		namespaceTools:                make(map[string]NamespacedToolName),
		responseNames:                 make(map[NamespacedToolName]string),
		repeatedToolSearchOutputItems: make(map[int]bool),
		historicalToolNames:           make(map[int]historicalResponseToolName),
	}
	activeHistoricalNames := make(map[NamespacedToolName]map[string]int)
	forEachRegistrationIdentity := func(registration registeredResponsesTool, visit func(string, NamespacedToolName)) {
		tool := registration.tool
		switch tool.Type {
		case "function":
			if tool.Name == "" {
				return
			}
			identity := NamespacedToolName{Name: tool.Name}
			if registration.source == responsesToolSourceToolSearchOutput {
				identity.Namespace = tool.Name
			}
			visit(tool.Name, identity)
		case "namespace":
			children := tool.Tools
			if len(children) == 0 {
				children = tool.Children
			}
			for _, child := range children {
				if child.Type != "function" || child.Name == "" || tool.Name == "" {
					continue
				}
				visit(flattenNamespaceToolName(tool.Name, child.Name), NamespacedToolName{Namespace: tool.Name, Name: child.Name})
			}
		}
	}
	activateHistoricalRegistration := func(registration registeredResponsesTool) {
		forEachRegistrationIdentity(registration, func(chatName string, identity NamespacedToolName) {
			names := activeHistoricalNames[identity]
			if names == nil {
				names = make(map[string]int)
				activeHistoricalNames[identity] = names
			}
			names[chatName]++
		})
	}
	deactivateHistoricalRegistration := func(registration registeredResponsesTool) {
		forEachRegistrationIdentity(registration, func(chatName string, identity NamespacedToolName) {
			names := activeHistoricalNames[identity]
			if names == nil {
				return
			}
			if names[chatName] <= 1 {
				delete(names, chatName)
			} else {
				names[chatName]--
			}
			if len(names) == 0 {
				delete(activeHistoricalNames, identity)
			}
		})
	}
	resolveHistoricalName := func(inputItemIndex int, namespace, name string) error {
		identity := NamespacedToolName{Namespace: namespace, Name: name}
		candidates := activeHistoricalNames[identity]
		if len(candidates) == 0 {
			return nil
		}
		chatName := ""
		for candidate := range candidates {
			if chatName != "" && chatName != candidate {
				return newChatCompletionsCapabilityError("chat_tool_identity", fmt.Sprintf("Responses tool identity %q/%q maps to conflicting historical Chat tool names %q and %q", namespace, name, chatName, candidate))
			}
			chatName = candidate
		}
		registry.historicalToolNames[inputItemIndex] = historicalResponseToolName{identity: identity, chatName: chatName}
		return nil
	}

	toolCount, totalBytes := 0, 0
	validateTool := func(tool ResponsesTool) error {
		if err := validateResponsesToolResourceBounds(tool, 0, &toolCount, &totalBytes); err != nil {
			return err
		}
		return nil
	}
	appendRegistration := func(tool ResponsesTool, source responsesToolSource, inputItemIndex int) {
		registration := registeredResponsesTool{
			tool:                   tool,
			source:                 source,
			active:                 true,
			inputItemIndex:         inputItemIndex,
			inactiveFromInputIndex: -1,
		}
		registry.registrations = append(registry.registrations, registration)
		activateHistoricalRegistration(registration)
	}
	appendTool := func(tool ResponsesTool, source responsesToolSource, inputItemIndex int) error {
		if err := validateTool(tool); err != nil {
			return err
		}
		appendRegistration(tool, source, inputItemIndex)
		return nil
	}

	for _, tool := range req.Tools {
		if err := validateTool(tool); err != nil {
			return nil, err
		}
		visible, ok := topLevelCallableTool(tool)
		if !ok {
			continue
		}
		appendRegistration(visible, responsesToolSourceTopLevel, -1)
	}

	inputRaw := bytesTrimSpace(req.Input)
	if len(inputRaw) == 0 || string(inputRaw) == "null" {
		if err := registry.rebuildIdentityMaps(); err != nil {
			return nil, err
		}
		return registry, nil
	}
	if err := json.Unmarshal(inputRaw, &registry.inputText); err == nil {
		registry.inputIsText = true
		if err := registry.rebuildIdentityMaps(); err != nil {
			return nil, err
		}
		return registry, nil
	}
	if err := json.Unmarshal(inputRaw, &registry.inputItems); err != nil {
		return nil, fmt.Errorf("parse responses input: %w", err)
	}
	if len(registry.inputItems) > maxResponsesInputItemCount {
		return nil, fmt.Errorf("responses input item count exceeds %d", maxResponsesInputItemCount)
	}

	toolSearchRanges := make(map[string][]int)
	firstToolSearchOutputItems := make(map[string]int)
	for inputItemIndex, raw := range registry.inputItems {
		var item map[string]json.RawMessage
		if err := json.Unmarshal(bytesTrimSpace(raw), &item); err != nil {
			// Bare string input items are valid user text and cannot carry tools.
			continue
		}
		carrierType := rawString(item["type"])
		if carrierType == "function_call" {
			namespace := rawString(item["namespace"])
			if namespace != "" {
				if err := resolveHistoricalName(inputItemIndex, namespace, rawString(item["name"])); err != nil {
					return nil, err
				}
			}
		}
		if carrierType != "additional_tools" && carrierType != "tool_search_output" {
			continue
		}
		toolsRaw := bytesTrimSpace(item["tools"])
		if len(toolsRaw) == 0 || string(toolsRaw) == "null" {
			continue
		}
		var tools []ResponsesTool
		if err := json.Unmarshal(toolsRaw, &tools); err != nil {
			return nil, fmt.Errorf("parse responses %s tools: %w", carrierType, err)
		}

		source := responsesToolSourceAdditionalTools
		callID := ""
		if carrierType == "tool_search_output" {
			source = responsesToolSourceToolSearchOutput
			callID = rawString(item["call_id"])
			// A repeated call_id represents an updated copy of the same historical
			// output. Replace its previous registrations instead of unioning stale
			// definitions back into the current tool set.
			if callID != "" {
				if _, exists := firstToolSearchOutputItems[callID]; exists {
					registry.repeatedToolSearchOutputItems[inputItemIndex] = true
				} else {
					firstToolSearchOutputItems[callID] = inputItemIndex
				}
				for _, registrationIndex := range toolSearchRanges[callID] {
					deactivateHistoricalRegistration(registry.registrations[registrationIndex])
					registry.registrations[registrationIndex].active = false
					registry.registrations[registrationIndex].inactiveFromInputIndex = inputItemIndex
				}
				toolSearchRanges[callID] = nil
			}
		}

		for _, tool := range tools {
			before := len(registry.registrations)
			if err := appendTool(tool, source, inputItemIndex); err != nil {
				return nil, err
			}
			if source == responsesToolSourceToolSearchOutput && callID != "" {
				toolSearchRanges[callID] = append(toolSearchRanges[callID], before)
			}
		}
	}

	if err := registry.rebuildIdentityMaps(); err != nil {
		return nil, err
	}
	return registry, nil
}

func topLevelCallableTool(tool ResponsesTool) (ResponsesTool, bool) {
	switch tool.Type {
	case "function", "custom":
		if tool.DeferLoading {
			return ResponsesTool{}, false
		}
	case "namespace":
		children := tool.Tools
		useChildren := false
		if len(children) == 0 {
			children = tool.Children
			useChildren = true
		}
		visible := make([]ResponsesTool, 0, len(children))
		for _, child := range children {
			if child.Type == "function" && !child.DeferLoading {
				visible = append(visible, child)
			}
		}
		if len(visible) == 0 {
			return ResponsesTool{}, false
		}
		tool.rawDefinition = nil
		if useChildren {
			tool.Children = visible
			tool.Tools = nil
		} else {
			tool.Tools = visible
			tool.Children = nil
		}
	}
	return tool, true
}

func validateResponsesToolResourceBounds(tool ResponsesTool, depth int, count, totalBytes *int) error {
	if depth > maxResponsesToolNamespaceDepth {
		return fmt.Errorf("responses tool namespace depth exceeds %d", maxResponsesToolNamespaceDepth)
	}
	(*count)++
	if *count > maxResponsesToolCount {
		return fmt.Errorf("responses tool count exceeds %d", maxResponsesToolCount)
	}
	raw, err := responsesToolDefinitionJSON(tool)
	if err != nil {
		return fmt.Errorf("marshal responses tool definition: %w", err)
	}
	if len(raw) > maxResponsesToolDefinitionBytes {
		return fmt.Errorf("responses tool definition exceeds %d bytes", maxResponsesToolDefinitionBytes)
	}
	*totalBytes += len(raw)
	if *totalBytes > maxResponsesToolTotalBytes {
		return fmt.Errorf("responses tool definitions exceed %d bytes", maxResponsesToolTotalBytes)
	}
	children := tool.Tools
	if len(children) == 0 {
		children = tool.Children
	}
	for _, child := range children {
		if err := validateResponsesToolResourceBounds(child, depth+1, count, totalBytes); err != nil {
			return err
		}
	}
	return nil
}

func (r *ResponsesToolRegistry) rebuildIdentityMaps() error {
	for key := range r.customTools {
		delete(r.customTools, key)
	}
	for key := range r.namespaceTools {
		delete(r.namespaceTools, key)
	}
	for key := range r.responseNames {
		delete(r.responseNames, key)
	}
	registerIdentity := func(chatName string, identity NamespacedToolName) error {
		if previousIdentity, exists := r.namespaceTools[chatName]; exists && previousIdentity != identity {
			return newChatCompletionsCapabilityError("chat_tool_identity", fmt.Sprintf("Responses tools %q/%q and %q/%q map to the same Chat tool name %q", previousIdentity.Namespace, previousIdentity.Name, identity.Namespace, identity.Name, chatName))
		}
		if previousChatName, exists := r.responseNames[identity]; exists && previousChatName != chatName {
			return newChatCompletionsCapabilityError("chat_tool_identity", fmt.Sprintf("Responses tool identity %q/%q maps to conflicting Chat tool names %q and %q", identity.Namespace, identity.Name, previousChatName, chatName))
		}
		r.namespaceTools[chatName] = identity
		r.responseNames[identity] = chatName
		return nil
	}

	for _, registration := range r.registrations {
		if !registration.active {
			continue
		}
		tool := registration.tool
		switch tool.Type {
		case "custom":
			if tool.Name != "" {
				r.customTools[tool.Name] = true
			}
		case "function":
			if tool.Name != "" {
				identity := NamespacedToolName{Name: tool.Name}
				if registration.source == responsesToolSourceToolSearchOutput {
					identity.Namespace = tool.Name
				}
				if err := registerIdentity(tool.Name, identity); err != nil {
					return err
				}
			}
		case "namespace":
			children := tool.Tools
			if len(children) == 0 {
				children = tool.Children
			}
			for _, child := range children {
				if child.Type != "function" || child.Name == "" || tool.Name == "" {
					continue
				}
				chatName := flattenNamespaceToolName(tool.Name, child.Name)
				identity := NamespacedToolName{Namespace: tool.Name, Name: child.Name}
				if err := registerIdentity(chatName, identity); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// Tools returns the tools callable at the end of the request input history.
func (r *ResponsesToolRegistry) Tools() []ResponsesTool {
	if r == nil {
		return nil
	}
	out := make([]ResponsesTool, 0, len(r.registrations))
	for _, registration := range r.registrations {
		if registration.active {
			out = append(out, registration.tool)
		}
	}
	return out
}

func (r *ResponsesToolRegistry) CustomToolNames() map[string]bool {
	if r == nil || len(r.customTools) == 0 {
		return nil
	}
	out := make(map[string]bool, len(r.customTools))
	for name, enabled := range r.customTools {
		out[name] = enabled
	}
	return out
}

// NamespaceToolNames returns the Chat name -> Responses identity map used by
// response conversion. It includes ordinary functions with an empty namespace
// so cross-source identity collisions cannot be silently collapsed.
func (r *ResponsesToolRegistry) NamespaceToolNames() map[string]NamespacedToolName {
	if r == nil || len(r.namespaceTools) == 0 {
		return nil
	}
	out := make(map[string]NamespacedToolName, len(r.namespaceTools))
	for name, identity := range r.namespaceTools {
		out[name] = identity
	}
	return out
}

func (r *ResponsesToolRegistry) HasClientToolSearch(capabilities ChatCompletionsCapabilities) bool {
	if r == nil {
		return false
	}
	for _, registration := range r.registrations {
		if !registration.active || registration.tool.Type != "tool_search" {
			continue
		}
		if clientToolSearchExecutionAllowed(registration.tool.Execution, capabilities) {
			return true
		}
	}
	return false
}

func clientToolSearchExecutionAllowed(execution string, capabilities ChatCompletionsCapabilities) bool {
	return execution == "client" || execution == "" && capabilities.AllowImplicitClientToolSearch
}

func validateClientToolSearchExecution(execution string, capabilities ChatCompletionsCapabilities) error {
	switch execution {
	case "client":
		return nil
	case "":
		if capabilities.AllowImplicitClientToolSearch {
			return nil
		}
		return newChatCompletionsCapabilityError("hosted_tool_search", "type-only tool_search uses hosted execution and cannot be represented by a Chat Completions upstream; use execution=client or a Responses-capable account")
	case "server":
		return newChatCompletionsCapabilityError("hosted_tool_search", "tool_search execution=server cannot be represented by a Chat Completions upstream; use execution=client or a Responses-capable account")
	default:
		return fmt.Errorf("tool_search execution must be client or server, got %q", execution)
	}
}

func (r *ResponsesToolRegistry) chatNameForResponseToolAt(inputItemIndex int, namespace, name string) (string, bool, error) {
	if r == nil {
		return "", false, nil
	}
	identity := NamespacedToolName{Namespace: namespace, Name: name}
	resolved, ok := r.historicalToolNames[inputItemIndex]
	if !ok || resolved.identity != identity {
		return "", false, nil
	}
	return resolved.chatName, true, nil
}

func (r *ResponsesToolRegistry) isRepeatedToolSearchOutputItem(inputItemIndex int) bool {
	return r != nil && r.repeatedToolSearchOutputItems[inputItemIndex]
}
