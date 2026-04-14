(function () {
  const storageKeys = {
    token: "dipole.web.token",
    user: "dipole.web.user",
    lastOfflineID: "dipole.web.lastOfflineID",
  };

  const state = {
    token: "",
    currentUser: null,
    ws: null,
    conversations: [],
    contacts: [],
    groups: new Map(),
    users: new Map(),
    messagesByKey: new Map(),
    currentChat: null,
    lastOfflineID: Number(localStorage.getItem(storageKeys.lastOfflineID) || "0"),
  };

  const elements = {
    currentUserBox: byId("currentUserBox"),
    wsStatus: byId("wsStatus"),
    loginForm: byId("loginForm"),
    registerForm: byId("registerForm"),
    restoreSessionBtn: byId("restoreSessionBtn"),
    logoutBtn: byId("logoutBtn"),
    connectWsBtn: byId("connectWsBtn"),
    disconnectWsBtn: byId("disconnectWsBtn"),
    conversationList: byId("conversationList"),
    contactList: byId("contactList"),
    incomingApplications: byId("incomingApplications"),
    outgoingApplications: byId("outgoingApplications"),
    userSearchForm: byId("userSearchForm"),
    userSearchList: byId("userSearchList"),
    groupList: byId("groupList"),
    createGroupForm: byId("createGroupForm"),
    fetchGroupForm: byId("fetchGroupForm"),
    deviceList: byId("deviceList"),
    chatTitle: byId("chatTitle"),
    chatMeta: byId("chatMeta"),
    messageList: byId("messageList"),
    sendMessageForm: byId("sendMessageForm"),
    messageInput: byId("messageInput"),
    sendFileBtn: byId("sendFileBtn"),
    fileInput: byId("fileInput"),
    sendHint: byId("sendHint"),
    loadHistoryBtn: byId("loadHistoryBtn"),
    markReadBtn: byId("markReadBtn"),
    logList: byId("logList"),
    clearLogsBtn: byId("clearLogsBtn"),
    reloadConversationsBtn: byId("reloadConversationsBtn"),
    syncOfflineBtn: byId("syncOfflineBtn"),
    reloadContactsBtn: byId("reloadContactsBtn"),
    reloadApplicationsBtn: byId("reloadApplicationsBtn"),
    reloadDevicesBtn: byId("reloadDevicesBtn"),
    logoutAllDevicesBtn: byId("logoutAllDevicesBtn"),
    wsDevice: byId("wsDevice"),
    wsDeviceId: byId("wsDeviceId"),
  };

  bindEvents();
  restoreSessionSilently();

  function bindEvents() {
    document.querySelectorAll("[data-auth-tab]").forEach((button) => {
      button.addEventListener("click", () => switchAuthTab(button.dataset.authTab));
    });

    elements.loginForm.addEventListener("submit", async (event) => {
      event.preventDefault();
      await login({
        telephone: byId("loginTelephone").value.trim(),
        password: byId("loginPassword").value,
      });
    });

    elements.registerForm.addEventListener("submit", async (event) => {
      event.preventDefault();
      await register({
        nickname: byId("registerNickname").value.trim(),
        telephone: byId("registerTelephone").value.trim(),
        password: byId("registerPassword").value,
        email: byId("registerEmail").value.trim(),
      });
    });

    elements.restoreSessionBtn.addEventListener("click", restoreSessionSilently);
    elements.logoutBtn.addEventListener("click", logout);
    elements.connectWsBtn.addEventListener("click", connectWS);
    elements.disconnectWsBtn.addEventListener("click", disconnectWS);
    elements.reloadConversationsBtn.addEventListener("click", loadConversations);
    elements.syncOfflineBtn.addEventListener("click", syncOfflineMessages);
    elements.reloadContactsBtn.addEventListener("click", loadContacts);
    elements.reloadApplicationsBtn.addEventListener("click", loadApplications);
    elements.reloadDevicesBtn.addEventListener("click", loadDevices);
    elements.logoutAllDevicesBtn.addEventListener("click", forceLogoutAllDevices);
    elements.clearLogsBtn.addEventListener("click", () => {
      elements.logList.innerHTML = "";
    });

    elements.userSearchForm.addEventListener("submit", async (event) => {
      event.preventDefault();
      const keyword = byId("userSearchKeyword").value.trim();
      if (!keyword) {
        log("warn", "请输入搜索关键字");
        return;
      }
      await searchUsers(keyword);
    });

    elements.createGroupForm.addEventListener("submit", async (event) => {
      event.preventDefault();
      await createGroup();
    });

    elements.fetchGroupForm.addEventListener("submit", async (event) => {
      event.preventDefault();
      const uuid = byId("fetchGroupUUID").value.trim();
      if (!uuid) {
        log("warn", "请输入群 UUID");
        return;
      }
      await fetchGroup(uuid, true);
    });

    elements.sendMessageForm.addEventListener("submit", async (event) => {
      event.preventDefault();
      await sendTextMessage();
    });

    elements.sendFileBtn.addEventListener("click", async () => {
      await uploadAndSendFile();
    });

    elements.loadHistoryBtn.addEventListener("click", loadOlderMessages);
    elements.markReadBtn.addEventListener("click", markCurrentConversationRead);
  }

  function switchAuthTab(tab) {
    const buttons = document.querySelectorAll("[data-auth-tab]");
    buttons.forEach((button) => button.classList.toggle("active", button.dataset.authTab === tab));
    elements.loginForm.classList.toggle("hidden", tab !== "login");
    elements.registerForm.classList.toggle("hidden", tab !== "register");
  }

  async function register(payload) {
    try {
      const result = await apiRequest("/api/v1/auth/register", {
        method: "POST",
        body: JSON.stringify(payload),
      });
      handleAuthSuccess(result.data, "注册成功，已自动登录");
    } catch (error) {
      log("error", error.message);
    }
  }

  async function login(payload) {
    try {
      const result = await apiRequest("/api/v1/auth/login", {
        method: "POST",
        body: JSON.stringify(payload),
      });
      handleAuthSuccess(result.data, "登录成功");
    } catch (error) {
      log("error", error.message);
    }
  }

  async function logout() {
    try {
      if (state.token) {
        await apiRequest("/api/v1/auth/logout", { method: "POST", auth: true });
      }
    } catch (error) {
      log("warn", `退出登录请求失败: ${error.message}`);
    }

    disconnectWS();
    clearSession();
    renderCurrentUser();
    renderConversations();
    renderContacts();
    renderApplications([], []);
    renderGroups();
    renderDevices([]);
    renderMessages([]);
    updateChatHeader();
    log("info", "本地会话已清理");
  }

  function handleAuthSuccess(data, message) {
    state.token = data.token;
    state.currentUser = data.user;
    localStorage.setItem(storageKeys.token, state.token);
    localStorage.setItem(storageKeys.user, JSON.stringify(state.currentUser));
    renderCurrentUser();
    log("info", message);
    bootstrapAfterLogin();
  }

  async function restoreSessionSilently() {
    const token = localStorage.getItem(storageKeys.token);
    const userRaw = localStorage.getItem(storageKeys.user);
    if (!token || !userRaw) {
      renderCurrentUser();
      return;
    }

    state.token = token;
    try {
      state.currentUser = JSON.parse(userRaw);
    } catch (error) {
      clearSession();
      renderCurrentUser();
      return;
    }

    renderCurrentUser();
    log("info", "已恢复本地会话");
    await bootstrapAfterLogin();
  }

  function clearSession() {
    state.token = "";
    state.currentUser = null;
    state.conversations = [];
    state.contacts = [];
    state.groups.clear();
    state.users.clear();
    state.messagesByKey.clear();
    state.currentChat = null;
    state.lastOfflineID = 0;
    localStorage.removeItem(storageKeys.token);
    localStorage.removeItem(storageKeys.user);
    localStorage.removeItem(storageKeys.lastOfflineID);
  }

  async function bootstrapAfterLogin() {
    await Promise.allSettled([
      fetchCurrentUser(),
      loadConversations(),
      loadContacts(),
      loadApplications(),
      loadDevices(),
    ]);
    connectWS();
  }

  async function fetchCurrentUser() {
    try {
      const result = await apiRequest("/api/v1/users/me", { auth: true });
      state.currentUser = result.data;
      localStorage.setItem(storageKeys.user, JSON.stringify(state.currentUser));
      renderCurrentUser();
    } catch (error) {
      log("error", `获取当前用户失败: ${error.message}`);
    }
  }

  async function loadConversations() {
    if (!state.token) return;
    try {
      const result = await apiRequest("/api/v1/conversations?limit=50", { auth: true });
      state.conversations = Array.isArray(result.data) ? result.data : [];
      state.conversations.forEach((item) => {
        if (item.target_user) {
          state.users.set(item.target_user.uuid, item.target_user);
        }
      });
      renderConversations();
      updateChatHeader();
      log("info", `会话已刷新，共 ${state.conversations.length} 条`);
    } catch (error) {
      log("error", `刷新会话失败: ${error.message}`);
    }
  }

  async function loadContacts() {
    if (!state.token) return;
    try {
      const result = await apiRequest("/api/v1/contacts", { auth: true });
      state.contacts = Array.isArray(result.data) ? result.data : [];
      state.contacts.forEach((item) => {
        if (item.user) state.users.set(item.user.uuid, item.user);
      });
      renderContacts();
    } catch (error) {
      log("error", `刷新联系人失败: ${error.message}`);
    }
  }

  async function loadApplications() {
    if (!state.token) return;
    try {
      const [incomingResult, outgoingResult] = await Promise.all([
        apiRequest("/api/v1/contacts/applications?box=incoming", { auth: true }),
        apiRequest("/api/v1/contacts/applications?box=outgoing", { auth: true }),
      ]);
      renderApplications(normalizeList(incomingResult.data), normalizeList(outgoingResult.data));
    } catch (error) {
      log("error", `刷新好友申请失败: ${error.message}`);
    }
  }

  async function loadDevices() {
    if (!state.token) return;
    try {
      const result = await apiRequest("/api/v1/users/me/devices", { auth: true });
      renderDevices(normalizeList(result.data));
    } catch (error) {
      log("error", `刷新设备会话失败: ${error.message}`);
    }
  }

  async function searchUsers(keyword) {
    try {
      const result = await apiRequest(`/api/v1/users?keyword=${encodeURIComponent(keyword)}&limit=20`, { auth: true });
      const users = normalizeList(result.data);
      users.forEach((user) => state.users.set(user.uuid, user));
      renderUserSearch(users);
      log("info", `搜索到 ${users.length} 个用户`);
    } catch (error) {
      log("error", `搜索用户失败: ${error.message}`);
    }
  }

  async function applyFriend(targetUUID) {
    const message = window.prompt("申请留言，可为空", "");
    if (message === null) return;

    try {
      await apiRequest("/api/v1/contacts/applications", {
        method: "POST",
        auth: true,
        body: JSON.stringify({
          target_uuid: targetUUID,
          message,
        }),
      });
      log("info", `已向 ${targetUUID} 发送好友申请`);
      await loadApplications();
    } catch (error) {
      log("error", `发送好友申请失败: ${error.message}`);
    }
  }

  async function handleApplication(id, action) {
    try {
      await apiRequest(`/api/v1/contacts/applications/${id}`, {
        method: "PATCH",
        auth: true,
        body: JSON.stringify({ action }),
      });
      log("info", `好友申请 ${id} 已${action === "accept" ? "接受" : "拒绝"}`);
      await Promise.allSettled([loadApplications(), loadContacts(), loadConversations()]);
    } catch (error) {
      log("error", `处理好友申请失败: ${error.message}`);
    }
  }

  async function updateRemark(friendUUID) {
    const remark = window.prompt("新的备注", "");
    if (remark === null) return;
    try {
      await apiRequest(`/api/v1/contacts/${friendUUID}/remark`, {
        method: "PATCH",
        auth: true,
        body: JSON.stringify({ remark }),
      });
      await loadContacts();
      log("info", `备注已更新: ${friendUUID}`);
    } catch (error) {
      log("error", `更新备注失败: ${error.message}`);
    }
  }

  async function updateBlock(friendUUID, blocked) {
    try {
      await apiRequest(`/api/v1/contacts/${friendUUID}/block`, {
        method: "PATCH",
        auth: true,
        body: JSON.stringify({ blocked }),
      });
      await loadContacts();
      log("info", `${blocked ? "已拉黑" : "已取消拉黑"}: ${friendUUID}`);
    } catch (error) {
      log("error", `更新拉黑状态失败: ${error.message}`);
    }
  }

  async function deleteFriend(friendUUID) {
    if (!window.confirm(`确认删除好友 ${friendUUID} 吗？`)) return;
    try {
      await apiRequest(`/api/v1/contacts/${friendUUID}`, { method: "DELETE", auth: true });
      await Promise.allSettled([loadContacts(), loadConversations()]);
      log("info", `已删除好友 ${friendUUID}`);
    } catch (error) {
      log("error", `删除好友失败: ${error.message}`);
    }
  }

  async function createGroup() {
    const payload = {
      name: byId("createGroupName").value.trim(),
      notice: byId("createGroupNotice").value.trim(),
      member_uuids: splitCSV(byId("createGroupMembers").value),
    };

    try {
      const result = await apiRequest("/api/v1/groups", {
        method: "POST",
        auth: true,
        body: JSON.stringify(payload),
      });
      cacheGroup(result.data);
      renderGroups();
      byId("fetchGroupUUID").value = result.data.uuid;
      elements.createGroupForm.reset();
      log("info", `群聊已创建: ${result.data.uuid}`);
      await loadConversations();
    } catch (error) {
      log("error", `创建群聊失败: ${error.message}`);
    }
  }

  async function fetchGroup(uuid, openChat) {
    try {
      const result = await apiRequest(`/api/v1/groups/${encodeURIComponent(uuid)}`, { auth: true });
      cacheGroup(result.data);
      renderGroups();
      if (openChat) {
        openGroupChat(result.data.uuid);
      }
      log("info", `群详情已加载: ${uuid}`);
    } catch (error) {
      log("error", `查询群聊失败: ${error.message}`);
    }
  }

  async function listGroupMembers(uuid) {
    try {
      const result = await apiRequest(`/api/v1/groups/${encodeURIComponent(uuid)}/members`, { auth: true });
      const group = state.groups.get(uuid);
      if (group) {
        group.members = normalizeList(result.data);
        state.groups.set(uuid, group);
      }
      renderGroups();
      log("info", `群成员已刷新: ${uuid}`);
    } catch (error) {
      log("error", `拉取群成员失败: ${error.message}`);
    }
  }

  async function addGroupMembers(uuid) {
    const input = window.prompt("输入要添加的成员 UUID，逗号分隔", "");
    if (input === null) return;
    try {
      await apiRequest(`/api/v1/groups/${encodeURIComponent(uuid)}/members`, {
        method: "POST",
        auth: true,
        body: JSON.stringify({ member_uuids: splitCSV(input) }),
      });
      await Promise.allSettled([fetchGroup(uuid, false), listGroupMembers(uuid)]);
      log("info", `已添加群成员: ${uuid}`);
    } catch (error) {
      log("error", `添加群成员失败: ${error.message}`);
    }
  }

  async function removeGroupMembers(uuid) {
    const input = window.prompt("输入要移除的成员 UUID，逗号分隔", "");
    if (input === null) return;
    try {
      await apiRequest(`/api/v1/groups/${encodeURIComponent(uuid)}/remove-members`, {
        method: "POST",
        auth: true,
        body: JSON.stringify({ member_uuids: splitCSV(input) }),
      });
      await Promise.allSettled([fetchGroup(uuid, false), listGroupMembers(uuid)]);
      log("info", `已移除群成员: ${uuid}`);
    } catch (error) {
      log("error", `移除群成员失败: ${error.message}`);
    }
  }

  async function updateGroup(uuid) {
    const current = state.groups.get(uuid) || {};
    const name = window.prompt("新的群名称", current.name || "");
    if (name === null) return;
    const notice = window.prompt("新的群公告", current.notice || "");
    if (notice === null) return;
    try {
      const result = await apiRequest(`/api/v1/groups/${encodeURIComponent(uuid)}/update`, {
        method: "POST",
        auth: true,
        body: JSON.stringify({ name, notice }),
      });
      cacheGroup(result.data);
      renderGroups();
      updateChatHeader();
      log("info", `群资料已更新: ${uuid}`);
    } catch (error) {
      log("error", `更新群资料失败: ${error.message}`);
    }
  }

  async function dismissGroup(uuid) {
    if (!window.confirm(`确认解散群 ${uuid} 吗？`)) return;
    try {
      await apiRequest(`/api/v1/groups/${encodeURIComponent(uuid)}/dismiss`, {
        method: "POST",
        auth: true,
      });
      state.groups.delete(uuid);
      renderGroups();
      if (state.currentChat && state.currentChat.type === "group" && state.currentChat.uuid === uuid) {
        state.currentChat = null;
        renderMessages([]);
        updateChatHeader();
      }
      await loadConversations();
      log("info", `群已解散: ${uuid}`);
    } catch (error) {
      log("error", `解散群失败: ${error.message}`);
    }
  }

  async function leaveGroup(uuid) {
    if (!window.confirm(`确认退出群 ${uuid} 吗？`)) return;
    try {
      await apiRequest(`/api/v1/groups/${encodeURIComponent(uuid)}/members/me`, {
        method: "DELETE",
        auth: true,
      });
      state.groups.delete(uuid);
      renderGroups();
      await loadConversations();
      log("info", `已退出群: ${uuid}`);
    } catch (error) {
      log("error", `退群失败: ${error.message}`);
    }
  }

  async function openDirectChat(targetUUID) {
    const user = await ensureUser(targetUUID);
    state.currentChat = {
      type: "direct",
      uuid: targetUUID,
      title: user ? user.nickname || targetUUID : targetUUID,
    };
    updateChatHeader();
    await loadDirectMessages(targetUUID);
  }

  function openGroupChat(groupUUID) {
    const group = state.groups.get(groupUUID);
    state.currentChat = {
      type: "group",
      uuid: groupUUID,
      title: group ? group.name || groupUUID : groupUUID,
    };
    updateChatHeader();
    loadGroupMessages(groupUUID);
  }

  async function ensureUser(uuid) {
    if (!uuid) return null;
    if (state.users.has(uuid)) {
      return state.users.get(uuid);
    }
    try {
      const result = await apiRequest(`/api/v1/users/${encodeURIComponent(uuid)}`, { auth: true });
      state.users.set(uuid, result.data);
      return result.data;
    } catch (error) {
      log("warn", `补拉用户信息失败: ${uuid}`);
      return null;
    }
  }

  async function loadDirectMessages(targetUUID, beforeID) {
    try {
      const query = beforeID ? `?before_id=${beforeID}&limit=30` : "?limit=30";
      const result = await apiRequest(`/api/v1/messages/direct/${encodeURIComponent(targetUUID)}${query}`, { auth: true });
      const items = normalizeList(result.data);
      setMessagesForCurrentChat(items, Boolean(beforeID));
      await markCurrentConversationRead();
    } catch (error) {
      log("error", `拉取单聊消息失败: ${error.message}`);
    }
  }

  async function loadGroupMessages(groupUUID, beforeID) {
    try {
      const query = beforeID ? `?before_id=${beforeID}&limit=30` : "?limit=30";
      const result = await apiRequest(`/api/v1/messages/group/${encodeURIComponent(groupUUID)}${query}`, { auth: true });
      const items = normalizeList(result.data);
      setMessagesForCurrentChat(items, Boolean(beforeID));
      await markCurrentConversationRead();
    } catch (error) {
      log("error", `拉取群聊消息失败: ${error.message}`);
    }
  }

  function setMessagesForCurrentChat(messages, prepend) {
    if (!state.currentChat) return;
    const key = currentChatKey();
    const existing = state.messagesByKey.get(key) || [];
    const merged = prepend ? dedupeMessages([...messages, ...existing]) : dedupeMessages(messages);
    state.messagesByKey.set(key, merged);
    renderMessages(merged);
    updateLastOfflineID(merged);
  }

  async function loadOlderMessages() {
    if (!state.currentChat) {
      log("warn", "请先选择会话");
      return;
    }
    const key = currentChatKey();
    const items = state.messagesByKey.get(key) || [];
    const oldest = items[0];
    const beforeID = oldest ? oldest.id : 0;
    if (state.currentChat.type === "direct") {
      await loadDirectMessages(state.currentChat.uuid, beforeID);
      return;
    }
    await loadGroupMessages(state.currentChat.uuid, beforeID);
  }

  async function markCurrentConversationRead() {
    if (!state.currentChat) {
      return;
    }
    try {
      if (state.currentChat.type === "direct") {
        await apiRequest(`/api/v1/conversations/direct/${encodeURIComponent(state.currentChat.uuid)}/read`, {
          method: "PATCH",
          auth: true,
        });
      } else {
        await apiRequest(`/api/v1/conversations/group/${encodeURIComponent(state.currentChat.uuid)}/read`, {
          method: "PATCH",
          auth: true,
        });
      }
      state.conversations = state.conversations.map((item) => {
        if (matchesConversation(item, state.currentChat)) {
          return { ...item, unread_count: 0 };
        }
        return item;
      });
      renderConversations();
      log("info", "当前会话已标记已读");
    } catch (error) {
      log("error", `标记已读失败: ${error.message}`);
    }
  }

  async function sendTextMessage() {
    if (!state.currentChat) {
      log("warn", "请先选择会话");
      return;
    }
    const content = elements.messageInput.value.trim();
    if (!content) {
      log("warn", "消息内容不能为空");
      return;
    }
    if (!state.ws || state.ws.readyState !== WebSocket.OPEN) {
      log("warn", "请先建立 WebSocket 连接");
      return;
    }

    state.ws.send(JSON.stringify({
      type: "chat.send",
      data: {
        target_uuid: state.currentChat.uuid,
        content,
      },
    }));
    elements.messageInput.value = "";
    log("info", `已发送文本消息到 ${state.currentChat.uuid}`);
  }

  async function uploadAndSendFile() {
    if (!state.currentChat) {
      log("warn", "请先选择会话");
      return;
    }
    if (!state.ws || state.ws.readyState !== WebSocket.OPEN) {
      log("warn", "请先建立 WebSocket 连接");
      return;
    }
    const file = elements.fileInput.files[0];
    if (!file) {
      log("warn", "请先选择文件");
      return;
    }

    try {
      const formData = new FormData();
      formData.append("file", file);
      const uploadResult = await apiRequest("/api/v1/files", {
        method: "POST",
        auth: true,
        body: formData,
      });
      state.ws.send(JSON.stringify({
        type: "chat.send_file",
        data: {
          target_uuid: state.currentChat.uuid,
          file_id: uploadResult.data.file_id,
        },
      }));
      elements.fileInput.value = "";
      log("info", `文件已上传并投递: ${uploadResult.data.file_name}`);
    } catch (error) {
      log("error", `上传文件失败: ${error.message}`);
    }
  }

  async function downloadFile(fileID) {
    try {
      const result = await apiRequest(`/api/v1/files/${encodeURIComponent(fileID)}/download`, {
        auth: true,
      });
      const url = result.data.download_url;
      window.open(url, "_blank", "noopener,noreferrer");
      log("info", `已请求下载文件: ${fileID}`);
    } catch (error) {
      log("error", `下载文件失败: ${error.message}`);
    }
  }

  async function syncOfflineMessages() {
    if (!state.token) return;
    try {
      const result = await apiRequest(`/api/v1/messages/offline?after_id=${state.lastOfflineID}&limit=100`, {
        auth: true,
      });
      const items = normalizeList(result.data);
      if (items.length === 0) {
        log("info", "没有新的离线消息");
        return;
      }

      items.forEach((message) => handleIncomingMessage(message, true));
      log("info", `已补拉 ${items.length} 条离线消息`);
      await loadConversations();
    } catch (error) {
      log("error", `补拉离线消息失败: ${error.message}`);
    }
  }

  function connectWS() {
    if (!state.token) {
      log("warn", "请先登录");
      return;
    }
    if (state.ws && state.ws.readyState === WebSocket.OPEN) {
      log("info", "WebSocket 已连接");
      return;
    }

    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const url = new URL(`${protocol}//${window.location.host}/api/v1/ws`);
    url.searchParams.set("token", state.token);
    if (elements.wsDevice.value.trim()) {
      url.searchParams.set("device", elements.wsDevice.value.trim());
    }
    if (elements.wsDeviceId.value.trim()) {
      url.searchParams.set("device_id", elements.wsDeviceId.value.trim());
    }

    const ws = new WebSocket(url.toString());
    state.ws = ws;
    setWSStatus("连接中...", "status-idle");

    ws.addEventListener("open", () => {
      setWSStatus("已连接", "status-open");
      log("info", "WebSocket 已连接");
    });

    ws.addEventListener("close", () => {
      setWSStatus("已断开", "status-closed");
      log("warn", "WebSocket 已断开");
    });

    ws.addEventListener("error", () => {
      setWSStatus("连接异常", "status-closed");
      log("error", "WebSocket 连接异常");
    });

    ws.addEventListener("message", async (event) => {
      let packet;
      try {
        packet = JSON.parse(event.data);
      } catch (error) {
        log("error", `无法解析 WS 消息: ${event.data}`);
        return;
      }

      await handleWSEvent(packet);
    });
  }

  function disconnectWS() {
    if (state.ws) {
      state.ws.close();
      state.ws = null;
    }
    setWSStatus("未连接", "status-idle");
  }

  async function handleWSEvent(packet) {
    const type = packet.type;
    const data = packet.data || {};
    log("ws", `${type}: ${JSON.stringify(data)}`);

    switch (type) {
      case "connected":
        await Promise.allSettled([loadConversations(), loadDevices()]);
        break;
      case "chat.message":
        handleIncomingMessage({
          id: 0,
          message_id: data.message_id,
          from_uuid: data.from_uuid,
          target_uuid: data.target_uuid,
          target_type: data.target_type,
          message_type: data.message_type,
          content: data.content,
          file_id: data.file ? data.file.file_id : "",
          file_name: data.file ? data.file.file_name : "",
          file_size: data.file ? data.file.file_size : 0,
          download_path: data.file ? data.file.download_path : "",
          content_type: data.file ? data.file.content_type : "",
          file_expires_at: data.file ? data.file.file_expires_at : null,
          sent_at: data.sent_at,
        }, false);
        await loadConversations();
        break;
      case "chat.sent":
        handleIncomingMessage({
          id: 0,
          message_id: data.message_id,
          from_uuid: data.from_uuid,
          target_uuid: data.target_uuid,
          target_type: data.target_type,
          message_type: data.message_type,
          content: data.content,
          file_id: data.file ? data.file.file_id : "",
          file_name: data.file ? data.file.file_name : "",
          file_size: data.file ? data.file.file_size : 0,
          download_path: data.file ? data.file.download_path : "",
          content_type: data.file ? data.file.content_type : "",
          file_expires_at: data.file ? data.file.file_expires_at : null,
          sent_at: data.sent_at,
        }, false);
        await loadConversations();
        break;
      case "chat.read":
        log("info", `收到已读回执：${data.reader_uuid} 已读到 ${data.last_read_message_uuid}`);
        break;
      case "group.updated":
        cacheGroup({
          uuid: data.group_uuid,
          name: data.name,
          notice: data.notice,
          avatar: data.avatar,
        });
        renderGroups();
        updateChatHeader();
        break;
      case "group.members_added":
      case "group.members_removed":
      case "group.dismissed":
        await loadConversations();
        break;
      case "session.kicked":
        log("warn", `当前连接被踢下线: ${data.reason || "unknown"}`);
        disconnectWS();
        break;
      case "error":
        log("error", `WS 错误: ${data.message || "unknown"}`);
        break;
      default:
        break;
    }
  }

  function handleIncomingMessage(message, fromOfflinePull) {
    const conversation = deriveConversationFromMessage(message);
    const key = conversation.key;
    const items = state.messagesByKey.get(key) || [];
    state.messagesByKey.set(key, dedupeMessages([...items, message]).sort((a, b) => a.id - b.id || compareTime(a.sent_at, b.sent_at)));

    if (state.currentChat && currentChatKey() === key) {
      renderMessages(state.messagesByKey.get(key));
      if (!fromOfflinePull) {
        markCurrentConversationRead();
      }
    }

    updateLastOfflineID([message]);
  }

  function deriveConversationFromMessage(message) {
    if (message.target_type === 1) {
      return {
        key: `group:${message.target_uuid}`,
      };
    }

    const peerUUID = message.from_uuid === state.currentUser?.uuid ? message.target_uuid : message.from_uuid;
    const users = [state.currentUser?.uuid || "", peerUUID].sort();
    return {
      key: `direct:${users[0]}:${users[1]}`,
    };
  }

  async function forceLogoutDevice(connectionID) {
    try {
      await apiRequest(`/api/v1/users/me/devices/${encodeURIComponent(connectionID)}/logout`, {
        method: "POST",
        auth: true,
      });
      await loadDevices();
      log("info", `设备已下线: ${connectionID}`);
    } catch (error) {
      log("error", `下线设备失败: ${error.message}`);
    }
  }

  async function forceLogoutAllDevices() {
    if (!window.confirm("确认下线全部设备吗？当前页面也会失效。")) return;
    try {
      await apiRequest("/api/v1/users/me/devices/logout-all", {
        method: "POST",
        auth: true,
      });
      log("info", "已请求下线全部设备");
      await logout();
    } catch (error) {
      log("error", `下线全部设备失败: ${error.message}`);
    }
  }

  function renderCurrentUser() {
    elements.currentUserBox.textContent = state.currentUser
      ? JSON.stringify(state.currentUser, null, 2)
      : "未登录";
  }

  function renderConversations() {
    renderList(elements.conversationList, state.conversations, (item) => {
      const div = document.createElement("div");
      const active = state.currentChat && matchesConversation(item, state.currentChat);
      div.className = `item${active ? " active" : ""}`;

      const title = document.createElement("div");
      title.className = "item-title";
      title.textContent = conversationTitle(item);

      const meta = document.createElement("div");
      meta.className = "muted";
      meta.textContent = `${item.last_message.preview || "(空消息)"} · 未读 ${item.unread_count}`;

      const openBtn = button("打开", () => {
        if (item.target_type === 0 && item.target_user) {
          openDirectChat(item.target_user.uuid);
          return;
        }
        openGroupChat(groupUUIDFromConversation(item));
      });

      div.append(title, meta, wrapActions(openBtn));
      return div;
    });
  }

  function renderContacts() {
    renderList(elements.contactList, state.contacts, (item) => {
      const div = document.createElement("div");
      div.className = "item";
      const title = document.createElement("div");
      title.className = "item-title";
      title.textContent = `${item.user.nickname} (${item.user.uuid})`;
      const meta = document.createElement("div");
      meta.className = "muted";
      meta.textContent = `备注: ${item.remark || "-"} · 状态: ${item.status}`;
      div.append(
        title,
        meta,
        wrapActions(
          button("聊天", () => openDirectChat(item.user.uuid)),
          button("备注", () => updateRemark(item.user.uuid), "secondary"),
          button("拉黑", () => updateBlock(item.user.uuid, true), "secondary"),
          button("取消拉黑", () => updateBlock(item.user.uuid, false), "secondary"),
          button("删除", () => deleteFriend(item.user.uuid), "danger"),
        ),
      );
      return div;
    });
  }

  function renderApplications(incoming, outgoing) {
    renderList(elements.incomingApplications, incoming, (item) => {
      const div = document.createElement("div");
      div.className = "item";
      div.append(
        textBlock("item-title", `${item.applicant.nickname} (${item.applicant.uuid})`),
        textBlock("muted", `留言: ${item.message || "-"} · 状态: ${item.status}`),
        wrapActions(
          button("接受", () => handleApplication(item.id, "accept")),
          button("拒绝", () => handleApplication(item.id, "reject"), "secondary"),
        ),
      );
      return div;
    });

    renderList(elements.outgoingApplications, outgoing, (item) => {
      const div = document.createElement("div");
      div.className = "item";
      div.append(
        textBlock("item-title", `${item.target.nickname} (${item.target.uuid})`),
        textBlock("muted", `留言: ${item.message || "-"} · 状态: ${item.status}`),
      );
      return div;
    });
  }

  function renderUserSearch(users) {
    renderList(elements.userSearchList, users, (user) => {
      const div = document.createElement("div");
      div.className = "item";
      div.append(
        textBlock("item-title", `${user.nickname} (${user.uuid})`),
        textBlock("muted", `类型: ${user.user_type} · 状态: ${user.status}`),
        wrapActions(
          button("聊天", () => openDirectChat(user.uuid)),
          button("加好友", () => applyFriend(user.uuid), "secondary"),
        ),
      );
      return div;
    });
  }

  function renderGroups() {
    const groups = Array.from(state.groups.values()).sort((a, b) => String(a.name || a.uuid).localeCompare(String(b.name || b.uuid)));
    renderList(elements.groupList, groups, (group) => {
      const div = document.createElement("div");
      div.className = "item";
      const memberSummary = Array.isArray(group.members) ? ` · 已拉取成员 ${group.members.length}` : "";
      div.append(
        textBlock("item-title", `${group.name || group.uuid} (${group.uuid})`),
        textBlock("muted", `人数 ${group.member_count || 0}${memberSummary}`),
        wrapActions(
          button("打开", () => openGroupChat(group.uuid)),
          button("成员", () => listGroupMembers(group.uuid), "secondary"),
          button("加人", () => addGroupMembers(group.uuid), "secondary"),
          button("移人", () => removeGroupMembers(group.uuid), "secondary"),
          button("改资料", () => updateGroup(group.uuid), "secondary"),
          button("退群", () => leaveGroup(group.uuid), "secondary"),
          button("解散", () => dismissGroup(group.uuid), "danger"),
        ),
      );
      if (Array.isArray(group.members) && group.members.length > 0) {
        const members = document.createElement("div");
        members.className = "muted";
        members.textContent = "成员: " + group.members.map((member) => `${member.user.nickname}(${member.user.uuid})`).join(", ");
        div.append(members);
      }
      return div;
    });
  }

  function renderDevices(devices) {
    renderList(elements.deviceList, devices, (device) => {
      const div = document.createElement("div");
      div.className = "item";
      div.append(
        textBlock("item-title", `${device.device || "unknown"} · ${device.connection_id}`),
        textBlock("muted", `node=${device.node_id} · last_seen=${device.last_seen_at}`),
        wrapActions(button("下线", () => forceLogoutDevice(device.connection_id), "danger")),
      );
      return div;
    });
  }

  function renderMessages(messages) {
    elements.messageList.innerHTML = "";
    if (!messages || messages.length === 0) {
      elements.messageList.append(cloneEmpty("暂无消息"));
      return;
    }

    messages
      .slice()
      .sort((a, b) => a.id - b.id || compareTime(a.sent_at, b.sent_at))
      .forEach((message) => {
        const card = document.createElement("div");
        const mine = message.from_uuid === state.currentUser?.uuid;
        const system = message.message_type === 3;
        card.className = `message-card${mine ? " mine" : ""}${system ? " system" : ""}`;

        const meta = document.createElement("div");
        meta.className = "message-meta";
        meta.innerHTML = `
          <span>${mine ? "我" : message.from_uuid}</span>
          <span>${messageKindLabel(message)}</span>
          <span>${formatTime(message.sent_at)}</span>
          <span>${message.message_id || ""}</span>
        `;

        const content = document.createElement("div");
        content.className = "message-content";
        if (message.message_type === 1) {
          const fileLine = document.createElement("div");
          const link = document.createElement("a");
          link.href = "#";
          link.className = "file-link";
          link.textContent = `${message.file_name} (${formatSize(message.file_size)})`;
          link.addEventListener("click", (event) => {
            event.preventDefault();
            downloadFile(message.file_id);
          });
          fileLine.append(link);
          if (message.file_expires_at) {
            const expiry = document.createElement("div");
            expiry.className = "muted";
            expiry.textContent = `过期时间: ${formatTime(message.file_expires_at)}`;
            content.append(fileLine, expiry);
          } else {
            content.append(fileLine);
          }
        } else {
          content.textContent = message.content || "(空内容)";
        }

        card.append(meta, content);
        elements.messageList.append(card);
      });

    elements.messageList.scrollTop = elements.messageList.scrollHeight;
  }

  function updateChatHeader() {
    if (!state.currentChat) {
      elements.chatTitle.textContent = "未选择会话";
      elements.chatMeta.textContent = "请选择会话、联系人或群组开始测试。";
      elements.sendHint.textContent = "当前会话未选择。";
      return;
    }

    elements.chatTitle.textContent = state.currentChat.title || state.currentChat.uuid;
    elements.chatMeta.textContent = `${state.currentChat.type === "direct" ? "单聊" : "群聊"} · ${state.currentChat.uuid}`;
    elements.sendHint.textContent = `当前发送目标: ${state.currentChat.uuid}`;
  }

  function cacheGroup(group) {
    if (!group || !group.uuid) return;
    const existing = state.groups.get(group.uuid) || {};
    state.groups.set(group.uuid, { ...existing, ...group });
  }

  async function apiRequest(path, options = {}) {
    const headers = new Headers(options.headers || {});
    if (!(options.body instanceof FormData)) {
      headers.set("Content-Type", "application/json");
    }
    if (options.auth) {
      if (!state.token) {
        throw new Error("当前未登录");
      }
      headers.set("Authorization", `Bearer ${state.token}`);
    }

    const response = await fetch(path, {
      method: options.method || "GET",
      headers,
      body: options.body,
    });

    let payload = {};
    try {
      payload = await response.json();
    } catch (error) {
      payload = {};
    }

    if (!response.ok) {
      const message = payload.message || `request failed with status ${response.status}`;
      throw new Error(message);
    }

    if (typeof payload.code !== "number") {
      return payload;
    }

    return payload;
  }

  function setWSStatus(text, cls) {
    elements.wsStatus.textContent = text;
    elements.wsStatus.className = `status ${cls}`;
  }

  function log(level, message) {
    const entry = document.createElement("div");
    entry.className = "log-entry";
    entry.textContent = `[${new Date().toLocaleTimeString()}] [${level}] ${message}`;
    elements.logList.prepend(entry);
  }

  function renderList(container, items, renderItem) {
    container.innerHTML = "";
    if (!items || items.length === 0) {
      container.append(cloneEmpty("暂无数据"));
      return;
    }
    items.forEach((item) => container.append(renderItem(item)));
  }

  function cloneEmpty(text) {
    const div = document.createElement("div");
    div.className = "empty";
    div.textContent = text;
    return div;
  }

  function button(label, onClick, variant) {
    const btn = document.createElement("button");
    btn.type = "button";
    btn.textContent = label;
    if (variant) btn.classList.add(variant);
    btn.addEventListener("click", onClick);
    return btn;
  }

  function wrapActions(...buttons) {
    const div = document.createElement("div");
    div.className = "item-actions";
    buttons.forEach((btn) => div.append(btn));
    return div;
  }

  function textBlock(className, text) {
    const div = document.createElement("div");
    div.className = className;
    div.textContent = text;
    return div;
  }

  function normalizeList(value) {
    return Array.isArray(value) ? value : [];
  }

  function currentChatKey() {
    if (!state.currentChat) return "";
    if (state.currentChat.type === "group") {
      return `group:${state.currentChat.uuid}`;
    }
    const users = [state.currentUser?.uuid || "", state.currentChat.uuid].sort();
    return `direct:${users[0]}:${users[1]}`;
  }

  function dedupeMessages(messages) {
    const seen = new Set();
    return messages.filter((message) => {
      const key = message.message_id || `${message.id}-${message.sent_at}`;
      if (seen.has(key)) return false;
      seen.add(key);
      return true;
    });
  }

  function matchesConversation(item, chat) {
    if (!item || !chat) return false;
    if (chat.type === "direct") {
      return item.target_type === 0 && item.target_user && item.target_user.uuid === chat.uuid;
    }
    return item.target_type === 1 && groupUUIDFromConversation(item) === chat.uuid;
  }

  function conversationTitle(item) {
    if (item.target_type === 0 && item.target_user) {
      return `${item.target_user.nickname} (${item.target_user.uuid})`;
    }
    const groupUUID = groupUUIDFromConversation(item);
    const group = state.groups.get(groupUUID);
    return `${group ? group.name || groupUUID : groupUUID} [群聊]`;
  }

  function groupUUIDFromConversation(item) {
    if (!item || !item.conversation_key) return "";
    return String(item.conversation_key).replace(/^group:/, "");
  }

  function splitCSV(value) {
    return String(value || "")
      .split(",")
      .map((item) => item.trim())
      .filter(Boolean);
  }

  function formatTime(value) {
    if (!value) return "-";
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return String(value);
    return date.toLocaleString();
  }

  function compareTime(a, b) {
    return new Date(a).getTime() - new Date(b).getTime();
  }

  function formatSize(value) {
    if (!value) return "0 B";
    const units = ["B", "KB", "MB", "GB"];
    let size = Number(value);
    let idx = 0;
    while (size >= 1024 && idx < units.length - 1) {
      size /= 1024;
      idx += 1;
    }
    return `${size.toFixed(size >= 10 || idx === 0 ? 0 : 1)} ${units[idx]}`;
  }

  function updateLastOfflineID(messages) {
    messages.forEach((message) => {
      if (Number(message.id) > state.lastOfflineID) {
        state.lastOfflineID = Number(message.id);
      }
    });
    localStorage.setItem(storageKeys.lastOfflineID, String(state.lastOfflineID));
  }

  function messageKindLabel(message) {
    switch (message.message_type) {
      case 1:
        return "文件";
      case 2:
        return "AI";
      case 3:
        return "系统";
      default:
        return "文本";
    }
  }

  function byId(id) {
    return document.getElementById(id);
  }
})();
