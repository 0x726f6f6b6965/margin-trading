package leverage

import (
	"fmt"
	"math"
)

// NewLeverageStateMachine 初始化狀態機
func NewLeverageStateMachine(initialTier LeverageTier, initialEquity float64) *LeverageStateMachine {
	return &LeverageStateMachine{
		CurrentTier:         initialTier,
		LastRebalanceEquity: initialEquity,
		TaiexWeight:         0.8,
		TsmcWeight:          0.2,
		RebalanceBand4x:     0.10,
		LeftDipBiasThresh:   -6.0,
	}
}

// EvaluateDailyClose 根據「當前檔位 (CurrentTier)」執行明確的狀態轉移判定
func (sm *LeverageStateMachine) EvaluateDailyClose(in *DailyMarketInput) PositionDecision {
	prevTier := sm.CurrentTier
	bias60 := ((in.Close0050 - in.MA60) / in.MA60) * 100.0
	isMA60Falling := in.MA60 < in.PrevMA60
	isLeftDipTrigger := (in.Close0050 < in.MA60) && (bias60 < sm.LeftDipBiasThresh) && (in.PrevRSI6 < 20.0 && in.RSI6 >= 25.0)

	var targetTier LeverageTier
	var reason string

	// 依當前所處檔位 (0x, 1x, 2x, 4x) 進行獨立且完整的轉移路徑判定
	switch sm.CurrentTier {
	case Tier0x:
		targetTier, reason = sm.evalFrom0x(in, bias60, isLeftDipTrigger)
	case Tier1x:
		targetTier, reason = sm.evalFrom1x(in, bias60, isMA60Falling, isLeftDipTrigger)
	case Tier2x:
		targetTier, reason = sm.evalFrom2x(in, bias60, isMA60Falling)
	case Tier4x:
		targetTier, reason = sm.evalFrom4x(in, isMA60Falling)
	default:
		targetTier = Tier0x
		reason = "異常檔位重置為 0x"
	}

	// 若離開 2x 檔位，清除左側抄底專屬狀態標記
	if targetTier != Tier2x {
		sm.IsLeftSide2x = false
		sm.Left2xPassedMA20 = false
		sm.DipBuyStopLow = 0
	}

	tierChanged := targetTier != prevTier
	rebalanceTriggered := tierChanged

	// 若維持在 4x 檔位，檢查帳戶權益是否變動達 ±10% 觸發動態加減碼
	if !tierChanged && targetTier == Tier4x && sm.LastRebalanceEquity > 0 {
		equityDrift := math.Abs(in.AccountEquity-sm.LastRebalanceEquity) / sm.LastRebalanceEquity
		if equityDrift >= sm.RebalanceBand4x {
			rebalanceTriggered = true
			reason = fmt.Sprintf("4x內部動態再平衡:權益變動達 %.1f%% (%.1f萬→%.1f萬)，調整口數鎖回 4 倍曝險",
				equityDrift*100.0, sm.LastRebalanceEquity/10000.0, in.AccountEquity/10000.0)
		}
	}

	sm.CurrentTier = targetTier
	if rebalanceTriggered {
		sm.LastRebalanceEquity = in.AccountEquity
	}

	return sm.calculateContractAllocation(in, prevTier, targetTier, tierChanged, rebalanceTriggered, reason, bias60, isMA60Falling)
}

// calculateContractAllocation 根據 2026 常態點位精算微台 (TMF)、小台 (MTX)、小台積 (QFF) 口數
func (sm *LeverageStateMachine) calculateContractAllocation(
	in *DailyMarketInput,
	prevTier, targetTier LeverageTier,
	tierChanged, rebalanceTriggered bool,
	reason string,
	bias60 float64,
	isMA60Falling bool,
) PositionDecision {
	totalTargetExposure := in.AccountEquity * float64(targetTier)
	targetTaiexExp := totalTargetExposure * sm.TaiexWeight
	targetTsmcExp := totalTargetExposure * sm.TsmcWeight

	var mtxCount, tmfCount, pureTMFCount, qffCount int
	var actualTaiexExp, actualTsmcExp float64

	if targetTier > Tier0x && in.TaiexFuturesPrice > 0 && in.TsmcFuturesPrice > 0 {
		tmfContractValue := in.TaiexFuturesPrice * float64(TMFMultiplier)
		pureTMFCount = int(math.Round(targetTaiexExp / tmfContractValue))

		// 1 口小台 (MTX) = 5 口微台 (TMF)
		mtxCount = pureTMFCount / 5
		tmfCount = pureTMFCount % 5
		actualTaiexExp = float64(pureTMFCount) * tmfContractValue

		qffContractValue := in.TsmcFuturesPrice * float64(QFFMultiplier)
		qffCount = int(math.Round(targetTsmcExp / qffContractValue))
		actualTsmcExp = float64(qffCount) * qffContractValue
	}

	actualTotalExp := actualTaiexExp + actualTsmcExp
	var actualLever float64
	if in.AccountEquity > 0 {
		actualLever = actualTotalExp / in.AccountEquity
	}

	return PositionDecision{
		Date:                 in.Date,
		PreviousTier:         prevTier,
		TargetTier:           targetTier,
		TransitionPath:       fmt.Sprintf("%dx -> %dx", int(prevTier), int(targetTier)),
		TierChanged:          tierChanged,
		RebalanceTriggered:   rebalanceTriggered,
		Reason:               reason,
		Bias60Pct:            bias60,
		IsMA60Falling:        isMA60Falling,
		TargetTotalExposure:  totalTargetExposure,
		TargetTaiexExposure:  targetTaiexExp,
		TargetTsmcExposure:   targetTsmcExp,
		TargetMTXContracts:   mtxCount,
		TargetTMFContracts:   tmfCount,
		PureTMFContracts:     pureTMFCount,
		TargetQFFContracts:   qffCount,
		ActualTotalExposure:  actualTotalExp,
		ActualEffectiveLever: actualLever,
	}
}
