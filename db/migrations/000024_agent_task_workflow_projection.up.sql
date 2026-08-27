ALTER TABLE agent_tasks
    ADD COLUMN workflow_id VARCHAR(255) NULL AFTER goal,
    ADD COLUMN workflow_run_id VARCHAR(64) NULL AFTER workflow_id,
    ADD COLUMN workflow_status VARCHAR(32) NULL AFTER workflow_run_id,
    ADD COLUMN workflow_revision BIGINT UNSIGNED NULL AFTER workflow_status,
    ADD COLUMN workflow_updated_at DATETIME(3) NULL AFTER workflow_revision,
    ADD KEY idx_agent_task_workflow_status (workflow_status, workflow_updated_at),
    ADD CONSTRAINT chk_agent_task_workflow_projection CHECK (
        (workflow_id IS NULL AND workflow_run_id IS NULL AND workflow_status IS NULL AND workflow_revision IS NULL AND workflow_updated_at IS NULL)
        OR
        (workflow_id IS NOT NULL AND workflow_run_id IS NOT NULL AND workflow_status IS NOT NULL AND workflow_revision IS NOT NULL AND workflow_updated_at IS NOT NULL)
    ),
    ADD CONSTRAINT chk_agent_task_workflow_status CHECK (
        workflow_status IS NULL OR workflow_status IN ('created', 'running', 'waiting_input', 'waiting_approval', 'completed', 'failed', 'cancelled')
    );
