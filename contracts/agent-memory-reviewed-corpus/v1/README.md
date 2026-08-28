# Agent Memory Reviewed Corpus v1

该语料用于离线校准 Memory candidate 的晋级判断。每条 case 只保留 candidate type、resource type、evidence 数量、脱敏内容 SHA-256 和 gold label，不携带消息正文、候选摘要、用户身份或凭据。

review 必须由两名不同 reviewer 覆盖全部 case；出现分歧时必须由独立 adjudicator 只标注分歧 case。任何覆盖缺失、重复 ID、reviewer 身份复用、corpus hash 漂移或最终标签与 gold label 不一致都阻断门禁。
