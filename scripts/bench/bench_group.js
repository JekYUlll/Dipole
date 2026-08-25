/**
 * Dipole 大群广播压测
 * 场景：500 人群，1 个发送者持续发消息，其余 499 人监听，测量 E2E 延迟和投递率
 *
 * 运行：
 *   k6 run -e GROUP_SIZE=500 scripts/bench/bench_group.js
 */

import http from "k6/http";
import ws   from "k6/ws";
import { sleep } from "k6";
import { Trend, Counter, Rate } from "k6/metrics";

const BASE_URL   = __ENV.BASE_URL   || "http://localhost:8081";
const NODE1_WS   = __ENV.NODE1_WS   || "ws://localhost:8081";
const NODE2_WS   = __ENV.NODE2_WS   || "ws://localhost:8082";
const GROUP_SIZE = parseInt(__ENV.GROUP_SIZE || "500");
const IDLE_SECONDS = parseInt(__ENV.IDLE_SECONDS || "150");
const SEND_COUNT = parseInt(__ENV.SEND_COUNT || "20");
const SEND_INTERVAL_MS = parseInt(__ENV.SEND_INTERVAL_MS || "1000");
const SENDER_WARMUP_MS = parseInt(__ENV.SENDER_WARMUP_MS || "25000");
const RECEIVER_CONN_MS = parseInt(__ENV.RECEIVER_CONN_MS || "145000");
const SENDER_CONN_MS = parseInt(__ENV.SENDER_CONN_MS || "145000");

const msgLatency      = new Trend("msg_e2e_latency_ms", true);
const msgSent         = new Counter("msg_sent_total");
const msgReceived     = new Counter("msg_received_total");
const msgDeliveryRate = new Rate("msg_delivery_rate");
const msgExpected     = new Counter("msg_expected_receipts_total");

export const options = {
  setupTimeout: "300s",
  scenarios: {
    group_blast: {
      executor: "ramping-vus",
      startVUs: 0,
      stages: [
        { duration: "20s", target: GROUP_SIZE }, // 爬坡：所有成员上线
        { duration: "60s", target: GROUP_SIZE }, // 稳定：发送者持续发消息
        { duration: "10s", target: 0          }, // 收尾
      ],
      env: { SCENARIO: "group_blast" },
      tags: { scenario: "group_blast" },
    },
  },
  thresholds: {
    msg_e2e_latency_ms: ["p(95)<5000", "p(99)<10000"],
    msg_delivery_rate:  ["rate>0.80"],
  },
};

function post(path, body, token) {
  const headers = { "Content-Type": "application/json" };
  if (token) headers["Authorization"] = `Bearer ${token}`;
  return http.post(`${BASE_URL}${path}`, JSON.stringify(body), { headers, tags: { name: path.split("/").pop() } });
}

function get(path, token, params) {
  const headers = { Authorization: `Bearer ${token}` };
  const qs = params ? "?" + Object.entries(params).map(([k,v]) => `${k}=${v}`).join("&") : "";
  return http.get(`${BASE_URL}${path}${qs}`, { headers });
}

function phone(i) { return `139${String(i).padStart(8, "0")}`; }

function parseEnvelope(body) {
  try {
    const parsed = JSON.parse(body);
    return parsed && Array.isArray(parsed.data) ? parsed.data : [];
  } catch (_) {
    return [];
  }
}

function fetchGroupMessages(token, groupUUID, afterID) {
  const headers = { Authorization: `Bearer ${token}` };
  const query = afterID > 0 ? `?after_id=${afterID}&limit=100` : `?limit=100`;
  const res = http.get(`${BASE_URL}/api/v1/messages/group/${groupUUID}${query}`, {
    headers,
    tags: { name: "group_pull" },
  });
  if (res.status !== 200) return [];
  return parseEnvelope(res.body);
}

function collectBenchMessages(messages, seenMessageIDs, runToken) {
  let maxID = 0;
  let received = 0;

  for (const msg of messages) {
    if (typeof msg.id === "number" && msg.id > maxID) {
      maxID = msg.id;
    }
    const messageID = msg.message_id || "";
    if (messageID && seenMessageIDs.has(messageID)) continue;
    if (messageID) seenMessageIDs.add(messageID);

    const match = ((msg.content || "") + "").match(/^bench:([^:]+):(\d+)$/);
    if (!match) continue;
    if (match[1] !== runToken) continue;

    msgLatency.add(Date.now() - parseInt(match[2]), { scenario: "group_blast_hot" });
    msgReceived.add(1);
    msgDeliveryRate.add(1);
    received++;
  }

  return { maxID, received };
}

function loginUser(i) {
  const res = post("/api/v1/auth/login", { telephone: phone(i), password: "group123456" });
  if (res.status !== 200) return null;
  try { return JSON.parse(res.body).data; } catch (_) { return null; }
}

function registerUser(i) {
  http.post(
    `${BASE_URL}/api/v1/auth/register`,
    JSON.stringify({ nickname: `grp_${i}`, telephone: phone(i), password: "group123456" }),
    {
      headers: { "Content-Type": "application/json" },
      tags: { name: "register" },
      responseCallback: http.expectedStatuses(200, 409),
    },
  );
}

function getUserUUID(token, tel) {
  const res = get("/api/v1/users", token, { keyword: tel, limit: 1 });
  if (res.status !== 200) return null;
  try {
    const list = JSON.parse(res.body).data;
    return list && list.length > 0 ? list[0].uuid : null;
  } catch (_) { return null; }
}

function createGroup(token, memberUUIDs) {
  const res = post("/api/v1/groups", { name: `blast-${GROUP_SIZE}`, member_uuids: memberUUIDs }, token);
  if (res.status !== 200) return null;
  try { return JSON.parse(res.body).data.uuid; } catch (_) { return null; }
}

export function setup() {
  const runToken = String(Date.now());
  console.log(`[setup] registering ${GROUP_SIZE} users...`);
  for (let i = 0; i < GROUP_SIZE; i++) {
    registerUser(i);
  }

  console.log(`[setup] logging in ${GROUP_SIZE} users...`);
  const sessions = [];
  for (let i = 0; i < GROUP_SIZE; i++) {
    const data = loginUser(i);
    if (!data) { sessions.push(null); continue; }
    const uuid = data.user ? data.user.uuid : getUserUUID(data.token, phone(i));
    sessions.push({ token: data.token, uuid });
  }

  const ok = sessions.filter(Boolean).length;
  console.log(`[setup] logged in ${ok}/${GROUP_SIZE} users`);

  // 群主是 sessions[0]，其余为成员
  const memberUUIDs = sessions.slice(1).filter(Boolean).map(s => s.uuid);
  console.log(`[setup] creating group with ${memberUUIDs.length} members...`);
  const groupUUID = sessions[0] ? createGroup(sessions[0].token, memberUUIDs) : null;
  console.log(`[setup] group: ${groupUUID}`);

  return { sessions, groupUUID, recipientCount: memberUUIDs.length, runToken };
}

export default function(data) {
  if (__ITER > 0) {
    // 这个场景依赖“一人一条长连接”来稳定观察端到端延迟。
    // 迭代结束后让 VU 挂起，避免 ramping-vus 在同一阶段里反复重连并重复发消息。
    sleep(IDLE_SECONDS);
    return;
  }

  const { sessions, groupUUID, recipientCount, runToken } = data;
  if (!groupUUID) {
    sleep(IDLE_SECONDS);
    return;
  }

  const vuIndex = __VU - 1;
  const idx = vuIndex % GROUP_SIZE;
  const myS = sessions[idx];
  if (!myS) {
    sleep(IDLE_SECONDS);
    return;
  }

  const nodeWS = vuIndex % 2 === 0 ? NODE1_WS : NODE2_WS;
  const isSender = vuIndex === 0;
  const connDuration = isSender ? SENDER_CONN_MS : RECEIVER_CONN_MS;

  ws.connect(`${nodeWS}/api/v1/ws?token=${myS.token}`, {}, function(socket) {
    let i = 0;
    let lastMessageID = 0;
    const seenMessageIDs = new Set();

    socket.on("open", () => {
      if (isSender) {
        // 等接收者全部上线，再开始发消息。
        socket.setTimeout(() => {
          socket.setInterval(() => {
            if (i >= SEND_COUNT) return;
            const sentAt = Date.now();
            socket.send(JSON.stringify({ type: "chat.send", data: { target_uuid: groupUUID, content: `bench:${runToken}:${sentAt}` } }));
            msgSent.add(1);
            msgExpected.add(recipientCount);
            i++;
          }, SEND_INTERVAL_MS);
        }, SENDER_WARMUP_MS);
      }
    });
    socket.on("message", (raw) => {
      try {
        const evt = JSON.parse(raw);
        if (evt.type === "chat.message") {
          if (evt.data?.target_type !== 1 || evt.data?.target_uuid !== groupUUID) {
            return;
          }
          const messageID = evt.data && evt.data.message_id;
          if (messageID && seenMessageIDs.has(messageID)) {
            return;
          }
          if (messageID) {
            seenMessageIDs.add(messageID);
          }
          const m = (evt.data && evt.data.content || "").match(/^bench:([^:]+):(\d+)$/);
          if (m && !isSender) {
            if (m[1] !== runToken) return;
            msgLatency.add(Date.now() - parseInt(m[2]), { scenario: "group_blast_push" });
            msgReceived.add(1);
            msgDeliveryRate.add(1);
          }
        } else if (evt.type === "group.message.notify" && !isSender) {
          if (evt.data?.group_uuid !== groupUUID) {
            return;
          }
          // 热群模式只推 notify，正文通过 after_id 增量补拉拿回。
          // 这样压测结果能分别看到 push 广播和 notify + pull 两条链路的差异。
          const pulled = fetchGroupMessages(myS.token, groupUUID, lastMessageID);
          const { maxID } = collectBenchMessages(pulled, seenMessageIDs, runToken);
          if (maxID > lastMessageID) {
            lastMessageID = maxID;
          }
        }
      } catch (_) {}
    });
    socket.on("error", () => {});
    socket.setTimeout(() => { socket.close(); }, connDuration);
  });
}
