package analysis

import "math"

const earthRadiusKM = 6371.0088

func greatCircleDistanceKM(lat1, lon1, lat2, lon2 float64) float64 {
	latitude1 := degreesToRadians(lat1)
	latitude2 := degreesToRadians(lat2)
	deltaLatitude := degreesToRadians(lat2 - lat1)
	deltaLongitude := degreesToRadians(lon2 - lon1)
	a := math.Sin(deltaLatitude/2)*math.Sin(deltaLatitude/2) +
		math.Cos(latitude1)*math.Cos(latitude2)*math.Sin(deltaLongitude/2)*math.Sin(deltaLongitude/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusKM * c
}

func degreesToRadians(value float64) float64 {
	return value * math.Pi / 180
}

func freeSpacePathLossDB(distanceKM, frequencyHz float64) float64 {
	frequencyMHz := frequencyHz / 1_000_000
	effectiveDistance := math.Max(distanceKM, 0.001)
	return 32.44 + 20*math.Log10(effectiveDistance) + 20*math.Log10(frequencyMHz)
}

func heightCorrectionDB(heightM float64) float64 {
	// 高天线改善传播，因此从路径损耗中扣除，但限制修正幅度以保持模型保守。
	return math.Min(12, math.Max(0, 6*math.Log10(math.Max(heightM, 1)/10)))
}

func terrainCorrectionDB(terrainClass string) (float64, string) {
	switch terrainClass {
	case "open":
		return 0, "开阔地形不增加附加损耗"
	case "suburban":
		return 6, "郊区地形增加 6 dB 附加损耗"
	case "urban":
		return 12, "城区地形增加 12 dB 附加损耗"
	case "mountain":
		return 18, "山地遮挡增加 18 dB 附加损耗"
	default:
		return 0, "未知地形未应用修正"
	}
}

func frequencyRejectionDB(separationHz, transmitterBandwidthHz float64) (float64, string) {
	halfBandwidth := transmitterBandwidthHz / 2
	if separationHz <= halfBandwidth {
		return 0, "接收频率落入发射占用带宽，未获得频偏抑制"
	}
	ratio := separationHz / math.Max(halfBandwidth, 1)
	rejection := 20 * math.Log10(ratio)
	if rejection > 60 {
		rejection = 60
	}
	return rejection, "按频率间隔与半带宽比计算频偏抑制"
}

func round(value float64) float64 {
	return math.Round(value*1_000_000) / 1_000_000
}
