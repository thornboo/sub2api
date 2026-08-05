ALTER TABLE account_model_protocol_capabilities
    DROP CONSTRAINT IF EXISTS account_model_protocol_capability_protocol_check;

ALTER TABLE account_model_protocol_capabilities
    ADD CONSTRAINT account_model_protocol_capability_protocol_check
    CHECK (protocol IN (
        'anthropic_messages',
        'openai_chat_completions',
        'openai_responses',
        'openai_embeddings',
        'openai_images',
        'openai_live',
        'batch_images',
        'grok_video',
        'gemini_native'
    ));
