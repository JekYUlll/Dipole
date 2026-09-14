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
async function until(fn, label) {
  const deadline = Date.now() + 120000;
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
try {
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
} finally {
  clearInterval(heartbeat);
  socket.close();
}
