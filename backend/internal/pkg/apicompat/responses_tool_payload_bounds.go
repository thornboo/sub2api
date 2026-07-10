package apicompat

import (
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
)

const (
	maxResponsesJSONObjectFieldCount = 64
	maxResponsesContentPartCount     = 16 << 10
)

// ValidateResponsesToolPayload rejects invalid Tool Search execution values and
// oversized or excessively nested tool declarations directly from request
// bytes, before account scheduling and before encoding/json allocates full
// ResponsesTool trees and raw-definition copies.
func ValidateResponsesToolPayload(body []byte) error {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return nil
	}

	toolCount, totalBytes := 0, 0
	uniqueObjectFields := func(object gjson.Result, location string, retainedFields ...string) (map[string]gjson.Result, error) {
		if !object.IsObject() {
			return nil, fmt.Errorf("%s must be an object", location)
		}
		fields := make(map[string]gjson.Result, len(retainedFields))
		seen := make(map[string]struct{})
		var fieldErr error
		fieldCount := 0
		object.ForEach(func(key, value gjson.Result) bool {
			fieldCount++
			if fieldCount > maxResponsesJSONObjectFieldCount {
				fieldErr = fmt.Errorf("%s field count exceeds %d", location, maxResponsesJSONObjectFieldCount)
				return false
			}
			name := key.String()
			normalizedName := strings.ToLower(name)
			if _, exists := seen[normalizedName]; exists {
				fieldErr = fmt.Errorf("%s contains duplicate key %q", location, name)
				return false
			}
			seen[normalizedName] = struct{}{}
			for _, retainedField := range retainedFields {
				if normalizedName == retainedField {
					fields[normalizedName] = value
					break
				}
			}
			return true
		})
		return fields, fieldErr
	}
	validateContentPart := func(part gjson.Result, location string) error {
		fields, err := uniqueObjectFields(part, location, "image_url")
		if err != nil {
			return err
		}
		imageURL := fields["image_url"]
		if imageURL.IsObject() {
			_, err = uniqueObjectFields(imageURL, location+" image_url", "url")
		}
		return err
	}
	validateContent := func(content gjson.Result, location string) error {
		if !content.Exists() || content.Type == gjson.Null || content.Type == gjson.String {
			return nil
		}
		if content.IsObject() {
			return validateContentPart(content, location)
		}
		if !content.IsArray() {
			return nil
		}
		partCount := 0
		var contentErr error
		content.ForEach(func(index, part gjson.Result) bool {
			partCount++
			if partCount > maxResponsesContentPartCount {
				contentErr = fmt.Errorf("%s part count exceeds %d", location, maxResponsesContentPartCount)
				return false
			}
			if part.IsObject() {
				contentErr = validateContentPart(part, fmt.Sprintf("%s part %s", location, index.String()))
			}
			return contentErr == nil
		})
		return contentErr
	}
	arrayHasItems := func(array gjson.Result) bool {
		hasItems := false
		array.ForEach(func(_, _ gjson.Result) bool {
			hasItems = true
			return false
		})
		return hasItems
	}
	validateExecution := func(fields map[string]gjson.Result) error {
		execution, exists := fields["execution"]
		if !exists {
			return nil
		}
		if execution.Type != gjson.String {
			return fmt.Errorf("tool_search execution must be client or server")
		}
		switch execution.String() {
		case "client", "server":
			return nil
		default:
			return fmt.Errorf("tool_search execution must be client or server, got %q", execution.String())
		}
	}
	var validateTool func(gjson.Result, int) error
	validateTool = func(tool gjson.Result, depth int) error {
		if depth > maxResponsesToolNamespaceDepth {
			return fmt.Errorf("responses tool namespace depth exceeds %d", maxResponsesToolNamespaceDepth)
		}
		toolCount++
		if toolCount > maxResponsesToolCount {
			return fmt.Errorf("responses tool count exceeds %d", maxResponsesToolCount)
		}
		definitionBytes := len(tool.Raw)
		if definitionBytes > maxResponsesToolDefinitionBytes {
			return fmt.Errorf("responses tool definition exceeds %d bytes", maxResponsesToolDefinitionBytes)
		}
		totalBytes += definitionBytes
		if totalBytes > maxResponsesToolTotalBytes {
			return fmt.Errorf("responses tool definitions exceed %d bytes", maxResponsesToolTotalBytes)
		}

		// String shorthand represents a custom tool and has no object fields.
		if tool.Type == gjson.String {
			return nil
		}
		fields, err := uniqueObjectFields(tool, "responses tool", "type", "execution", "tools", "children")
		if err != nil {
			return err
		}
		if fields["type"].String() == "tool_search" {
			if err := validateExecution(fields); err != nil {
				return err
			}
		}

		children := fields["tools"]
		if !children.Exists() || !children.IsArray() || !arrayHasItems(children) {
			children = fields["children"]
		}
		if !children.IsArray() {
			return nil
		}
		var childErr error
		children.ForEach(func(_, child gjson.Result) bool {
			childErr = validateTool(child, depth+1)
			return childErr == nil
		})
		return childErr
	}

	validateTools := func(tools gjson.Result, location string) error {
		if !tools.Exists() || tools.Type == gjson.Null {
			return nil
		}
		if !tools.IsArray() {
			return fmt.Errorf("%s must be an array", location)
		}
		var toolErr error
		tools.ForEach(func(_, tool gjson.Result) bool {
			toolErr = validateTool(tool, 0)
			return toolErr == nil
		})
		return toolErr
	}

	root := gjson.ParseBytes(body)
	rootFields, err := uniqueObjectFields(root, "Responses request", "tools", "tool_choice", "input")
	if err != nil {
		return err
	}
	if err := validateTools(rootFields["tools"], "Responses request tools"); err != nil {
		return err
	}
	toolChoice := rootFields["tool_choice"]
	if toolChoice.IsObject() {
		choiceFields, err := uniqueObjectFields(toolChoice, "Responses tool_choice", "type", "tools", "function")
		if err != nil {
			return err
		}
		if function := choiceFields["function"]; function.IsObject() {
			if _, err := uniqueObjectFields(function, "Responses tool_choice function", "name"); err != nil {
				return err
			}
		}
		if choiceFields["type"].String() == "allowed_tools" {
			if err := validateTools(choiceFields["tools"], "Responses allowed_tools"); err != nil {
				return err
			}
		}
	}

	input := rootFields["input"]
	if !input.IsArray() {
		return nil
	}
	var inputErr error
	inputItemCount := 0
	input.ForEach(func(index, item gjson.Result) bool {
		inputItemCount++
		if inputItemCount > maxResponsesInputItemCount {
			inputErr = fmt.Errorf("responses input item count exceeds %d", maxResponsesInputItemCount)
			return false
		}
		if !item.IsObject() {
			return true
		}
		location := fmt.Sprintf("Responses input item %s", index.String())
		fields, err := uniqueObjectFields(item, location, "type", "execution", "tools", "content", "summary")
		if err != nil {
			inputErr = err
			return false
		}
		if inputErr = validateContent(fields["content"], location+" content"); inputErr != nil {
			return false
		}
		if inputErr = validateContent(fields["summary"], location+" summary"); inputErr != nil {
			return false
		}
		switch fields["type"].String() {
		case "additional_tools":
			inputErr = validateTools(fields["tools"], "Responses additional_tools")
		case "tool_search_call":
			inputErr = validateExecution(fields)
		case "tool_search_output":
			if inputErr = validateExecution(fields); inputErr == nil {
				inputErr = validateTools(fields["tools"], "Responses tool_search_output tools")
			}
		}
		return inputErr == nil
	})
	return inputErr
}
