# B1 Provider Synthetic Recall

该隔离 smoke 在 2026-09-09（Asia/Shanghai）以真实 Provider 运行一条合成、已审核的 Memory canary。回执只保存 revision、哈希化 Task/回复标识与计数，不保存 owner、会话、Memory ID、提示词、回复正文或凭据。

首条受权 B1 Task 完成一次模型调用、写入一次 pre-model Memory lineage，并在回复中命中 synthetic `ORBIT-91` canary。owner revoke 后的新 Task 仍完成一次模型调用，已撤销 Memory lineage 为零。

撤销后的回复不用于 canary 否定断言：同一直接会话的短期消息上下文可能包含之前的回复。Core lineage 是该撤销边界的权威证据。该样本量为一，不能用于宣称整体语义成功率或默认持久 Memory 已启用。
