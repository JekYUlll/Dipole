ALTER TABLE agent_tasks
    DROP FOREIGN KEY fk_agent_task_subscription,
    DROP INDEX idx_agent_task_subscription,
    DROP COLUMN trigger_subscription_uuid;

DROP TABLE IF EXISTS agent_event_subscriptions;
