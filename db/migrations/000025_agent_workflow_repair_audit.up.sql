CREATE TABLE IF NOT EXISTS agent_workflow_repair_operator_grants (
    user_uuid VARCHAR(24) NOT NULL,
    can_propose BOOLEAN NOT NULL DEFAULT FALSE,
    can_approve BOOLEAN NOT NULL DEFAULT FALSE,
    granted_by_uuid VARCHAR(24) NOT NULL,
    valid_from DATETIME(3) NOT NULL,
    expires_at DATETIME(3) NULL,
    revoked_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (user_uuid),
    CONSTRAINT chk_agent_repair_operator_capability CHECK (can_propose OR can_approve)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS agent_workflow_repair_proposals (
    proposal_uuid VARCHAR(71) NOT NULL,
    task_uuid VARCHAR(64) NOT NULL,
    outcome VARCHAR(32) NOT NULL,
    action VARCHAR(64) NOT NULL,
    proposer_uuid VARCHAR(24) NOT NULL,
    ticket_ref VARCHAR(128) NOT NULL,
    reason TEXT NOT NULL,
    projected_json JSON NULL,
    temporal_json JSON NOT NULL,
    evidence_sha256 CHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL,
    required_approvals TINYINT UNSIGNED NOT NULL DEFAULT 2,
    proposed_at DATETIME(3) NOT NULL,
    expires_at DATETIME(3) NOT NULL,
    decided_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (proposal_uuid),
    UNIQUE KEY idx_agent_repair_evidence (evidence_sha256),
    KEY idx_agent_repair_task_status (task_uuid, status),
    KEY idx_agent_repair_expiry (status, expires_at),
    CONSTRAINT fk_agent_repair_task FOREIGN KEY (task_uuid) REFERENCES agent_tasks(task_uuid),
    CONSTRAINT chk_agent_repair_outcome CHECK (outcome IN ('missing', 'stale', 'ahead', 'conflict')),
    CONSTRAINT chk_agent_repair_action CHECK (action = 'reproject_from_temporal'),
    CONSTRAINT chk_agent_repair_status CHECK (status IN ('proposed', 'approved', 'rejected', 'expired')),
    CONSTRAINT chk_agent_repair_approvals CHECK (required_approvals = 2)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS agent_workflow_repair_decisions (
    proposal_uuid VARCHAR(71) NOT NULL,
    approver_uuid VARCHAR(24) NOT NULL,
    decision VARCHAR(16) NOT NULL,
    decided_at DATETIME(3) NOT NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (proposal_uuid, approver_uuid),
    KEY idx_agent_repair_decision (proposal_uuid, decision),
    CONSTRAINT fk_agent_repair_decision_proposal FOREIGN KEY (proposal_uuid) REFERENCES agent_workflow_repair_proposals(proposal_uuid),
    CONSTRAINT chk_agent_repair_decision CHECK (decision IN ('approved', 'rejected'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
