ALTER TABLE agent_model_calls
    ADD COLUMN output_json JSON NULL AFTER status;
