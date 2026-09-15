# 佣金、返现、钱包与提现

实际佣金扣除税费、固定成本和推广奖励后形成可分配基础；用户返现受规则比例与平台最低毛利双重约束。订单创建时冻结佣金规则快照。

返现状态：`ESTIMATED → PENDING_CONFIRM → PENDING_SETTLEMENT → AVAILABLE → WITHDRAWN`，异常为 `INVALID` 或 `CLAWED_BACK`。

钱包分别维护预计、待结算、可提现和冻结金额。每笔变更写不可变流水，业务唯一键保证一次入账；禁止后台直接覆盖余额。

提现：`CREATED → RISK_CHECK → APPROVED → PROCESSING → SUCCESS`，异常为 `REJECTED`、`FAILED`、`CANCELLED`。提交时冻结余额，失败解冻；高风险和大额提现需双人审批。

按渠道结算日对比订单数、有效佣金、退款、内部返现和渠道结算单。差异分为缺单、状态、金额、佣金或退款不一致，关闭差异必须记录证据。
