# ai-context

`ai-context` 是一个面向 AI coding agent 的 repo-local context 工具。

Stage 1 刻意保持确定性，并且不引入外部依赖：

- 初始化 `.ai-context` 知识库
- 从 thrift IDL 文件解析 `Service.Method`
- 基于 thrift 事实校验知识卡
- 查询 capability / RPC mapping / flow / integration / runbook 等知识卡
- 为 AI agent 打包紧凑的开发上下文
- 将已审核的 pending knowledge 接受到正式知识库

Stage 1 不包含 LLM 集成。后续可以让 LLM 起草 pending knowledge，但校验和正式写入必须由工具负责。

## 快速开始

```bash
go run ./cmd/ai-context init
go run ./cmd/ai-context resolve InstitutionService.VerifyInstitution
go run ./cmd/ai-context validate
go run ./cmd/ai-context query "机构号校验"
go run ./cmd/ai-context query --json "机构号校验"
go run ./cmd/ai-context pack "机构号中台回调幂等"
go run ./cmd/ai-context graph
```

运行内置 demo：

```bash
cd examples/demo
go run ../../cmd/ai-context validate
go run ../../cmd/ai-context query "机构号校验"
go run ../../cmd/ai-context query --json "机构号校验"
go run ../../cmd/ai-context pack "机构号中台回调幂等"
go run ../../cmd/ai-context graph
```

## 知识卡类型

Stage 1 支持以下目录中的 JSON 知识卡：

- `.ai-context/capabilities`
- `.ai-context/rpc-mapping`
- `.ai-context/flows`
- `.ai-context/integrations`
- `.ai-context/runbooks`
- `.ai-context/decisions`
- `.ai-context/terms`
- `.ai-context/pending`

Capability 刻意保持在业务能力层，而不是具体动作层。它用于把 PRD 语言路由到领域或模块，再通过 `related_actions` 连接到具体业务动作。

最小知识卡结构：

```json
{
  "type": "capability",
  "id": "merchant.onboarding",
  "title": "商户入驻",
  "aliases": ["商户准入", "商家入驻"],
  "related_actions": ["institution.verify.start"]
}
```

RPC 引用使用 `Service.Method` 格式，例如：

```json
{
  "primary_rpcs": ["InstitutionService.VerifyInstitution"]
}
```

`ai-context validate` 会检查 RPC 引用是否真实存在于 thrift 文件中。

## Query JSON

`ai-context query --json` 输出给 AI agent 消费的结构化检索结果，而不是直接回答业务问题。

结果包含：

- 命中的知识卡、分数和置信度
- 结构化 `match_reasons`
- `related_*` 关系
- 拆分后的 `rpcs.positive`、`rpcs.avoid`、`rpcs.unknown`
- 命中片段 `snippets`

需要让 LLM 起草知识卡变更时，可以加 `--include-raw` 带上原始知识卡内容：

```bash
ai-context query --json --include-raw "机构号中台回调超时"
```

## Graph

`ai-context graph` 会根据正式知识卡和 thrift RPC index 生成派生的 `.ai-context/graph.json` property graph。它不会替代知识卡存储，知识卡仍然是正式事实源。

图谱节点包括 capability、RPC mapping、flow、integration、runbook、decision、term、thrift service、RPC、thrift file 和 flow step。

这个 graph 可用于影响范围分析、完整性检查、可视化，以及后续基于图路径的 context packing。
