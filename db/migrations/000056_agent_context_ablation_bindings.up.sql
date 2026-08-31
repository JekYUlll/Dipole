CREATE TABLE agent_context_ablation_bindings (
    experiment_uuid VARCHAR(64) NOT NULL,
    case_sha256 CHAR(64) NOT NULL,
    condition_name VARCHAR(16) NOT NULL,
    task_uuid VARCHAR(64) NOT NULL,
    run_uuid VARCHAR(64) NOT NULL,
    candidate_version VARCHAR(128) NOT NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (experiment_uuid, case_sha256, condition_name),
    UNIQUE KEY uq_agent_context_ablation_run (run_uuid),
    KEY idx_agent_context_ablation_experiment (experiment_uuid, created_at),
    CONSTRAINT fk_agent_context_ablation_task FOREIGN KEY (task_uuid) REFERENCES agent_tasks(task_uuid) ON DELETE RESTRICT,
    CONSTRAINT fk_agent_context_ablation_run FOREIGN KEY (run_uuid) REFERENCES agent_runs(run_uuid) ON DELETE RESTRICT,
    CONSTRAINT chk_agent_context_ablation_condition CHECK (condition_name IN ('baseline', 'retrieval', 'memory')),
    CONSTRAINT chk_agent_context_ablation_case_hash CHECK (case_sha256 REGEXP '^[a-f0-9]{64}$')
);
