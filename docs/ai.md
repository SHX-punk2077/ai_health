# 大模型接入（国内优先）

## 1. 位置与分层

```
handler.Chat  →  service.AIService  →  pkg/aiclient（计划）  →  厂商 /chat/completions
                     ↓
              repository.AIRepo  →  ai_conversation / ai_message
```

- 上游统一走 **OpenAI 兼容协议**（`POST {base_url}/chat/completions`），国内主流厂商全部支持，
  因此 `internal/pkg/aiclient` 只需一份实现，换厂商 = 换 `base_url + model + api_key`。
- 配置见 `internal/config/config.go` 的 `AIConfig` 与 `aiPresets`；
  `ApplyDefaults()` 在启动时按 `provider` 补齐 `base_url` / `model`，显式配置优先。
- 业务层（对话、上下文、落库）不感知厂商差异；厂商私有能力只在 aiclient 内做能力探测，
  禁止把厂商名散落到 service 里。

## 2. 厂商预置

| provider | 厂商 | base_url | 默认模型 | 控制台 |
| --- | --- | --- | --- | --- |
| `deepseek` | DeepSeek 深度求索 | `https://api.deepseek.com/v1` | `deepseek-chat` | platform.deepseek.com |
| `qwen` | 阿里云百炼 · 通义千问 | `https://dashscope.aliyuncs.com/compatible-mode/v1` | `qwen-plus` | bailian.console.aliyun.com |
| `glm` | 智谱 AI · GLM | `https://open.bigmodel.cn/api/paas/v4` | `glm-4-flash` | open.bigmodel.cn |
| `moonshot` | 月之暗面 · Kimi | `https://api.moonshot.cn/v1` | `moonshot-v1-8k` | platform.moonshot.cn |
| `openai` | OpenAI（备用/对照） | `https://api.openai.com/v1` | `gpt-4o-mini` | platform.openai.com |
| `mock` | 本地假数据 | — | — | 无需密钥 |

选型建议（答辩场景）：

- **DeepSeek**：性价比高、中文健康问答稳，作为主用通道；
- **通义千问**：适合后续对接阿里云体检/挂号类服务；
- **GLM-4-Flash**：有免费额度，适合答辩演示与压测；
- **Kimi**：长上下文，适合整份体检报告问答的扩展方向。

## 3. 配置项

```yaml
ai:
  provider: "deepseek"      # deepseek | qwen | glm | moonshot | openai | mock
  base_url: ""              # 留空按 provider 预置
  api_key: ""               # 只从环境变量注入
  model: ""                 # 留空按 provider 预置
  timeout: 60s              # 单次请求超时（含首 token）
  max_tokens: 1024
  temperature: 0.3          # 健康建议偏保守，取值压低
  history_limit: 10         # 携带最近 10 条消息做上下文
```

环境变量（优先级最高）：`AIH_AI_PROVIDER` / `AIH_AI_BASE_URL` / `AIH_AI_MODEL` / `AIH_AI_API_KEY`。
启动期校验：未知 provider、非 mock 但缺 api_key 都直接失败，避免带病上线。

## 4. Prompt 设计

系统提示词固定四段，计划放在 `internal/pkg/prompt`：

1. **角色**：健康管理助手，不替代医生诊断；涉及急症/用药必须提示就医；
2. **用户档案**：性别、年龄、身高、体重、BMI、既往病史、过敏史（来自 `health_profile`）；
3. **近期数据**：最近 7 天 `health_record` 的血压/血糖/心率/睡眠/步数均值与极值；
4. **运动约束**：按 `sport_disease_detail` 的 `risk_level` 过滤禁忌运动，高危时优先劝阻。

约束：只依据上述数据作答；数据缺失不得编造数值；输出 Markdown 短段落，不超过 300 字。

## 5. 会话与落库

- 一次问答两条消息：先写 `ai_message`(role=user, sequence_no=n)，再写 assistant(n+1)，同一事务；
  `sequence_no` 由 `NextSequence` 在会话维度自增（表上有唯一键兜底）。
- 首轮成功后把 `ai_conversation.title` 改写为提问前 20 字（默认标题为「新对话」）。
- 上下文只取最近 `history_limit` 条，避免 token 随会话无限增长；更长历史靠摘要（后续迭代）。
- 缓存：`aihealth:ai:ctx:{uid}` 存档案摘要（TTL 10 分钟）；`aihealth:ai:lock:{uid}` 防重复提交。

## 6. 限流与错误处理

| 场景 | 处理 |
| --- | --- |
| 单用户刷接口 | `RateLimit(uid, 20, 1min)`，超限返回业务码 40001 |
| 上游 429 / 5xx | 指数退避重试 2 次（仅非流式），仍失败映射 60001 并记录 requestId |
| 上游超时 | 按 `ai.timeout` 熔断本次请求，前端提示稍后重试，不写半截消息 |
| 密钥失效 401 | 记录 error 日志但不回显上游报文，前端统一提示「AI 服务暂不可用」 |

密钥安全：只走 `AIH_AI_API_KEY`；`config.yaml` 保持空串；日志脱敏（不打印 Authorization）；
仓库 `.gitignore` 已忽略 `.env` 与 `configs/local.yaml`。

## 7. 本地验证

```bash
# 1. 不花钱跑通链路
AIH_AI_PROVIDER=mock go run ./cmd/api

# 2. 切真实厂商
AIH_AI_PROVIDER=deepseek AIH_AI_API_KEY=sk-xxx go run ./cmd/api

# 3. 冒烟（需先登录拿 accessToken）
curl -X POST http://127.0.0.1:8080/api/v1/ai/chat \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"content":"我最近血压 145/95，适合做什么运动？","stream":false}'
```

期望：返回 `code=0`，`data.answer` 含高血压禁忌提示（避免大重量憋气类运动），
且 `ai_message` 新增两条记录、`ai_conversation.updated_at` 被刷新。
