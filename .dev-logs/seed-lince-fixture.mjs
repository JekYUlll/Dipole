import { randomUUID } from "node:crypto";
import { writeFileSync } from "node:fs";

const GW = process.env.GW || "http://gateway:8080";
const WS = process.env.WS || GW.replace(/^http/, "ws");
const PASSWORD = "Demo1234";
const ASSISTANT = "UAI000000000000000001";

const ACCOUNTS = [
  { key: "lince", nickname: "林测", telephone: "13900001111", email: "lince@dipole.test", role: "primary" },
  { key: "zhou", nickname: "周友", telephone: "13900001112", email: "zhouyou@dipole.test", role: "friend-chatty" },
  { key: "chen", nickname: "陈默", telephone: "13900001113", email: "chenmo@dipole.test", role: "friend-quiet" },
  { key: "zhao", nickname: "赵忙", telephone: "13900001114", email: "zhaomang@dipole.test", role: "friend-active" },
  { key: "qian", nickname: "钱新", telephone: "13900001115", email: "qianxin@dipole.test", role: "friend-new" },
  { key: "sun", nickname: "孙阻", telephone: "13900001116", email: "sunzu@dipole.test", role: "friend-blocked" },
  { key: "wu", nickname: "吴申", telephone: "13900001117", email: "wushen@dipole.test", role: "pending-in" },
  { key: "zheng", nickname: "郑出", telephone: "13900001118", email: "zhengchu@dipole.test", role: "pending-out" },
  { key: "wang", nickname: "王拒", telephone: "13900001119", email: "wangju@dipole.test", role: "rejected" },
  { key: "huang", nickname: "黄主", telephone: "13900001120", email: "huangzhu@dipole.test", role: "group-owner" },
  { key: "liu", nickname: "刘多", telephone: "13900001121", email: "liuduo@dipole.test", role: "extra" },
  { key: "ma", nickname: "马群", telephone: "13900001122", email: "maqun@dipole.test", role: "extra" },
  { key: "zhu", nickname: "朱闲", telephone: "13900001123", email: "zhuxian@dipole.test", role: "extra" },
];

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function api(method, path, { token, body } = {}) {
  const res = await fetch(GW + path, {
    method,
    headers: {
      "content-type": "application/json",
      ...(token ? { authorization: "Bearer " + token } : {}),
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const text = await res.text();
  let json = {};
  try { json = text ? JSON.parse(text) : {}; } catch { json = { raw: text }; }
  if (!res.ok) {
    const err = new Error(`${method} ${path} ${res.status} ${json.message || text.slice(0, 180)}`);
    err.status = res.status;
    err.body = json;
    throw err;
  }
  return json.data;
}

async function upsertUser(spec) {
  try {
    const data = await api("POST", "/api/v1/auth/register", {
      body: { nickname: spec.nickname, telephone: spec.telephone, password: PASSWORD, email: spec.email },
    });
    console.log("registered", spec.nickname, data.user.uuid);
    return { ...spec, token: data.token, uuid: data.user.uuid };
  } catch {
    const data = await api("POST", "/api/v1/auth/login", {
      body: { telephone: spec.telephone, password: PASSWORD },
    });
    console.log("logged-in", spec.nickname, data.user.uuid);
    return { ...spec, token: data.token, uuid: data.user.uuid };
  }
}

async function applyFriend(from, to, message) {
  try {
    const data = await api("POST", "/api/v1/contacts/applications", {
      token: from.token,
      body: { target_uuid: to.uuid, message },
    });
    console.log("apply", from.nickname, "->", to.nickname, "id=" + data.id);
    return data.id;
  } catch (err) {
    if (err.status === 409) {
      console.log("apply skip", from.nickname, "->", to.nickname, err.message);
      return null;
    }
    throw err;
  }
}

async function handleApp(user, id, action) {
  if (!id) {
    const incoming = await api("GET", "/api/v1/contacts/applications?box=incoming", { token: user.token });
    const list = Array.isArray(incoming) ? incoming : [];
    const hit = list.find((a) => Number(a.status) === 0);
    if (!hit) {
      console.log("handle skip: no pending for", user.nickname, action);
      return;
    }
    id = hit.id;
  }
  await api("PATCH", `/api/v1/contacts/applications/${id}`, {
    token: user.token,
    body: { action },
  });
  console.log("handle", action, "id=" + id, "by", user.nickname);
}

async function remark(user, friend, text) {
  await api("PATCH", `/api/v1/contacts/${encodeURIComponent(friend.uuid)}/remark`, {
    token: user.token,
    body: { remark: text },
  });
}

async function block(user, friend) {
  await api("PATCH", `/api/v1/contacts/${encodeURIComponent(friend.uuid)}/block`, {
    token: user.token,
    body: { blocked: true },
  });
}

class ChatSession {
  constructor(user) {
    this.user = user;
    this.ws = null;
    this.pending = new Map();
  }
  async open() {
    if (this.ws && this.ws.readyState === 1) return;
    this.ws = new WebSocket(`${WS}/api/v1/ws?token=${this.user.token}`);
    await new Promise((resolve, reject) => {
      const t = setTimeout(() => reject(new Error("ws connect timeout " + this.user.nickname)), 10000);
      this.ws.addEventListener("message", (ev) => {
        const msg = JSON.parse(typeof ev.data === "string" ? ev.data : "{}");
        if (msg.type === "connected") {
          clearTimeout(t);
          resolve();
        }
        if (msg.type === "chat.sent") {
          const cmid = msg.data?.client_message_id;
          const waiter = this.pending.get(cmid);
          if (waiter) {
            this.pending.delete(cmid);
            waiter.resolve(msg);
          }
        }
        if (msg.type === "error") {
          const first = this.pending.values().next().value;
          if (first) first.reject(new Error("ws error " + JSON.stringify(msg).slice(0, 240)));
        }
      });
      this.ws.addEventListener("error", (e) => {
        clearTimeout(t);
        reject(e);
      });
    });
  }
  async send(targetUUID, content) {
    await this.open();
    const cmid = "seed-" + randomUUID();
    const done = new Promise((resolve, reject) => {
      const t = setTimeout(() => {
        this.pending.delete(cmid);
        reject(new Error("send timeout " + this.user.nickname + " -> " + targetUUID));
      }, 15000);
      this.pending.set(cmid, {
        resolve: (m) => { clearTimeout(t); resolve(m); },
        reject: (e) => { clearTimeout(t); reject(e); },
      });
    });
    this.ws.send(JSON.stringify({
      type: "chat.send",
      data: { target_uuid: targetUUID, content, client_message_id: cmid },
    }));
    return done;
  }
  close() {
    try { this.ws?.close(); } catch { /* ignore */ }
  }
}

async function ensureGroup(owner, name, notice, memberUUIDs) {
  const group = await api("POST", "/api/v1/groups", {
    token: owner.token,
    body: { name, notice, member_uuids: memberUUIDs },
  });
  console.log("group", name, group.uuid, "members=" + group.member_count);
  return group;
}

async function main() {
  const users = {};
  for (const spec of ACCOUNTS) {
    users[spec.key] = await upsertUser(spec);
    await sleep(80);
  }
  const me = users.lince;
  const assistant = { uuid: ASSISTANT, nickname: "Dipole AI" };

  const pairs = [
    [users.zhou, "一起协作的同事，常聊项目"],
    [users.chen, "话少但靠谱"],
    [users.zhao, "日程很满"],
    [users.qian, "刚加上，还没说过话"],
    [users.sun, "准备拉黑对照"],
    [users.huang, "其他群的群主"],
    [users.liu, "大群成员"],
    [users.ma, "大群成员"],
    [users.zhu, "大群成员"],
  ];
  for (const [friend, note] of pairs) {
    const id = await applyFriend(me, friend, "林测测试夹具：" + note);
    await handleApp(friend, id, "accept");
    await sleep(50);
  }
  await remark(me, users.zhou, "项目搭档");
  await remark(me, users.chen, "安静好友");
  await remark(me, users.qian, "新朋友");

  try {
    await applyFriend(me, assistant, "加小助手方便从通讯录点开");
  } catch (err) {
    console.log("assistant friend:", err.message);
  }

  await applyFriend(users.wu, me, "我是吴申，想加你测试待处理申请");
  await applyFriend(me, users.zheng, "加一下，用于待发出申请");
  const rejectId = await applyFriend(users.wang, me, "请加我（将被拒绝）");
  await handleApp(me, rejectId, "reject");
  await block(me, users.sun);
  console.log("blocked 孙阻");

  const groups = {};
  groups.assistantCollab = await ensureGroup(
    me,
    "【测】小助手协作群",
    "含小助手。林测是群主。用来测群 @ 小助手。",
    [ASSISTANT, users.zhou.uuid, users.chen.uuid, users.zhao.uuid],
  );
  groups.noAssistant = await ensureGroup(
    me,
    "【测】项目同步群",
    "不含小助手。用来对照：群里没有小助手时不应触发。",
    [users.zhou.uuid, users.zhao.uuid, users.liu.uuid],
  );
  groups.small = await ensureGroup(
    me,
    "【测】周末约饭",
    "两人小群，林测群主。",
    [users.chen.uuid],
  );
  groups.onlyAssistant = await ensureGroup(
    me,
    "【测】只和小助手",
    "只有林测 + 小助手。用来测群里 1v1 式 @。",
    [ASSISTANT],
  );
  groups.huangBig = await ensureGroup(
    users.huang,
    "【测】黄主大群",
    "黄主是群主，林测只是成员，含小助手。测「非群主 @」。",
    [me.uuid, ASSISTANT, users.zhou.uuid, users.liu.uuid, users.ma.uuid, users.zhu.uuid, users.zhao.uuid],
  );
  groups.huangQuiet = await ensureGroup(
    users.huang,
    "【测】静默观察群",
    "黄主群主，林测成员，无小助手，几乎不说话。",
    [me.uuid, users.zhu.uuid],
  );

  const sessions = {};
  const open = async (u) => {
    if (!sessions[u.key]) {
      sessions[u.key] = new ChatSession(u);
      await sessions[u.key].open();
    }
    return sessions[u.key];
  };

  const chatty = await open(me);
  const zhou = await open(users.zhou);
  const chen = await open(users.chen);
  const zhao = await open(users.zhao);
  const huang = await open(users.huang);
  const liu = await open(users.liu);

  const dm = async (fromSess, toUser, text) => {
    await fromSess.send(toUser.uuid, text);
    await sleep(120);
  };
  const gm = async (fromSess, group, text) => {
    await fromSess.send(group.uuid, text);
    await sleep(150);
  };

  await dm(chatty, users.zhou, "周友，下午的接口评审纪要你那边有吗？");
  await dm(zhou, me, "有，我整理了一版：支付回调要补幂等，超时改成 3s。");
  await dm(chatty, users.zhou, "好，那我按这个改。另外小助手那边我准备测群 @。");
  await dm(zhou, me, "行，协作群我已经在了。");

  await dm(chatty, users.chen, "陈默，周五能看一眼容量评估吗？");
  await dm(chen, me, "可以，我周五上午看。");

  await dm(chatty, users.zhao, "赵忙，你今天下午有空对一下排期吗？");
  await dm(zhao, me, "三点之后可以，我先把发布单推完。");
  await dm(chatty, users.zhao, "那就三点，我发会议。");

  await dm(chatty, users.qian, "钱新你好，我是林测，刚加上。后面群里见。");

  await dm(chatty, assistant, "你好，我是林测。请记住：我待会会在「小助手协作群」里 @ 你。先告诉我你能做什么？");
  await sleep(4000);

  await gm(chatty, groups.assistantCollab, "各位，这个群用来联调小助手。先别 @，我看下普通群聊记录能不能被读到。");
  await gm(zhou, groups.assistantCollab, "收到。我这边是周友，刚把评审纪要同步到私聊了。");
  await gm(zhao, groups.assistantCollab, "赵忙在。下午三点有排期会。");
  await gm(chatty, groups.assistantCollab, "好。下一步我会单独 @ 小助手，请它总结这个群在聊什么。");

  await gm(chatty, groups.noAssistant, "项目同步：回调幂等 + 超时 3s，今天合入。");
  await gm(zhou, groups.noAssistant, "我提 MR 了，标题是 fix/callback-idempotency。");
  await gm(liu, groups.noAssistant, "刘多看过了，测试用例我补两条。");

  await gm(chatty, groups.small, "陈默，周日中午湘菜可以吗？");
  await gm(chen, groups.small, "可以，我订位子。");

  await gm(chatty, groups.onlyAssistant, "这是只和小助手的群。后面我会在这里 @ 你。");

  await gm(huang, groups.huangBig, "我是黄主。这个大群林测只是成员，小助手也在。");
  await gm(chatty, groups.huangBig, "林测报到。我不是群主，待会也会试着 @ 小助手。");
  await gm(liu, groups.huangBig, "刘多也在，人有点多，消息会比较杂。");
  await gm(zhao, groups.huangBig, "赵忙路过，三点还有会。");

  await gm(huang, groups.huangQuiet, "建群占个位，平时不大说话。");

  for (const s of Object.values(sessions)) s.close();

  const contacts = await api("GET", "/api/v1/contacts", { token: me.token });
  const incoming = await api("GET", "/api/v1/contacts/applications?box=incoming", { token: me.token });
  const outgoing = await api("GET", "/api/v1/contacts/applications?box=outgoing", { token: me.token });
  const convos = await api("GET", "/api/v1/conversations", { token: me.token });

  const summary = {
    primary: { nickname: me.nickname, telephone: me.telephone, password: PASSWORD, uuid: me.uuid, email: me.email },
    loginUrl: "https://alias-bolt-conversion-written.trycloudflare.com/app/login",
    passwordForAllFixtureAccounts: PASSWORD,
    friends: (contacts || []).map((c) => ({
      nickname: c.user?.nickname, uuid: c.user?.uuid, remark: c.remark, status: c.status,
    })),
    incomingApplications: (incoming || []).map((a) => ({
      id: a.id, status: a.status, from: a.applicant?.nickname, message: a.message,
    })),
    outgoingApplications: (outgoing || []).map((a) => ({
      id: a.id, status: a.status, to: a.target?.nickname, message: a.message,
    })),
    groups: Object.fromEntries(Object.entries(groups).map(([k, g]) => [k, { uuid: g.uuid, name: g.name, members: g.member_count }])),
    conversations: (convos || []).map((c) => ({
      key: c.conversation_key, type: c.target_type, peer: c.target_user?.nickname, preview: c.last_message?.preview,
    })),
    accounts: Object.fromEntries(Object.entries(users).map(([k, u]) => [k, { nickname: u.nickname, telephone: u.telephone, uuid: u.uuid }])),
  };
  writeFileSync("/tmp/lince-fixture.json", JSON.stringify(summary, null, 2));
  console.log("=== FIXTURE READY ===");
  console.log(JSON.stringify(summary, null, 2));
}

main().catch((err) => {
  console.error("SEED FAILED", err);
  process.exit(1);
});
