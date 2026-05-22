# ai-context

`ai-context` is a repo-local context tool for AI coding agents.

Stage 1 is intentionally deterministic and dependency-free:

- initialize a `.ai-context` knowledge store
- resolve `Service.Method` from thrift IDL files
- validate knowledge cards against thrift facts
- query capability / RPC mapping / flow / integration / runbook cards
- pack a compact development context for an AI agent
- accept reviewed pending knowledge into the formal store

LLM integration is intentionally not included in stage 1. LLMs can draft pending
knowledge later, but the tool owns validation and official writes.

## Quick Start

```bash
go run ./cmd/ai-context init
go run ./cmd/ai-context resolve InstitutionService.VerifyInstitution
go run ./cmd/ai-context validate
go run ./cmd/ai-context query "机构号校验"
go run ./cmd/ai-context pack "机构号中台回调幂等"
```

To try the bundled demo:

```bash
cd examples/demo
go run ../../cmd/ai-context validate
go run ../../cmd/ai-context query "机构号校验"
go run ../../cmd/ai-context pack "机构号中台回调幂等"
```

## Knowledge Card Types

Stage 1 supports JSON cards in these directories:

- `.ai-context/capabilities`
- `.ai-context/rpc-mapping`
- `.ai-context/flows`
- `.ai-context/integrations`
- `.ai-context/runbooks`
- `.ai-context/decisions`
- `.ai-context/terms`
- `.ai-context/pending`

Capabilities are intentionally business-level, not action-level. Use them to
route PRD language to a domain/module, then connect them to concrete actions
through `related_actions`.

The minimum card shape is:

```json
{
  "type": "capability",
  "id": "merchant.onboarding",
  "title": "商户入驻",
  "aliases": ["商户准入", "商家入驻"],
  "related_actions": ["institution.verify.start"]
}
```

RPC references use `Service.Method`, for example:

```json
{
  "primary_rpcs": ["InstitutionService.VerifyInstitution"]
}
```

`ai-context validate` checks that RPC references exist in thrift files.
