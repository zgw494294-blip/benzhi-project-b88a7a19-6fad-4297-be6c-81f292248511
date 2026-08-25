package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/coordination"
	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/domain"
)

type selfcheckClient struct {
	baseURL string
	client  *http.Client
}

type clientEnvelope[T any] struct {
	Data T              `json:"data"`
	Meta map[string]any `json:"meta"`
}

type clientErrorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func RunSelfcheck(ctx context.Context, baseURL string) error {
	client := &selfcheckClient{baseURL: strings.TrimRight(baseURL, "/"), client: &http.Client{Timeout: 3 * time.Second}}
	if err := client.waitReady(ctx); err != nil {
		return err
	}
	value, err := client.createCase(ctx)
	if err != nil {
		return err
	}
	caseID := value.ID
	value, err = client.replaceProposal(ctx, caseID, value.Version, 12)
	if err != nil {
		return err
	}
	value, err = client.addReceiver(ctx, caseID, value.Version)
	if err != nil {
		return err
	}
	value, replay, err := client.submit(ctx, caseID, value.Version, "selfcheck-submit-1")
	if err != nil {
		return err
	}
	if replay {
		return fmt.Errorf("首次提交被错误识别为幂等重放")
	}
	_, replay, err = client.submit(ctx, caseID, value.Version-1, "selfcheck-submit-1")
	if err != nil {
		return fmt.Errorf("幂等重放失败: %w", err)
	}
	if !replay {
		return fmt.Errorf("重复提交没有返回幂等重放标记")
	}
	value, err = client.versioned(ctx, http.MethodPost, "/api/v1/cases/"+caseID+"/assessments", value.Version, "分析工程师")
	if err != nil {
		return err
	}
	if value.State != domain.StateAnalyzed || value.CurrentAssessmentRevision != 1 {
		return fmt.Errorf("首轮分析状态不正确: %s", value.State)
	}
	value, err = client.versioned(ctx, http.MethodPost, "/api/v1/cases/"+caseID+"/review-submissions", value.Version, "规划工程师")
	if err != nil {
		return err
	}
	value, err = client.review(ctx, caseID, value.Version, 1, "changes_requested")
	if err != nil {
		return err
	}
	if value.State != domain.StateRevisionRequired {
		return fmt.Errorf("退回后状态不正确: %s", value.State)
	}
	value, err = client.replaceProposal(ctx, caseID, value.Version, 8)
	if err != nil {
		return err
	}
	if len(value.Reviews) == 0 || len(value.Reviews[len(value.Reviews)-1].Findings) == 0 {
		return fmt.Errorf("退回复核意见缺少稳定标识")
	}
	value, err = client.respondReview(ctx, caseID, value.Version, value.Reviews[len(value.Reviews)-1].Findings[0].ID)
	if err != nil {
		return err
	}
	value, _, err = client.submit(ctx, caseID, value.Version, "selfcheck-submit-2")
	if err != nil {
		return err
	}
	value, err = client.versioned(ctx, http.MethodPost, "/api/v1/cases/"+caseID+"/assessments", value.Version, "分析工程师")
	if err != nil {
		return err
	}
	value, err = client.versioned(ctx, http.MethodPost, "/api/v1/cases/"+caseID+"/review-submissions", value.Version, "规划工程师")
	if err != nil {
		return err
	}
	value, err = client.review(ctx, caseID, value.Version, 2, "approved")
	if err != nil {
		return err
	}
	value, err = client.freeze(ctx, caseID, value.Version)
	if err != nil {
		return err
	}
	authorization, err := client.authorize(ctx, caseID, value.Version)
	if err != nil {
		return err
	}
	if authorization.AuthorizationNo == "" || authorization.VerificationDigest == "" {
		return fmt.Errorf("授权凭据缺少编号或摘要")
	}
	if err := client.verifyQueries(ctx, caseID, authorization.AuthorizationNo); err != nil {
		return err
	}
	return nil
}

func (c *selfcheckClient) respondReview(ctx context.Context, caseID string, version int64, findingID string) (*domain.CoordinationCase, error) {
	command := coordination.ReviewResponseCommand{ExpectedVersion: version, FindingID: findingID, Explanation: "已按意见降低发射功率并完成重新分析", Responder: "规划工程师"}
	return requestData[domain.CoordinationCase](ctx, c, http.MethodPost, "/api/v1/cases/"+caseID+"/review-responses", command, http.StatusOK)
}

func (c *selfcheckClient) waitReady(ctx context.Context) error {
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		request, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/healthz", nil)
		response, err := c.client.Do(request)
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return fmt.Errorf("等待 HTTP 服务就绪超时")
		case <-ticker.C:
		}
	}
}

func (c *selfcheckClient) createCase(ctx context.Context) (*domain.CoordinationCase, error) {
	command := coordination.CreateCaseCommand{Title: "沿江调频试播协调案", RegionCode: "CN-31", Applicant: "自检广播技术部", Actor: "规划工程师"}
	return requestData[domain.CoordinationCase](ctx, c, http.MethodPost, "/api/v1/cases", command, http.StatusCreated)
}

func (c *selfcheckClient) replaceProposal(ctx context.Context, caseID string, version int64, eirp float64) (*domain.CoordinationCase, error) {
	command := coordination.ProposalCommand{
		ExpectedVersion: version, FrequencyHz: 100_100_000, BandwidthHz: 200_000, EIRPDbm: eirp,
		AntennaGainDbi: 3, AntennaHeightM: 50, Latitude: 31.2304, Longitude: 121.4737,
		EmissionClass: "F3E", Rationale: "依据试播覆盖设计及现场勘测记录", Actor: "规划工程师",
	}
	return requestData[domain.CoordinationCase](ctx, c, http.MethodPut, "/api/v1/cases/"+caseID+"/proposal", command, http.StatusOK)
}

func (c *selfcheckClient) addReceiver(ctx context.Context, caseID string, version int64) (*domain.CoordinationCase, error) {
	command := coordination.ReceiverCommand{
		ExpectedVersion: version, ID: "rx-selfcheck-1", Label: "东部保护监测点",
		Latitude: 31.3204, Longitude: 121.5237, ReceiveFrequencyHz: 100_100_000,
		ProtectionThresholdDbm: -65, AntennaGainDbi: 2, TerrainClass: "urban",
		EvidenceRef: "MONITOR-2026-001", Actor: "规划工程师",
	}
	return requestData[domain.CoordinationCase](ctx, c, http.MethodPost, "/api/v1/cases/"+caseID+"/receivers", command, http.StatusCreated)
}

func (c *selfcheckClient) submit(ctx context.Context, caseID string, version int64, key string) (*domain.CoordinationCase, bool, error) {
	command := coordination.SubmitCommand{ExpectedVersion: version, IdempotencyKey: key, Actor: "规划工程师"}
	envelope, err := requestEnvelope[domain.CoordinationCase](ctx, c, http.MethodPost, "/api/v1/cases/"+caseID+"/submit", command, http.StatusOK)
	if err != nil {
		return nil, false, err
	}
	replay, _ := envelope.Meta["idempotentReplay"].(bool)
	return &envelope.Data, replay, nil
}

func (c *selfcheckClient) versioned(ctx context.Context, method, path string, version int64, actor string) (*domain.CoordinationCase, error) {
	return requestData[domain.CoordinationCase](ctx, c, method, path, coordination.VersionedCommand{ExpectedVersion: version, Actor: actor}, http.StatusCreated, http.StatusOK)
}

func (c *selfcheckClient) review(ctx context.Context, caseID string, version int64, assessmentRevision int, decision string) (*domain.CoordinationCase, error) {
	findings := []domain.ReviewFinding{{Item: "输入依据", Severity: "info", Comment: "参数与现场资料一致"}}
	reason := "计算过程可复算，保护裕量满足要求"
	if decision == "changes_requested" {
		findings = []domain.ReviewFinding{{Item: "发射功率", Severity: "blocking", Comment: "建议降低试播功率后重新计算"}}
		reason = "需要降低发射功率并重新分析"
	}
	command := coordination.ReviewCommand{ExpectedVersion: version, AssessmentRevision: assessmentRevision, Reviewer: "技术复核员", Findings: findings, Decision: decision, Reason: reason}
	return requestData[domain.CoordinationCase](ctx, c, http.MethodPost, "/api/v1/cases/"+caseID+"/reviews", command, http.StatusCreated)
}

func (c *selfcheckClient) freeze(ctx context.Context, caseID string, version int64) (*domain.CoordinationCase, error) {
	command := coordination.FreezeCommand{ExpectedVersion: version, FrozenBy: "技术负责人"}
	return requestData[domain.CoordinationCase](ctx, c, http.MethodPost, "/api/v1/cases/"+caseID+"/freeze", command, http.StatusOK)
}

func (c *selfcheckClient) authorize(ctx context.Context, caseID string, version int64) (*domain.TrialAuthorization, error) {
	now := time.Now().UTC().Truncate(time.Second)
	command := coordination.AuthorizationCommand{ExpectedVersion: version, ValidFrom: now.Add(time.Hour), ValidUntil: now.Add(7 * 24 * time.Hour), Conditions: []string{"仅按冻结参数运行", "每日试播不超过四小时"}, Issuer: "授权负责人"}
	return requestData[domain.TrialAuthorization](ctx, c, http.MethodPost, "/api/v1/cases/"+caseID+"/authorizations", command, http.StatusCreated)
}

func (c *selfcheckClient) verifyQueries(ctx context.Context, caseID, authorizationNo string) error {
	basis, err := requestData[coordination.AnalysisBasis](ctx, c, http.MethodGet, "/api/v1/cases/"+caseID+"/assessment", nil, http.StatusOK)
	if err != nil {
		return err
	}
	if !basis.InputVerified || !basis.ComputationVerified || basis.Assessment.Revision != 2 {
		return fmt.Errorf("最新分析依据未通过摘要校验")
	}
	audit, err := requestData[auditPage](ctx, c, http.MethodGet, "/api/v1/cases/"+caseID+"/audit", nil, http.StatusOK)
	if err != nil {
		return err
	}
	if len(audit.Entries) != 15 || !audit.ChainValid {
		return fmt.Errorf("审计轨迹校验失败: %d", len(audit.Entries))
	}
	for index, entry := range audit.Entries {
		if index > 0 && entry.Sequence <= audit.Entries[index-1].Sequence {
			return fmt.Errorf("审计序号未严格递增")
		}
		if entry.Hash == "" {
			return fmt.Errorf("审计事件缺少哈希")
		}
	}
	verification, err := requestData[coordination.AuthorizationVerification](ctx, c, http.MethodPost, "/api/v1/authorizations/verify", map[string]string{"caseId": caseID, "authorizationNo": authorizationNo}, http.StatusOK)
	if err != nil {
		return err
	}
	if !verification.Valid {
		return fmt.Errorf("授权验真失败: %s", verification.Reason)
	}
	value, err := requestData[domain.CoordinationCase](ctx, c, http.MethodGet, "/api/v1/cases/"+caseID, nil, http.StatusOK)
	if err != nil {
		return err
	}
	if value.State != domain.StateAuthorized {
		return fmt.Errorf("最终状态不是 authorized: %s", value.State)
	}
	return nil
}

type auditPage struct {
	Entries    []domain.AuditEntry `json:"entries"`
	ChainValid bool                `json:"chainValid"`
}

func requestData[T any](ctx context.Context, client *selfcheckClient, method, path string, body any, expected ...int) (*T, error) {
	envelope, err := requestEnvelope[T](ctx, client, method, path, body, expected...)
	if err != nil {
		return nil, err
	}
	return &envelope.Data, nil
}

func requestEnvelope[T any](ctx context.Context, client *selfcheckClient, method, path string, body any, expected ...int) (clientEnvelope[T], error) {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return clientEnvelope[T]{}, err
		}
		reader = bytes.NewReader(payload)
	}
	request, err := http.NewRequestWithContext(ctx, method, client.baseURL+path, reader)
	if err != nil {
		return clientEnvelope[T]{}, err
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := client.client.Do(request)
	if err != nil {
		return clientEnvelope[T]{}, fmt.Errorf("%s %s 请求失败: %w", method, path, err)
	}
	defer response.Body.Close()
	accepted := false
	for _, status := range expected {
		if response.StatusCode == status {
			accepted = true
			break
		}
	}
	if !accepted {
		var failure clientErrorEnvelope
		_ = json.NewDecoder(response.Body).Decode(&failure)
		return clientEnvelope[T]{}, fmt.Errorf("%s %s 返回 %d: %s %s", method, path, response.StatusCode, failure.Error.Code, failure.Error.Message)
	}
	var envelope clientEnvelope[T]
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		return clientEnvelope[T]{}, fmt.Errorf("解析 %s %s 响应失败: %w", method, path, err)
	}
	return envelope, nil
}
