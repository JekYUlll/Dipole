// Run against the existing local experience stack. No mocks or extra services.
import { execFileSync } from 'node:child_process';
import { randomUUID } from 'node:crypto';
import assert from 'node:assert/strict';

const base = process.env.DIPOLE_SMOKE_URL ?? 'http://127.0.0.1:8080';
const project = process.env.COMPOSE_PROJECT_NAME ?? 'dipole-agent-finalization';
const ai = 'UAI000000000000000001';
const sleep = ms => new Promise(resolve => setTimeout(resolve, ms));
const sql = query => execFileSync('docker', ['exec', '-i', `${project}-mysql-1`, 'sh', '-c',
  'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql -uroot dipole -N -B'], { input: query, encoding: 'utf8' }).trim();
const quote = value => "'" + String(value).replaceAll("'", "''") + "'";
let token;
async function api(path, body, acceptedStatus) {
  const response = await fetch(base + '/api/v1' + path, {
    signal: AbortSignal.timeout(10000),
    method: body === undefined ? 'GET' : 'POST',
    headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}) },
    ...(body === undefined ? {} : { body: JSON.stringify(body) })
  });
  const result = await response.json();
  if (response.status === acceptedStatus) return result;
  if (!response.ok || (result.code !== undefined && result.code !== 0)) throw new Error(`${path}: ${response.status} ${result.message ?? result.error}`);
  return result.data ?? result;
}
async function until(fn, label, timeoutMs = 120000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    const value = await fn();
    if (value) return value;
    await sleep(1000);
  }
  throw new Error(`Timeout: ${label}`);
}
const registration = await api('/auth/register', {
  nickname: 'Agent smoke', telephone: '19' + String(Date.now()).slice(-9), password: randomUUID().slice(0, 24)
});
token = registration.token;
const owner = registration.user.uuid;
const socket = new WebSocket(base.replace(/^http/, 'ws') + '/api/v1/ws?token=' + encodeURIComponent(token));
const received = [];
socket.addEventListener('message', e => received.push(JSON.parse(e.data)));
await new Promise((resolve, reject) => { socket.addEventListener('open', resolve, { once: true }); socket.addEventListener('error', reject, { once: true }); });
const heartbeat = setInterval(() => { if (socket.readyState === WebSocket.OPEN) socket.send(JSON.stringify({ type: 'ping' })); }, 20000);
async function send(content, target = ai, expectTask = true) {
  const id = randomUUID();
  socket.send(JSON.stringify({ type: 'chat.send', data: { target_uuid: target, content, client_message_id: id } }));
  if (!expectTask) {
    return until(() => sql(`SELECT uuid FROM messages WHERE sender_uuid=${quote(owner)} AND client_message_id=${quote(id)} LIMIT 1`), 'Message persistence');
  }
  const task = await until(() => sql(`SELECT t.task_uuid FROM agent_tasks t JOIN messages m ON m.uuid=t.trigger_ref WHERE m.sender_uuid=${quote(owner)} AND m.client_message_id=${quote(id)} LIMIT 1`), 'Task admission');
  console.log(`Task ${task}`);
  return task;
}
const status = task => {
  const value = sql(`SELECT COALESCE(workflow_status,status) FROM agent_tasks WHERE task_uuid=${quote(task)}`);
  if (value === 'failed') throw new Error(`Task failed: ${task}`);
  return value;
};
let faultTrigger;
let workerStopped = false;
try {
  if (!process.argv.includes('--report-only')) {
  const direct = await send('Hello, please reply briefly.');
  await until(() => status(direct) === 'completed', 'Direct completed');
  assert(received.some(e => e.data?.from_uuid === ai), 'AI reply delivered over WebSocket');
  console.log('PASS Direct');
  for (const decision of ['denied', 'approved']) {
    const content = `Deployment notice ${randomUUID()}`;
    const task = await send(`Please publish a system message in this conversation with exactly this text: ${content}`);
    await until(() => status(task) === 'waiting_approval', 'Natural-language approval');
    const approval = sql(`SELECT approval_uuid FROM agent_approvals WHERE task_uuid=${quote(task)} LIMIT 1`);
    const count = () => Number(sql(`SELECT COUNT(*) FROM messages WHERE sender_uuid=${quote(ai)} AND target_uuid=${quote(owner)} AND content=${quote(content)}`));
    assert.equal(count(), 0);
    if (decision === 'approved') execFileSync('docker', ['restart', `${project}-agent-1`], { stdio: 'ignore' });
    await until(async () => { try { await api(`/agent/tasks/${task}`); return true; } catch { return false; } }, 'Runtime ready');
    await api(`/agent/tasks/${task}/approvals/${approval}`, { decision });
    await until(() => ['completed', 'cancelled'].includes(status(task)), 'Approval resolved');
    assert.equal(count(), decision === 'approved' ? 1 : 0);
    if (decision === 'approved') {
      await api(`/agent/tasks/${task}/approvals/${approval}`, { decision }, 409);
      assert.equal(count(), 1);
    }
    console.log(`PASS ${decision === 'approved' ? 'Approval + WorkerRecovery + DuplicateApproval' : 'Deny'} task=${task} messages=${count()}`);
  }
  const recoveryText = `Recovery notice ${randomUUID()}`;
  const recoveryTask = await send(`Please publish a system message in this conversation with exactly this text: ${recoveryText}`);
  await until(() => status(recoveryTask) === 'waiting_approval', 'Recovery approval');
  const recoveryApproval = sql(`SELECT approval_uuid FROM agent_approvals WHERE task_uuid=${quote(recoveryTask)} LIMIT 1`);
  const recoveryCount = () => Number(sql(`SELECT COUNT(*) FROM messages WHERE sender_uuid=${quote(ai)} AND target_uuid=${quote(owner)} AND content=${quote(recoveryText)}`));
  faultTrigger = `dipole_smoke_${randomUUID().replaceAll('-', '')}`;
  // Fail only this task's real audit update, after its real message transaction commits.
  sql(`DELIMITER $$
    CREATE TRIGGER ${faultTrigger} BEFORE UPDATE ON agent_tool_invocations FOR EACH ROW
    BEGIN IF NEW.task_uuid=${quote(recoveryTask)} AND NEW.status='completed' THEN
      DO SLEEP(15); SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='smoke audit interruption';
    END IF; END$$
    DELIMITER ;`);
  await api(`/agent/tasks/${recoveryTask}/approvals/${recoveryApproval}`, { decision: 'approved' });
  await until(() => recoveryCount() === 1, 'Message committed before audit failure');
  assert.equal(sql(`SELECT status FROM agent_approvals WHERE approval_uuid=${quote(recoveryApproval)}`), 'consumed');
  assert.equal(sql(`SELECT status FROM agent_tool_invocations WHERE task_uuid=${quote(recoveryTask)} LIMIT 1`), 'running');
  execFileSync('docker', ['stop', '--time', '0', `${project}-agent-1`], { stdio: 'ignore' });
  workerStopped = true;
  sql(`DROP TRIGGER ${faultTrigger}`);
  faultTrigger = undefined;
  execFileSync('docker', ['start', `${project}-agent-1`], { stdio: 'ignore' });
  workerStopped = false;
  await until(() => status(recoveryTask) === 'completed', 'Post-consumption recovery', 180000);
  assert.equal(recoveryCount(), 1);
  assert.equal(sql(`SELECT COUNT(*) FROM agent_tool_invocations WHERE task_uuid=${quote(recoveryTask)} AND status='completed'`), '1');
  assert.equal(sql(`SELECT COUNT(*) FROM agent_tool_invocations WHERE task_uuid=${quote(recoveryTask)}`), '1');
  assert.equal(sql(`SELECT COUNT(*) FROM agent_model_calls c JOIN agent_model_runs r ON r.run_uuid=c.run_uuid WHERE r.task_uuid=${quote(recoveryTask)} AND c.status='completed'`), '1');
  console.log(`PASS PostConsumptionRecovery task=${recoveryTask} messages=1 invocations=1`);
  const group = await api('/groups', { name: 'Agent experience', member_uuids: [ai] });
  const marker = 'Cassandra' + Date.now();
  const source = await send(`${marker}: decision is to keep MySQL as the default backend.`, group.uuid, false);
  await until(async () => JSON.stringify(await api(`/messages/search?q=${marker}`)).includes(source), 'Elasticsearch projection');
  assert.equal(sql(`SELECT COUNT(*) FROM agent_tasks WHERE trigger_ref=${quote(source)}`), '0');
  const mention = await send('@AI Summarize the decision above briefly.', group.uuid);
  await until(() => status(mention) === 'completed', 'Group mention');
  assert.equal(sql(`SELECT COUNT(*) FROM messages WHERE sender_uuid=${quote(ai)} AND target_uuid=${quote(group.uuid)}`), '1');
  console.log('PASS GroupMention + NoSelfTrigger');
  const retrieval = await send(`Use conversation.search to find the discussion containing ${marker} in my conversations and summarize the decision. Query exactly ${marker}.`);
  await until(() => status(retrieval) === 'completed', 'Retrieval completed');
  assert(Number(sql(`SELECT COUNT(*) FROM agent_shadow_steps WHERE task_uuid=${quote(retrieval)} AND capability_id='conversation.search' AND status='completed'`)) > 0);
  assert.equal(sql(`SELECT COUNT(*) FROM agent_model_runs WHERE task_uuid=${quote(retrieval)} AND stage='answer' AND status='completed'`), '1');
  assert(sql(`SELECT output_json FROM agent_shadow_steps WHERE task_uuid=${quote(retrieval)} AND capability_id='conversation.search'`).includes(source), 'Search evidence includes the seeded message');
  const history = await api(`/messages/direct/${ai}`);
  assert(JSON.stringify(history).includes('MySQL'), 'Evidence-based answer persisted in history');
  const sync = await api('/sync?after_seq=0&limit=100');
  assert(JSON.stringify(sync).includes('MySQL'), 'Answer is available through user sync');
  console.log(`PASS Retrieval task=${retrieval} source=${source}`);
  console.log('PASS History + Sync');

  const publishAt = Date.now() + 120000;
  const scheduled = [];
  const groupCount = () => Number(sql(`SELECT COUNT(*) FROM messages WHERE sender_uuid=${quote(ai)} AND target_uuid=${quote(group.uuid)}`));
  const beforeScheduled = groupCount();
  for (const decision of ['approved', 'denied', 'cancelled']) {
    const task = await send(`@AI /digest ${new Date(publishAt).toISOString()} Use conversation.search with query ${marker}. Summarize the MySQL decision briefly for this group.`, group.uuid);
    await until(() => status(task) === 'waiting_approval', 'Scheduled digest draft');
    const approval = sql(`SELECT approval_uuid FROM agent_approvals WHERE task_uuid=${quote(task)} AND capability_id='message.group_reply.send' LIMIT 1`);
    assert(approval, 'Group draft has a bound approval');
    assert(Number(sql(`SELECT COUNT(*) FROM agent_shadow_steps WHERE task_uuid=${quote(task)} AND capability_id='conversation.search' AND status='completed'`)) > 0);
    assert.equal(sql(`SELECT COUNT(*) FROM agent_model_runs WHERE task_uuid=${quote(task)} AND stage='answer' AND status='completed'`), '1');
    await api(`/agent/tasks/${task}/approvals/${approval}`, { decision: decision === 'denied' ? 'denied' : 'approved' });
    if (decision === 'cancelled') await api(`/agent/tasks/${task}/cancel`, { reason: 'Cancel scheduled smoke publication' });
    scheduled.push({ task, approval, decision });
  }
  assert(Date.now() < publishAt, 'All decisions occurred before publication');
  assert.equal(groupCount(), beforeScheduled, 'No premature group publication');
  execFileSync('docker', ['restart', `${project}-agent-1`], { stdio: 'ignore' });
  await sleep(Math.max(0, publishAt - Date.now()));
  for (const { task, approval, decision } of scheduled) {
    await until(() => status(task) === (decision === 'approved' ? 'completed' : 'cancelled'), 'Scheduled task terminal state');
    const count = Number(sql(`SELECT COUNT(*) FROM agent_tool_invocations t JOIN messages m ON m.uuid=t.action_resource_uuid WHERE t.task_uuid=${quote(task)} AND t.status='completed' AND m.target_uuid=${quote(group.uuid)}`));
    assert.equal(count, decision === 'approved' ? 1 : 0);
    if (decision === 'approved') {
      await api(`/agent/tasks/${task}/approvals/${approval}`, { decision: 'approved' }, 409);
      const message = sql(`SELECT m.uuid FROM messages m JOIN agent_tool_invocations t ON t.action_resource_uuid=m.uuid WHERE t.task_uuid=${quote(task)}`);
      assert(received.some(e => JSON.stringify(e).includes(message)), 'Scheduled group message reached WebSocket');
      assert(JSON.stringify(await api('/sync?after_seq=0&limit=100')).includes(message), 'Scheduled publication reached Sync');
    }
    console.log(`PASS ScheduledGroup ${decision} task=${task} messages=${count}`);
  }
  assert.equal(groupCount(), beforeScheduled + 1, 'Exactly one scheduled group publication');
  console.log('PASS ScheduledWorkerRecovery + NoEarlyDispatch + GroupSync');
  }
  const reportGroup = await api('/groups', { name: 'Collaboration report', member_uuids: [ai] });
  const reportMarker = `report-${Date.now()}`;
  await send(`${reportMarker}: MySQL remains the default. Migration delivery date is unknown; only the task owner can confirm it.`, reportGroup.uuid, false);
  for (const scenario of ['owner-input', 'deadline']) {
    const deadline = Date.now() + (scenario === 'deadline' ? 45000 : 180000);
    const task = await send(`@AI /report ${new Date(deadline).toISOString()} Read this conversation. Summarize ${reportMarker}, including the migration delivery date. Ask me to confirm the missing delivery date.`, reportGroup.uuid);
    await until(() => status(task) === 'waiting_input', 'Report owner question');
    let view = await api(`/agent/tasks/${task}`);
    assert.equal(view.pending.form.fields[0].id, 'answer', 'Real model requests missing information');
    const before = Number(sql(`SELECT COUNT(*) FROM messages WHERE sender_uuid=${quote(ai)} AND target_uuid=${quote(reportGroup.uuid)}`));
    execFileSync('docker', ['restart', `${project}-agent-1`], { stdio: 'ignore' });
    await until(async () => { try { view = await api(`/agent/tasks/${task}`); return true; } catch { return false; } }, 'Report worker recovery');
    if (scenario === 'owner-input') {
      await api(`/agent/tasks/${task}/inputs/${view.pending.requestId}`, { value: { answer: 'Confirmed delivery: Friday at 18:00; owner Alice.' } });
    }
    await until(async () => { view = await api(`/agent/tasks/${task}`); return view.pending?.form?.fields?.[0]?.id === 'content'; }, 'Report draft after input/deadline');
    assert.equal(status(task), 'waiting_input');
    assert.equal(Number(sql(`SELECT COUNT(*) FROM messages WHERE sender_uuid=${quote(ai)} AND target_uuid=${quote(reportGroup.uuid)}`)), before);
    assert.equal(sql(`SELECT COUNT(*) FROM agent_model_runs WHERE task_uuid=${quote(task)} AND stage='report_final' AND status='completed'`), '1');
    const edited = `Reviewed ${reportMarker}: MySQL default; delivery Friday at 18:00 (owner confirmed).`;
    await api(`/agent/tasks/${task}/inputs/${view.pending.requestId}`, { value: scenario === 'owner-input' ? { content: edited } : {} });
    await until(() => status(task) === 'waiting_approval', 'Report approval');
    view = await api(`/agent/tasks/${task}`);
    if (scenario === 'owner-input') assert(view.pending.summary.includes(edited));
    const decision = scenario === 'owner-input' ? 'approved' : 'denied';
    const approval = view.pending.approvalId;
    await api(`/agent/tasks/${task}/approvals/${approval}`, { decision });
    await until(() => status(task) === (decision === 'approved' ? 'completed' : 'cancelled'), 'Report terminal');
    const count = Number(sql(`SELECT COUNT(*) FROM messages WHERE sender_uuid=${quote(ai)} AND target_uuid=${quote(reportGroup.uuid)}`));
    assert.equal(count, before + (decision === 'approved' ? 1 : 0));
    assert.equal(sql(`SELECT COUNT(*) FROM agent_artifacts WHERE task_uuid=${quote(task)}`), '2');
    if (decision === 'approved') {
      await api(`/agent/tasks/${task}/approvals/${approval}`, { decision }, 409);
      assert.equal(sql(`SELECT COUNT(*) FROM messages WHERE sender_uuid=${quote(ai)} AND target_uuid=${quote(reportGroup.uuid)} AND content=${quote(edited)}`), '1');
    }
    console.log(`PASS CollaborationReport ${scenario} task=${task} artifacts=2 publication=${decision === 'approved' ? 1 : 0}`);
  }
} finally {
  try {
    if (faultTrigger) sql(`DROP TRIGGER IF EXISTS ${faultTrigger}`);
  } finally {
    try {
      if (workerStopped) execFileSync('docker', ['start', `${project}-agent-1`], { stdio: 'ignore' });
    } finally {
      clearInterval(heartbeat);
      socket.close();
    }
  }
}
