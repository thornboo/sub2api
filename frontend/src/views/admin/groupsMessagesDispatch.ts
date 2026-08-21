import type { OpenAIMessagesDispatchModelConfig } from "@/types";

export interface MessagesDispatchMappingRow {
  claude_model: string;
  target_model: string;
}

export type MessagesDispatchFamilyMappingMode =
  | "legacy"
  | "passthrough"
  | "custom";

export interface MessagesDispatchFormState {
  allow_messages_dispatch: boolean;
  family_mapping_mode: MessagesDispatchFamilyMappingMode;
  opus_mapped_model: string;
  sonnet_mapped_model: string;
  haiku_mapped_model: string;
  exact_model_mappings: MessagesDispatchMappingRow[];
}

export function supportsMessagesDispatchPlatform(platform: string): boolean {
  return platform === "openai" || platform === "composite";
}

export function createDefaultMessagesDispatchFormState(): MessagesDispatchFormState {
  return {
    allow_messages_dispatch: false,
    family_mapping_mode: "passthrough",
    opus_mapped_model: "",
    sonnet_mapped_model: "",
    haiku_mapped_model: "",
    exact_model_mappings: [],
  };
}

const legacyOpenAIMessagesDispatchDefaults = {
  opus_mapped_model: "gpt-5.4",
  sonnet_mapped_model: "gpt-5.3-codex",
  haiku_mapped_model: "gpt-5.4-mini",
} as const;

export function messagesDispatchConfigToFormState(
  config?: OpenAIMessagesDispatchModelConfig | null,
): MessagesDispatchFormState {
  const configuredMode = config?.family_mapping_mode;
  const familyMappingMode: MessagesDispatchFamilyMappingMode =
    configuredMode === "passthrough" || configuredMode === "custom"
      ? configuredMode
      : "legacy";
  const exactMappings = Object.entries(config?.exact_model_mappings || {})
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([claude_model, target_model]) => ({ claude_model, target_model }));

  return {
    allow_messages_dispatch: false,
    family_mapping_mode: familyMappingMode,
    opus_mapped_model:
      familyMappingMode === "legacy"
        ? config?.opus_mapped_model?.trim() ||
          legacyOpenAIMessagesDispatchDefaults.opus_mapped_model
        : config?.opus_mapped_model?.trim() || "",
    sonnet_mapped_model:
      familyMappingMode === "legacy"
        ? config?.sonnet_mapped_model?.trim() ||
          legacyOpenAIMessagesDispatchDefaults.sonnet_mapped_model
        : config?.sonnet_mapped_model?.trim() || "",
    haiku_mapped_model:
      familyMappingMode === "legacy"
        ? config?.haiku_mapped_model?.trim() ||
          legacyOpenAIMessagesDispatchDefaults.haiku_mapped_model
        : config?.haiku_mapped_model?.trim() || "",
    exact_model_mappings: exactMappings,
  };
}

export function messagesDispatchFormStateToConfig(
  state: MessagesDispatchFormState,
): OpenAIMessagesDispatchModelConfig {
  const exactModelMappings = Object.fromEntries(
    state.exact_model_mappings
      .map((row) => [row.claude_model.trim(), row.target_model.trim()] as const)
      .filter(([claudeModel, targetModel]) => claudeModel && targetModel),
  );

  return {
    family_mapping_mode:
      state.family_mapping_mode === "legacy"
        ? undefined
        : state.family_mapping_mode,
    opus_mapped_model:
      state.family_mapping_mode === "passthrough"
        ? ""
        : state.opus_mapped_model.trim(),
    sonnet_mapped_model:
      state.family_mapping_mode === "passthrough"
        ? ""
        : state.sonnet_mapped_model.trim(),
    haiku_mapped_model:
      state.family_mapping_mode === "passthrough"
        ? ""
        : state.haiku_mapped_model.trim(),
    exact_model_mappings: exactModelMappings,
  };
}

export function applyOpenAIMessagesDispatchPreset(
  target: MessagesDispatchFormState,
): void {
  target.family_mapping_mode = "custom";
  target.opus_mapped_model =
    legacyOpenAIMessagesDispatchDefaults.opus_mapped_model;
  target.sonnet_mapped_model =
    legacyOpenAIMessagesDispatchDefaults.sonnet_mapped_model;
  target.haiku_mapped_model =
    legacyOpenAIMessagesDispatchDefaults.haiku_mapped_model;
}

export function isMessagesDispatchModelOutsideCandidates(
  model: string,
  candidates: string[],
): boolean {
  const normalizedModel = model.trim();
  if (!normalizedModel || candidates.length === 0) {
    return false;
  }
  return !candidates.some(
    (candidate) => candidate.trim() === normalizedModel,
  );
}

export function resetMessagesDispatchFormState(
  target: MessagesDispatchFormState,
): void {
  const defaults = createDefaultMessagesDispatchFormState();
  target.allow_messages_dispatch = defaults.allow_messages_dispatch;
  target.family_mapping_mode = defaults.family_mapping_mode;
  target.opus_mapped_model = defaults.opus_mapped_model;
  target.sonnet_mapped_model = defaults.sonnet_mapped_model;
  target.haiku_mapped_model = defaults.haiku_mapped_model;
  target.exact_model_mappings = [];
}
