import { describe, expect, it } from "vitest";

import {
  applyOpenAIMessagesDispatchPreset,
  createDefaultMessagesDispatchFormState,
  isMessagesDispatchModelOutsideCandidates,
  messagesDispatchConfigToFormState,
  messagesDispatchFormStateToConfig,
  resetMessagesDispatchFormState,
  type MessagesDispatchFormState,
} from "../groupsMessagesDispatch";

describe("groupsMessagesDispatch", () => {
  it("returns the expected default form state", () => {
    expect(createDefaultMessagesDispatchFormState()).toEqual({
      allow_messages_dispatch: false,
      family_mapping_mode: "passthrough",
      opus_mapped_model: "",
      sonnet_mapped_model: "",
      haiku_mapped_model: "",
      exact_model_mappings: [],
    });
  });

  it("sanitizes exact model mapping rows when converting to config", () => {
    const config = messagesDispatchFormStateToConfig({
      allow_messages_dispatch: true,
      family_mapping_mode: "custom",
      opus_mapped_model: " gpt-5.4 ",
      sonnet_mapped_model: "gpt-5.3-codex",
      haiku_mapped_model: " gpt-5.4-mini ",
      exact_model_mappings: [
        {
          claude_model: " claude-sonnet-4-5-20250929 ",
          target_model: " gpt-5.2 ",
        },
        { claude_model: "", target_model: "gpt-5.4" },
        { claude_model: "claude-opus-4-6", target_model: " " },
      ],
    });

    expect(config).toEqual({
      family_mapping_mode: "custom",
      opus_mapped_model: "gpt-5.4",
      sonnet_mapped_model: "gpt-5.3-codex",
      haiku_mapped_model: "gpt-5.4-mini",
      exact_model_mappings: {
        "claude-sonnet-4-5-20250929": "gpt-5.2",
      },
    });
  });

  it("hydrates form state from api config", () => {
    expect(
      messagesDispatchConfigToFormState({
        opus_mapped_model: "gpt-5.4",
        sonnet_mapped_model: "gpt-5.2",
        haiku_mapped_model: "gpt-5.4-mini",
        exact_model_mappings: {
          "claude-opus-4-6": "gpt-5.4",
          "claude-haiku-4-5-20251001": "gpt-5.4-mini",
        },
      }),
    ).toEqual({
      allow_messages_dispatch: false,
      family_mapping_mode: "legacy",
      opus_mapped_model: "gpt-5.4",
      sonnet_mapped_model: "gpt-5.2",
      haiku_mapped_model: "gpt-5.4-mini",
      exact_model_mappings: [
        {
          claude_model: "claude-haiku-4-5-20251001",
          target_model: "gpt-5.4-mini",
        },
        { claude_model: "claude-opus-4-6", target_model: "gpt-5.4" },
      ],
    });
  });

  it("round-trips explicit pass-through without restoring GPT defaults", () => {
    const state = messagesDispatchConfigToFormState({
      family_mapping_mode: "passthrough",
      opus_mapped_model: "",
      sonnet_mapped_model: "",
      haiku_mapped_model: "",
    });

    expect(state).toEqual({
      allow_messages_dispatch: false,
      family_mapping_mode: "passthrough",
      opus_mapped_model: "",
      sonnet_mapped_model: "",
      haiku_mapped_model: "",
      exact_model_mappings: [],
    });
    expect(messagesDispatchFormStateToConfig(state)).toEqual({
      family_mapping_mode: "passthrough",
      opus_mapped_model: "",
      sonnet_mapped_model: "",
      haiku_mapped_model: "",
      exact_model_mappings: {},
    });
  });

  it("applies the OpenAI preset as an explicit custom mapping", () => {
    const state = createDefaultMessagesDispatchFormState();

    applyOpenAIMessagesDispatchPreset(state);

    expect(state).toMatchObject({
      family_mapping_mode: "custom",
      opus_mapped_model: "gpt-5.4",
      sonnet_mapped_model: "gpt-5.3-codex",
      haiku_mapped_model: "gpt-5.4-mini",
    });
  });

  it("warns only when a non-empty model is outside known group candidates", () => {
    const candidates = ["minimax-m2.5", "minimax-m2.7"];

    expect(
      isMessagesDispatchModelOutsideCandidates("minimax-m2.7", candidates),
    ).toBe(false);
    expect(
      isMessagesDispatchModelOutsideCandidates(" MiniMax-M2.7 ", candidates),
    ).toBe(true);
    expect(isMessagesDispatchModelOutsideCandidates("", candidates)).toBe(
      false,
    );
    expect(
      isMessagesDispatchModelOutsideCandidates("custom-model", []),
    ).toBe(false);
  });

  it("resets mutable form state when platform switches away from openai", () => {
    const state: MessagesDispatchFormState = {
      allow_messages_dispatch: true,
      family_mapping_mode: "custom",
      opus_mapped_model: "gpt-5.2",
      sonnet_mapped_model: "gpt-5.4",
      haiku_mapped_model: "gpt-5.1",
      exact_model_mappings: [
        { claude_model: "claude-opus-4-6", target_model: "gpt-5.4" },
      ],
    };

    resetMessagesDispatchFormState(state);

    expect(state).toEqual({
      allow_messages_dispatch: false,
      family_mapping_mode: "passthrough",
      opus_mapped_model: "",
      sonnet_mapped_model: "",
      haiku_mapped_model: "",
      exact_model_mappings: [],
    });
  });
});
