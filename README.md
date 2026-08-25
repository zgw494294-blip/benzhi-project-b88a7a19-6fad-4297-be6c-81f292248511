# 无线电试播频率协调与干扰复核放行服务

本项目为频率规划工程师、无线电技术复核员和试播授权负责人提供本地可运行的版本化 JSON HTTP API。服务围绕同一份协调案保存候选发射参数、受保护接收点、确定性干扰分析、技术复核决定、冻结版本和不可变试播授权，并提供输入复算、凭据验真和带哈希链的有序审计轨迹。

系统不依赖外部服务。业务事实写入 `events.jsonl`，每条事件带 `schemaVersion`、连续序号、前序哈希和自身哈希；`projection.json` 是通过临时文件 `Sync` 后原子替换的查询投影。服务启动时始终校验并重放事实账本，投影缺失或落后时自动重建。

## 构建与测试

项目要求 Go 1.22 或更高版本。

```bash
go build ./...
go test ./...
```

## 运行

默认仅监听高位回环地址 `127.0.0.1:19081`，默认数据目录为 `data`：

```bash
go run ./cmd/server
```

可显式指定其他回环地址和数据目录：

```bash
go run ./cmd/server -addr=127.0.0.1:19181 -data-dir=./runtime-data
```

未显式传入 `-addr` 时，也可通过 `PORT` 提供端口号，服务会绑定 `127.0.0.1:<PORT>`。非回环监听地址会在启动前被拒绝。

## 自检

下列命令会在真实监听地址启动服务，通过同源 HTTP 请求依次执行建档、候选登记、保护点登记、首轮分析、退回、参数修订、重新分析、批准、冻结、授权、验真和审计查询，然后主动关闭服务并返回确定的退出码：

```bash
go run ./cmd/server -selfcheck -addr=127.0.0.1:19081
```

未传入 `-data-dir` 时，自检使用隔离的临时账本并在结束后清理；正常运行的数据不会被改动。

## API 与状态

所有业务请求和响应均使用 `application/json`。成功响应使用 `data` 包装，错误响应包含稳定的 `code`、中文 `message`、可选 `violations` 和 `requestId`。修改命令必须携带当前 `expectedVersion`；提交分析候选还必须携带 `idempotencyKey`，相同键与相同请求指纹会返回原结果，不同指纹会得到 `idempotency_conflict`。

主要状态顺序为：

```text
draft -> analysis_pending -> analyzed -> under_review
under_review -> revision_required -> draft -> analysis_pending
under_review -> approved -> frozen -> authorized
```

主要路由如下：

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| `POST` | `/api/v1/cases` | 创建协调案 |
| `GET` | `/api/v1/cases/{caseID}` | 查询协调案完整当前视图与不可变历史 |
| `PUT` | `/api/v1/cases/{caseID}/proposal` | 登记新的候选参数修订 |
| `POST` | `/api/v1/cases/{caseID}/receivers` | 新增受保护接收点 |
| `POST` | `/api/v1/cases/{caseID}/receivers/batch` | 原子批量登记受保护接收点 |
| `PUT` | `/api/v1/cases/{caseID}/receivers/{receiverID}` | 在首次分析前修订受保护接收点 |
| `POST` | `/api/v1/cases/{caseID}/submit` | 幂等提交分析候选 |
| `POST` | `/api/v1/cases/{caseID}/submit/preflight` | 提交前资料完备性预检 |
| `POST` | `/api/v1/cases/{caseID}/assessments` | 执行确定性逐点干扰分析 |
| `GET` | `/api/v1/cases/{caseID}/assessment` | 查询输入、计算明细和复算校验结果 |
| `GET` | `/api/v1/cases/{caseID}/assessment/remediation` | 查询超限点整改参数建议 |
| `GET` | `/api/v1/cases/{caseID}/assessment/compare` | 对比两个分析修订并归因变化 |
| `POST` | `/api/v1/cases/{caseID}/review-submissions` | 将最新分析送交复核 |
| `POST` | `/api/v1/cases/{caseID}/reviews` | 记录 `approved` 或 `changes_requested` 决定 |
| `POST` | `/api/v1/cases/{caseID}/review-responses` | 提交退回复核意见整改响应 |
| `GET` | `/api/v1/cases/{caseID}/review-responses` | 查询逐项整改闭环状态 |
| `POST` | `/api/v1/cases/{caseID}/freeze` | 冻结获批候选及其摘要 |
| `POST` | `/api/v1/cases/{caseID}/freeze/preflight` | 冻结前一致性预核验 |
| `POST` | `/api/v1/cases/{caseID}/authorizations` | 签发不可变试播授权 |
| `GET` | `/api/v1/cases/{caseID}/authorization` | 获取试播授权凭据 |
| `GET` | `/api/v1/cases/{caseID}/audit` | 获取按全局账本序号排列的审计轨迹 |
| `POST` | `/api/v1/authorizations/verify` | 按协调案、授权编号和可选 `at` 时刻验真 |

干扰分析使用固定算法版本 `radio-interference-v1.0`，结合球面距离、自由空间路径损耗、天线高度修正、地形附加损耗、频偏抑制和接收天线增益计算干扰电平。逐点结果保留所有中间量、命中规则、保护裕量和通过结论，查询时会重算并逐字段校验。
