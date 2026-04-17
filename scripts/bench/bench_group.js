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

const BASE_URL   = __ENV.BASE_URL   || "http://localhost:80";
const NODE1_WS   = __ENV.NODE1_WS   || "ws://localhost:8081";
const NODE2_WS   = __ENV.NODE2_WS   || "ws://localhost:8082";
const GROUP_SIZE = parseInt(__ENV.GROUP_SIZE || "500");

const msgLatency      = new Trend("msg_e2e_latency_ms", true);
const msgSent         = new Counter("msg_sent_total");
const msgReceived     = new Counter("msg_received_total");
const msgDeliveryRate = new Rate("msg_delivery_rate");

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

function loginUser(i) {
  const res = post("/api/v1/auth/login", { telephone: phone(i), password: "group123456" });
  if (res.status !== 200) return null;
  try { return JSON.parse(res.body).data; } catch (_) { return null; }
}

function registerUser(i) {
  post("/api/v1/auth/register", { nickname: `grp_${i}`, telephone: phone(i), password: "group123456" });
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

  return { sessions, groupUUID };
}

export default function(data) {
  const { sessions, groupUUID } = data;
  if (!groupUUID) return;

  const vuIndex = __VU - 1;
  const idx = vuIndex % GROUP_SIZE;
  const myS = sessions[idx];
  if (!myS) return;

  const nodeWS = vuIndex % 2 === 0 ? NODE1_WS : NODE2_WS;
  const isSender = vuIndex === 0;
  // 接收者保持长连接覆盖整个测试周期；发送者等 25s 让所有成员上线后再发
  const connDuration = isSender ? 75000 : 95000;

  ws.connect(`${nodeWS}/api/v1/ws?token=${myS.token}`, {}, function(socket) {
    let i = 0;
    socket.on("open", () => {
      if (isSender) {
        // 等 25s 让接收者全部上线，再开始发消息
        socket.setTimeout(() => {
          socket.setInterval(() => {
            if (i >= 20) return;
            const sentAt = Date.now();
            socket.send(JSON.stringify({ type: "chat.send", data: { target_uuid: groupUUID, content: `bench:${sentAt}` } }));
            msgSent.add(1);
            i++;
          }, 1000);
        }, 25000);
      }
    });
    socket.on("message", (raw) => {
      try {
        const evt = JSON.parse(raw);
        if (evt.type === "chat.message") {
          const m = (evt.data && evt.data.content || "").match(/^bench:(\d+)$/);
          if (m) {
            msgLatency.add(Date.now() - parseInt(m[1]), { scenario: "group_blast" });
            msgReceived.add(1);
            msgDeliveryRate.add(1);
          }
        }
      } catch (_) {}
    });
    socket.on("error", () => {});
    socket.setTimeout(() => { socket.close(); }, connDuration);
  });
}
