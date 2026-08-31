import pathlib
import unittest


ROOT = pathlib.Path(__file__).resolve().parents[1]
SCRIPT = (ROOT / "scripts" / "smoke-microservice-isolated-images.sh").read_text(encoding="utf-8")


class MicroserviceMessageRecoverySmokeContractTest(unittest.TestCase):
    def test_restart_is_explicit_and_limited_to_message_path_services(self):
        self.assertIn("message_restart_service=${SMOKE_MESSAGE_RESTART_SERVICE:-}", SCRIPT)
        self.assertIn("core|gateway|message|sync", SCRIPT)
        self.assertIn("SMOKE_MESSAGE_RESTART_SERVICE requires SMOKE_MESSAGE_FLOW=1", SCRIPT)

    def test_service_exec_has_a_bounded_timeout(self):
        self.assertIn("exec_timeout_seconds=${SMOKE_EXEC_TIMEOUT_SECONDS:-20}", SCRIPT)
        self.assertIn('compose_args=(-p "${project}")', SCRIPT)
        self.assertIn('docker compose "${compose_args[@]}" ps -q "${service}"', SCRIPT)
        self.assertIn('timeout --foreground -k 5s "${exec_timeout_seconds}" docker exec "${container_id}" "$@"', SCRIPT)
        self.assertNotIn('docker compose "${compose_args[@]}" exec', SCRIPT)

    def test_readiness_failures_emit_bounded_service_diagnostics(self):
        self.assertIn('service readiness did not converge: %s live=%s ready=%s', SCRIPT)
        self.assertIn('gateway health did not converge: %s', SCRIPT)
        self.assertIn('compose ps "${service}" >&2 || true', SCRIPT)
        self.assertIn('compose logs --tail 80 "${service}" >&2 || true', SCRIPT)

    def test_restart_happens_after_first_persist_and_before_idempotent_replay(self):
        first_send = SCRIPT.index("send_message 1")
        persisted = SCRIPT.index('initial message persistence did not converge: message=%s')
        restart = SCRIPT.index('restart_message_service "${message_restart_service}"')
        replay = SCRIPT.index("send_message 2")
        self.assertLess(first_send, persisted)
        self.assertLess(persisted, restart)
        self.assertLess(restart, replay)

    def test_receipt_binds_message_and_projection_side_effect_counts(self):
        self.assertIn('[[ "${message_count}" == "1" && "${outbox_count}" == "1" && "${inbox_count}" == "1" ]]', SCRIPT)
        self.assertIn("message recovery side effects did not converge", SCRIPT)
        self.assertIn("message_recovery:{restart_service:$message_restart_service", SCRIPT)
        self.assertIn("outbox_count:($outbox_count|tonumber)", SCRIPT)
        self.assertIn("inbox_count:($inbox_count|tonumber)", SCRIPT)

    def test_initial_persistence_failure_reports_ws_client_evidence(self):
        self.assertIn('initial message persistence did not converge: message=%s', SCRIPT)
        self.assertIn('tail -n 80 "${wscli_log}" >&2 || true', SCRIPT)


if __name__ == "__main__":
    unittest.main()
