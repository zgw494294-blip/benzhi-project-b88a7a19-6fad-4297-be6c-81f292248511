package domain

import (
	"math"
	"strings"
)

func ValidateNewCase(title, regionCode, applicant string) error {
	var failures []FieldViolation
	if strings.TrimSpace(title) == "" {
		failures = append(failures, Violation("title", "required", "协调案标题不能为空"))
	} else if len([]rune(title)) > 120 {
		failures = append(failures, Violation("title", "max_length", "协调案标题不能超过 120 个字符"))
	}
	if strings.TrimSpace(regionCode) == "" {
		failures = append(failures, Violation("regionCode", "required", "区域代码不能为空"))
	}
	if strings.TrimSpace(applicant) == "" {
		failures = append(failures, Violation("applicant", "required", "申请人不能为空"))
	}
	if len(failures) > 0 {
		return ValidationError(failures...)
	}
	return nil
}

func ValidateProposal(p TransmitterProposal) error {
	var failures []FieldViolation
	if !finite(p.FrequencyHz) || p.FrequencyHz < 30_000_000 || p.FrequencyHz > 3_000_000_000 {
		failures = append(failures, Violation("frequencyHz", "range", "频率必须在 30 MHz 至 3 GHz 之间"))
	}
	if !finite(p.BandwidthHz) || p.BandwidthHz <= 0 || p.BandwidthHz > 20_000_000 {
		failures = append(failures, Violation("bandwidthHz", "range", "带宽必须大于 0 且不超过 20 MHz"))
	}
	if !finite(p.EIRPDbm) || p.EIRPDbm < -30 || p.EIRPDbm > 100 {
		failures = append(failures, Violation("eirpDbm", "range", "EIRP 必须以 dBm 表示且处于 -30 至 100"))
	}
	if !finite(p.AntennaGainDbi) || p.AntennaGainDbi < -20 || p.AntennaGainDbi > 60 {
		failures = append(failures, Violation("antennaGainDbi", "range", "天线增益必须以 dBi 表示且处于 -20 至 60"))
	}
	if !finite(p.AntennaHeightM) || p.AntennaHeightM < 1 || p.AntennaHeightM > 5000 {
		failures = append(failures, Violation("antennaHeightM", "range", "天线高度必须处于 1 至 5000 米"))
	}
	failures = append(failures, coordinateViolations(p.Latitude, p.Longitude)...)
	if strings.TrimSpace(p.EmissionClass) == "" {
		failures = append(failures, Violation("emissionClass", "required", "发射类别不能为空"))
	}
	if strings.TrimSpace(p.Rationale) == "" {
		failures = append(failures, Violation("rationale", "required", "申报依据不能为空"))
	}
	if len(failures) > 0 {
		return ValidationError(failures...)
	}
	return nil
}

func ValidateReceiver(r ProtectedReceiver) error {
	var failures []FieldViolation
	if strings.TrimSpace(r.ID) == "" {
		failures = append(failures, Violation("id", "required", "受保护点 ID 不能为空"))
	}
	if strings.TrimSpace(r.Label) == "" {
		failures = append(failures, Violation("label", "required", "受保护点名称不能为空"))
	}
	failures = append(failures, coordinateViolations(r.Latitude, r.Longitude)...)
	if !finite(r.ReceiveFrequencyHz) || r.ReceiveFrequencyHz < 30_000_000 || r.ReceiveFrequencyHz > 3_000_000_000 {
		failures = append(failures, Violation("receiveFrequencyHz", "range", "接收频率必须在 30 MHz 至 3 GHz 之间"))
	}
	if !finite(r.ProtectionThresholdDbm) || r.ProtectionThresholdDbm < -200 || r.ProtectionThresholdDbm > -20 {
		failures = append(failures, Violation("protectionThresholdDbm", "range", "保护门限必须以 dBm 表示且处于 -200 至 -20"))
	}
	if !finite(r.AntennaGainDbi) || r.AntennaGainDbi < -20 || r.AntennaGainDbi > 60 {
		failures = append(failures, Violation("antennaGainDbi", "range", "接收天线增益必须处于 -20 至 60 dBi"))
	}
	if r.TerrainClass != "open" && r.TerrainClass != "suburban" && r.TerrainClass != "urban" && r.TerrainClass != "mountain" {
		failures = append(failures, Violation("terrainClass", "enum", "地形类型必须为 open、suburban、urban 或 mountain"))
	}
	if strings.TrimSpace(r.EvidenceRef) == "" {
		failures = append(failures, Violation("evidenceRef", "required", "保护依据不能为空"))
	}
	if len(failures) > 0 {
		return ValidationError(failures...)
	}
	return nil
}

func ValidateReview(decision ReviewDecision) error {
	var failures []FieldViolation
	if strings.TrimSpace(decision.Reviewer) == "" {
		failures = append(failures, Violation("reviewer", "required", "复核员不能为空"))
	}
	if decision.Decision != "approved" && decision.Decision != "changes_requested" {
		failures = append(failures, Violation("decision", "enum", "复核结论必须为 approved 或 changes_requested"))
	}
	if strings.TrimSpace(decision.Reason) == "" {
		failures = append(failures, Violation("reason", "required", "复核理由不能为空"))
	}
	for i, finding := range decision.Findings {
		if strings.TrimSpace(finding.Item) == "" || strings.TrimSpace(finding.Comment) == "" {
			failures = append(failures, Violation("findings", "required", "第 "+itoa(i+1)+" 条意见缺少检查项或说明"))
		}
		if finding.Severity != "info" && finding.Severity != "warning" && finding.Severity != "blocking" {
			failures = append(failures, Violation("findings.severity", "enum", "意见级别必须为 info、warning 或 blocking"))
		}
	}
	if len(failures) > 0 {
		return ValidationError(failures...)
	}
	return nil
}

func ValidateAuthorization(a TrialAuthorization) error {
	var failures []FieldViolation
	if strings.TrimSpace(a.Issuer) == "" {
		failures = append(failures, Violation("issuer", "required", "授权签发人不能为空"))
	}
	if a.ValidFrom.IsZero() || a.ValidUntil.IsZero() || !a.ValidUntil.After(a.ValidFrom) {
		failures = append(failures, Violation("validUntil", "after", "授权结束时间必须晚于开始时间"))
	}
	if a.ValidUntil.Sub(a.ValidFrom) > 90*24*60*60*1e9 {
		failures = append(failures, Violation("validUntil", "max_window", "试播授权有效期不能超过 90 天"))
	}
	if len(a.Conditions) == 0 {
		failures = append(failures, Violation("conditions", "min_items", "至少需要一项运行约束"))
	}
	for _, condition := range a.Conditions {
		if strings.TrimSpace(condition) == "" {
			failures = append(failures, Violation("conditions", "required", "运行约束不能为空"))
			break
		}
	}
	if len(failures) > 0 {
		return ValidationError(failures...)
	}
	return nil
}

func coordinateViolations(latitude, longitude float64) []FieldViolation {
	var failures []FieldViolation
	if !finite(latitude) || latitude < -90 || latitude > 90 {
		failures = append(failures, Violation("latitude", "range", "纬度必须处于 -90 至 90"))
	}
	if !finite(longitude) || longitude < -180 || longitude > 180 {
		failures = append(failures, Violation("longitude", "range", "经度必须处于 -180 至 180"))
	}
	return failures
}

func finite(value float64) bool { return !math.IsInf(value, 0) && !math.IsNaN(value) }

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var digits [20]byte
	position := len(digits)
	for value > 0 {
		position--
		digits[position] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[position:])
}
