/**
 * Dipole IM 压测脚本 v2
 *
 * 场景：
 *   1. direct_msg  — 单聊消息收发延迟（跨节点：发送方连 node1，接收方连 node2）
 *   2. concurrent  — 多人并发在线持续收发
 *   3. group_blast — 大群广播
 *
 * 运行：
 *   k6 run --out json=results/raw.json scripts/bench/bench.js
 */

import http from "k6/http";
import ws   from "k6/ws";
import exec from "k6/execution";
import { sleep } from "k6";
import { Trend, Counter, Rate } from "k6/metrics";

const BASE_URL   = __ENV.BASE_URL   || "http://localhost:8081";
const NODE1_WS   = __ENV.NODE1_WS   || "ws://localhost:8081";
const NODE2_WS   = __ENV.NODE2_WS   || "ws://localhost:8082";
const USER_COUNT = parseInt(__ENV.USER_COUNT || "50");
const GROUP_SIZE = parseInt(__ENV.GROUP_SIZE || "20");
const IDLE_SECONDS = parseInt(__ENV.IDLE_SECONDS || "120");
const RUN_ID = __ENV.RUN_ID || String(Date.now());
const PHONE_PREFIX = __ENV.PHONE_PREFIX || "138";
const SCENARIO_FILTER = __ENV.SCENARIO_FILTER || "";
const DIRECT_SEND_COUNT = parseInt(__ENV.DIRECT_SEND_COUNT || "5");
const CONCURRENT_SEND_COUNT = parseInt(__ENV.CONCURRENT_SEND_COUNT || "8");

// ── 自定义指标 ───────────────────────────────────────────────────────────────

const msgLatency      = new Trend("msg_e2e_latency_ms", true);
const msgAttempted    = new Counter("msg_attempted_total");
const msgAccepted     = new Counter("msg_accepted_total");
const msgRejected     = new Counter("msg_rejected_total");
const msgSent         = new Counter("msg_sent_total");
const msgReceived     = new Counter("msg_received_total");
const msgDeliveryRate = new Rate("msg_delivery_rate");
const msgExpected     = new Counter("msg_expected_receipts_total");

// ── 测试阶段 ─────────────────────────────────────────────────────────────────

const allScenarios = {
    direct_msg: {
      executor: "per-vu-iterations",
      vus: 20,
      iterations: 1,
      maxDuration: "20s",
      env: { SCENARIO: "direct_msg" },
      tags: { scenario: "direct_msg" },
    },
    concurrent: {
      executor: "per-vu-iterations",
      vus: USER_COUNT,
      iterations: 1,
      maxDuration: "20s",
      startTime: "25s",
      env: { SCENARIO: "concurrent" },
      tags: { scenario: "concurrent" },
    },
    group_blast: {
      executor: "per-vu-iterations",
      vus: GROUP_SIZE,
      iterations: 1,
      maxDuration: "20s",
      startTime: "50s",
      env: { SCENARIO: "group_blast" },
      tags: { scenario: "group_blast" },
    },
};

const selectedScenarios = SCENARIO_FILTER
  ? { [SCENARIO_FILTER]: { ...allScenarios[SCENARIO_FILTER], startTime: "0s" } }
  : allScenarios;

if (SCENARIO_FILTER && !allScenarios[SCENARIO_FILTER]) {
  throw new Error(`unknown SCENARIO_FILTER: ${SCENARIO_FILTER}`);
}

export const options = {
  scenarios: selectedScenarios,
  thresholds: {
    "http_req_failed{name:login}":   ["rate<0.01"],
  },
};

// ── HTTP 工具 ────────────────────────────────────────────────────────────────

function post(path, body, token) {
  const headers = { "Content-Type": "application/json" };
  if (token) headers["Authorization"] = `Bearer ${token}`;
  return http.post(`${BASE_URL}${path}`, JSON.stringify(body), { headers, tags: { name: path.split("/").pop() } });
}

function get(path, token, params) {
  const headers = { Authorization: `Bearer ${token}` };
  const qs = params ? "?" + Object.entries(params).map(([k,v]) => `${k}=${v}`).join("&") : "";
  return http.get(`${BASE_URL}${path}${qs}`, { headers, tags: { name: path.split("/").pop() } });
}

function patch(path, body, token) {
  const headers = { "Content-Type": "application/json", Authorization: `Bearer ${token}` };
  return http.patch(`${BASE_URL}${path}`, JSON.stringify(body), { headers, tags: { name: "patch" } });
}

function phone(i) { return `${PHONE_PREFIX}${String(i).padStart(8, "0")}`; }

function registerUser(i) {
  const tel = phone(i);
  const res = http.post(
    `${BASE_URL}/api/v1/auth/register`,
    JSON.stringify({ nickname: `bench_${i}`, telephone: tel, password: "bench123456" }),
    {
      headers: { "Content-Type": "application/json" },
      tags: { name: "register" },
      responseCallback: http.expectedStatuses(200, 409),
    },
  );
  return res.status === 200 || res.status === 409;
}

function loginUser(i) {
  const res = post("/api/v1/auth/login", { telephone: phone(i), password: "bench123456" }, null);
  if (res.status !== 200) return null;
  try { return JSON.parse(res.body).data; } catch (_) { return null; }
}

function getUserUUID(token, tel) {
  const res = get("/api/v1/users", token, { keyword: tel, limit: 1 });
  if (res.status !== 200) return null;
  try {
    const list = JSON.parse(res.body).data;
    return list && list.length > 0 ? list[0].uuid : null;
  } catch (_) { return null; }
}

function makeFriends(tokenA, uuidA, tokenB, uuidB) {
  // A 申请加 B
  const applyRes = post("/api/v1/contacts/applications", { target_uuid: uuidB, message: "bench" }, tokenA);
  if (applyRes.status !== 200) return; // 已是好友或已有申请

  // B 查收到的申请
  const listRes = get("/api/v1/contacts/applications", tokenB, { box: "incoming" });
  if (listRes.status !== 200) return;
  let appId = null;
  try {
    const apps = JSON.parse(listRes.body).data || [];
    const app = apps.find(a => a.applicant && a.applicant.uuid === uuidA && a.status === 0);
    if (app) appId = app.id;
  } catch (_) { return; }
  if (!appId) return;

  // B 接受
  http.patch(
    `${BASE_URL}/api/v1/contacts/applications/${appId}`,
    JSON.stringify({ action: "accept" }),
    { headers: { "Content-Type": "application/json", Authorization: `Bearer ${tokenB}` }, tags: { name: "friend_accept" } }
  );
}

function createGroup(token, memberUUIDs) {
  const res = post("/api/v1/groups", { name: "bench-group", member_uuids: memberUUIDs }, token);
  if (res.status !== 200) return null;
  try { return JSON.parse(res.body).data.uuid; } catch (_) { return null; }
}

// ── setup：预建用户、好友关系、群组 ──────────────────────────────────────────

export function setup() {
  const totalUsers = SCENARIO_FILTER === "direct_msg"
    ? 20
    : SCENARIO_FILTER === "concurrent"
      ? USER_COUNT
      : SCENARIO_FILTER === "group_blast"
        ? GROUP_SIZE
        : USER_COUNT + GROUP_SIZE + 10;
  console.log(`[setup] registering ${totalUsers} users...`);

  // 注册所有用户
  for (let i = 0; i < totalUsers; i++) {
    registerUser(i);
  }

  // 登录所有用户，获取 token 和 uuid
  const sessions = [];
  for (let i = 0; i < totalUsers; i++) {
    const data = loginUser(i);
    if (!data) { sessions.push(null); continue; }
    const uuid = data.user ? data.user.uuid : getUserUUID(data.token, phone(i));
    sessions.push({ token: data.token, uuid });
  }

  const loggedIn = sessions.filter(Boolean).length;
  console.log(`[setup] logged in ${loggedIn}/${totalUsers} users`);
  if (loggedIn !== totalUsers) {
    throw new Error(`setup login gate failed: ${loggedIn}/${totalUsers}`);
  }

  // 建立 direct_msg 场景的好友关系（前 20 对）
  if (!SCENARIO_FILTER || SCENARIO_FILTER === "direct_msg") {
    const directPairs = Math.min(10, Math.floor(USER_COUNT / 2));
    console.log(`[setup] creating ${directPairs} friend pairs for direct_msg...`);
    for (let i = 0; i < directPairs; i++) {
      const sIdx = i * 2, rIdx = i * 2 + 1;
      if (!sessions[sIdx] || !sessions[rIdx]) continue;
      makeFriends(sessions[sIdx].token, sessions[sIdx].uuid, sessions[rIdx].token, sessions[rIdx].uuid);
    }
  }

  // 建立 concurrent 场景的好友关系（USER_COUNT 个用户两两相邻）
  if (!SCENARIO_FILTER || SCENARIO_FILTER === "concurrent") {
    console.log(`[setup] creating friend pairs for concurrent...`);
    for (let i = 0; i < USER_COUNT - 1; i++) {
      if (!sessions[i] || !sessions[i+1]) continue;
      makeFriends(sessions[i].token, sessions[i].uuid, sessions[i+1].token, sessions[i+1].uuid);
    }
    if (USER_COUNT > 1) {
      makeFriends(
        sessions[USER_COUNT - 1].token,
        sessions[USER_COUNT - 1].uuid,
        sessions[0].token,
        sessions[0].uuid,
      );
    }
  }

  // 建立 group_blast 场景的群组
  const groupOffset = SCENARIO_FILTER === "group_blast" ? 0 : USER_COUNT + 5;
  const groupMemberUUIDs = [];
  for (let i = groupOffset + 1; i < groupOffset + GROUP_SIZE && i < sessions.length; i++) {
    if (sessions[i]) groupMemberUUIDs.push(sessions[i].uuid);
  }
  let groupUUID = null;
  if ((!SCENARIO_FILTER || SCENARIO_FILTER === "group_blast") && sessions[groupOffset]) {
    console.log(`[setup] creating group with ${groupMemberUUIDs.length} members...`);
    groupUUID = createGroup(sessions[groupOffset].token, groupMemberUUIDs);
    console.log(`[setup] group created: ${groupUUID}`);
  }
  if ((!SCENARIO_FILTER || SCENARIO_FILTER === "group_blast") && !groupUUID) {
    throw new Error("setup group gate failed");
  }

  console.log("[setup] done.");
  return { sessions, groupUUID, groupOffset };
}

// ── 场景入口 ─────────────────────────────────────────────────────────────────

export default function (data) {
  if (__ITER > 0) {
    sleep(IDLE_SECONDS);
    return;
  }

  const scenario = __ENV.SCENARIO;
  const vuIndex = Number(exec.scenario.iterationInInstance);
  if (!data || !data.sessions) return;

  if (scenario === "direct_msg") {
    runDirectMsg(vuIndex, data.sessions);
  } else if (scenario === "concurrent") {
    runConcurrent(vuIndex, data.sessions);
  } else if (scenario === "group_blast") {
    runGroupBlast(vuIndex, data);
  }
}

function wsURL(token, nodeWS) {
  return `${nodeWS}/api/v1/ws?token=${token}`;
}

function sendMsg(socket, targetUUID, content, clientMessageID) {
  socket.send(JSON.stringify({
    type: "chat.send",
    data: { target_uuid: targetUUID, content, client_message_id: clientMessageID },
  }));
  msgAttempted.add(1);
}

function recordCommandResult(raw, commandPrefix, seenCommandResults, expectedReceipts) {
  try {
    const evt = JSON.parse(raw);
    const clientMessageID = evt.data && evt.data.client_message_id || "";
    if (!clientMessageID.startsWith(commandPrefix) || seenCommandResults.has(clientMessageID)) return;
    if (evt.type === "chat.sent") {
      seenCommandResults.add(clientMessageID);
      msgAccepted.add(1);
      msgSent.add(1);
      msgExpected.add(expectedReceipts);
    } else if (evt.type === "error" && evt.data?.request_type === "chat.send") {
      seenCommandResults.add(clientMessageID);
      msgRejected.add(1);
    }
  } catch (_) {}
}

// ── 场景1：单聊延迟 ──────────────────────────────────────────────────────────

function runDirectMsg(vuIndex, sessions) {
  const pairIdx     = Math.floor(vuIndex / 2);
  const isSender    = vuIndex % 2 === 0;
  const senderIdx   = (pairIdx * 2)     % sessions.length;
  const receiverIdx = (pairIdx * 2 + 1) % sessions.length;

  const mySession   = isSender ? sessions[senderIdx]   : sessions[receiverIdx];
  const peerSession = isSender ? sessions[receiverIdx]  : sessions[senderIdx];
  if (!mySession || !peerSession) {
    sleep(IDLE_SECONDS);
    return;
  }

  const nodeWS = isSender ? NODE1_WS : NODE2_WS;

  if (isSender) {
    sleep(1); // 等接收方先连上
    ws.connect(wsURL(mySession.token, nodeWS), {}, function (socket) {
      let i = 0;
      const commandPrefix = `${RUN_ID}-direct-${vuIndex}-`;
      const seenCommandResults = new Set();
      socket.on("open", () => {
        // 用 setInterval 替代 open 里的 sleep，保持事件循环活跃
        socket.setInterval(() => {
          if (i >= DIRECT_SEND_COUNT) return;
          const sentAt = Date.now();
          sendMsg(socket, peerSession.uuid, `bench:${RUN_ID}:${sentAt}`, `${RUN_ID}-direct-${vuIndex}-${i}`);
          i++;
        }, 500);
      });
      socket.on("message", (raw) => recordCommandResult(raw, commandPrefix, seenCommandResults, 1));
      socket.on("error", () => {});
      socket.setTimeout(() => { socket.close(); }, 12000);
    });
  } else {
    ws.connect(wsURL(mySession.token, nodeWS), {}, function (socket) {
      socket.on("message", (raw) => {
        try {
          const evt = JSON.parse(raw);
          if (evt.type === "chat.message") {
            const m = (evt.data && evt.data.content || "").match(/^bench:([^:]+):(\d+)$/);
            if (m && m[1] === RUN_ID) {
              msgLatency.add(Date.now() - parseInt(m[2]), { scenario: "direct_msg" });
              msgReceived.add(1);
              msgDeliveryRate.add(1);
            }
          }
        } catch (_) {}
      });
      socket.on("error", () => {});
      socket.setTimeout(() => { socket.close(); }, 12000);
    });
  }
}

// ── 场景2：多人并发在线 ──────────────────────────────────────────────────────

function runConcurrent(vuIndex, sessions) {
  const idx      = vuIndex % USER_COUNT;
  const peerIdx  = (idx + 1) % USER_COUNT;
  const myS      = sessions[idx];
  const peerS    = sessions[peerIdx];
  if (!myS || !peerS) {
    sleep(IDLE_SECONDS);
    return;
  }

  const nodeWS = vuIndex % 2 === 0 ? NODE1_WS : NODE2_WS;

  ws.connect(wsURL(myS.token, nodeWS), {}, function (socket) {
    let i = 0;
    const commandPrefix = `${RUN_ID}-concurrent-${vuIndex}-`;
    const seenCommandResults = new Set();
    socket.on("open", () => {
      socket.setInterval(() => {
        if (i >= CONCURRENT_SEND_COUNT) return;
        const sentAt = Date.now();
        sendMsg(socket, peerS.uuid, `bench:${RUN_ID}:${sentAt}`, `${RUN_ID}-concurrent-${vuIndex}-${i}`);
        i++;
      }, 300);
    });
    socket.on("message", (raw) => {
      recordCommandResult(raw, commandPrefix, seenCommandResults, 1);
      try {
        const evt = JSON.parse(raw);
        if (evt.type === "chat.message") {
          const m = (evt.data && evt.data.content || "").match(/^bench:([^:]+):(\d+)$/);
          if (m && m[1] === RUN_ID) {
            msgLatency.add(Date.now() - parseInt(m[2]), { scenario: "concurrent" });
            msgReceived.add(1);
            msgDeliveryRate.add(1);
          }
        }
      } catch (_) {}
    });
    socket.on("error", () => {});
    socket.setTimeout(() => { socket.close(); }, 12000);
  });
}

// ── 场景3：大群广播 ──────────────────────────────────────────────────────────

function runGroupBlast(vuIndex, data) {
  const { sessions, groupUUID, groupOffset } = data;
  if (!groupUUID) return;

  const idx = groupOffset + (vuIndex % GROUP_SIZE);
  const myS = sessions[idx];
  if (!myS) {
    sleep(IDLE_SECONDS);
    return;
  }

  const nodeWS = vuIndex % 2 === 0 ? NODE1_WS : NODE2_WS;

  ws.connect(wsURL(myS.token, nodeWS), {}, function (socket) {
    let i = 0;
    const commandPrefix = `${RUN_ID}-group-${vuIndex}-`;
    const seenCommandResults = new Set();
    socket.on("open", () => {
      // 只有第一个 VU 发消息，其余监听
      if (vuIndex === 0) {
        socket.setTimeout(() => {
          socket.setInterval(() => {
            if (i >= 10) return;
            const sentAt = Date.now();
            sendMsg(socket, groupUUID, `bench:${RUN_ID}:${sentAt}`, `${RUN_ID}-group-${vuIndex}-${i}`);
            i++;
          }, 300);
        }, 1000);
      }
    });
    socket.on("message", (raw) => {
      recordCommandResult(raw, commandPrefix, seenCommandResults, Math.max(0, GROUP_SIZE - 1));
      try {
        const evt = JSON.parse(raw);
        if (evt.type === "chat.message") {
          const m = (evt.data && evt.data.content || "").match(/^bench:([^:]+):(\d+)$/);
          if (m && m[1] === RUN_ID) {
            msgLatency.add(Date.now() - parseInt(m[2]), { scenario: "group_blast" });
            msgReceived.add(1);
            msgDeliveryRate.add(1);
          }
        }
      } catch (_) {}
    });
    socket.on("error", () => {});
    socket.setTimeout(() => { socket.close(); }, 12000);
  });
}
