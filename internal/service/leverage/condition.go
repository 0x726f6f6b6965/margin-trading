package leverage

import "fmt"

// 1. 當前處於【0x 空倉】之轉출路徑：0x -> 4x | 0x -> 2x | 0x -> 1x | 0x -> 0x
func (sm *LeverageStateMachine) evalFrom0x(in *DailyMarketInput, bias60 float64, isLeftDipTrigger bool) (LeverageTier, string) {
	if in.Close0050 >= in.MA60 && in.RSI14 > 55.0 {
		return Tier4x, fmt.Sprintf("【0x→4x 主升全開】站上60MA 且 RSI14(%.1f)大於55", in.RSI14)
	}
	if isLeftDipTrigger {
		sm.IsLeftSide2x = true
		sm.Left2xPassedMA20 = false
		sm.DipBuyStopLow = in.Low0050
		return Tier2x, fmt.Sprintf("【0x→2x 左側超賣抄底】Bias60(%.2f%%)小於%.1f%% 且 RSI6由%.1f站回%.1f (停損低點%.2f)",
			bias60, sm.LeftDipBiasThresh, in.PrevRSI6, in.RSI6, in.Low0050)
	}
	if in.Close0050 >= in.MA20 && in.RSI14 > 50.0 {
		sm.IsLeftSide2x = false
		return Tier2x, fmt.Sprintf("【0x→2x 右側動能轉強】站上20MA 且 RSI14(%.1f)大於50", in.RSI14)
	}
	if in.Close0050 >= in.MA60 {
		return Tier1x, fmt.Sprintf("【0x→1x 站回季線試單】收盤站上60MA (RSI14=%.1f 尚未突破50)", in.RSI14)
	}
	return Tier0x, "【維持 0x 空倉】股價位於60MA下且未觸發抄底或轉強訊號"
}

// 2. 當前處於【1x 底倉】之轉出路徑：1x -> 4x | 1x -> 2x | 1x -> 0x | 1x -> 1x
func (sm *LeverageStateMachine) evalFrom1x(in *DailyMarketInput, bias60 float64, isMA60Falling, isLeftDipTrigger bool) (LeverageTier, string) {
	if in.Close0050 >= in.MA60 && in.RSI14 > 55.0 {
		return Tier4x, fmt.Sprintf("【1x→4x 主升全開】站上60MA 且 RSI14(%.1f)大於55", in.RSI14)
	}
	if isLeftDipTrigger {
		sm.IsLeftSide2x = true
		sm.Left2xPassedMA20 = false
		sm.DipBuyStopLow = in.Low0050
		return Tier2x, fmt.Sprintf("【1x→2x 左側超賣抄底】Bias60(%.2f%%)小於%.1f%% 且 RSI6由%.1f站回%.1f (停損低點%.2f)",
			bias60, sm.LeftDipBiasThresh, in.PrevRSI6, in.RSI6, in.Low0050)
	}
	if in.Close0050 < in.MA60 && isMA60Falling && in.RSI14 < 42.0 {
		return Tier0x, fmt.Sprintf("【1x→0x 順勢空倉】跌破下彎60MA 且 RSI14(%.1f)小於42，清空底倉", in.RSI14)
	}
	if in.Close0050 >= in.MA20 && in.RSI14 > 50.0 {
		sm.IsLeftSide2x = false
		return Tier2x, fmt.Sprintf("【1x→2x 右側動能轉強】站上20MA 且 RSI14(%.1f)大於50", in.RSI14)
	}
	return Tier1x, fmt.Sprintf("【維持 1x 底倉】季線支撐觀察中 (RSI14=%.1f)", in.RSI14)
}

// 3. 當前處於【2x 中繼半倉】之完整轉出路徑：2x -> 4x | ★ 2x -> 0x | ★ 2x -> 1x | 2x -> 2x
func (sm *LeverageStateMachine) evalFrom2x(in *DailyMarketInput, bias60 float64, isMA60Falling bool) (LeverageTier, string) {
	// 路徑 A：【2x → 4x 主升段全開】
	if in.Close0050 >= in.MA60 && in.RSI14 > 55.0 {
		return Tier4x, fmt.Sprintf("【2x→4x 主升全開】站上60MA 且 RSI14(%.1f)大於55，加碼至滿倉4倍", in.RSI14)
	}

	// 若目前處於「季線下左側超賣抄底之 2x 倉位」
	if sm.IsLeftSide2x {
		if in.Close0050 >= in.MA20 {
			sm.Left2xPassedMA20 = true // 記錄反彈已站上過 20MA
		}
		// ★ 路徑 B1：【2x → 0x 左側抄底破底停損】
		if in.Close0050 < sm.DipBuyStopLow || in.RSI6 < 20.0 {
			return Tier0x, fmt.Sprintf("【2x→0x 左側破底停損】收盤(%.2f)跌破抄底日低點(%.2f)或RSI6(%.1f)再破20，全數平倉",
				in.Close0050, sm.DipBuyStopLow, in.RSI6)
		}
		// ★ 路徑 B2：【2x → 0x / 1x 左側反彈站上20MA後卻受阻季線再次失守20MA】
		if sm.Left2xPassedMA20 && in.Close0050 < in.MA20 {
			if in.Close0050 < in.MA60 && isMA60Falling && in.RSI14 < 45.0 {
				return Tier0x, fmt.Sprintf("【2x→0x 左側反彈力竭清倉】反彈未過下彎60MA即再破20MA且RSI14(%.1f)小於45", in.RSI14)
			}
			if in.RSI14 < 48.0 {
				return Tier1x, fmt.Sprintf("【2x→1x 反彈回落減碼】左側2x站上20MA後回跌破20MA (RSI14=%.1f)，減半退守1x", in.RSI14)
			}
		}
		// 若直接反彈站上 60MA (但 RSI14 尚未 > 55)，解除左側標記轉為常規 2x
		if in.Close0050 >= in.MA60 {
			sm.IsLeftSide2x = false
			return Tier2x, fmt.Sprintf("【維持 2x】左側抄底成功站回60MA之上 (RSI14=%.1f)，轉為常規中繼倉", in.RSI14)
		}
		return Tier2x, fmt.Sprintf("【維持 2x 左側反彈倉】續抱反彈 (防守低點 %.2f，當前 Bias60=%.2f%%)", sm.DipBuyStopLow, bias60)
	}

	// 以下為「常規/右側 2x 倉位 (由 4x 減碼下來，或由右側站上 20MA 升上來)」之退場路徑：

	// ★ 路徑 C：【2x → 0x 右側跌破下彎季線順勢空倉】
	if in.Close0050 < in.MA60 && isMA60Falling && in.RSI14 < 42.0 {
		return Tier0x, fmt.Sprintf("【2x→0x 順勢空倉】跌破下彎60MA 且 RSI14(%.1f)小於42，由2x直接清倉歸零", in.RSI14)
	}

	// ★ 路徑 D1：【2x → 1x 跌破上揚季線退守底倉】
	if in.Close0050 < in.MA60 {
		// 若股價在季線下，且同時跌破 20MA (或原本從季線上跌破上揚季線) -> 退守 1x 底倉
		if in.Close0050 < in.MA20 || in.RSI14 <= 50.0 {
			return Tier1x, fmt.Sprintf("【2x→1x 跌破季線/月線退守】收盤低於60MA且動能轉弱(RSI14=%.1f)，退守1倍底倉", in.RSI14)
		}
		return Tier2x, fmt.Sprintf("【維持 2x】雖在60MA下但仍守穩20MA之上且RSI14(%.1f)大於50", in.RSI14)
	}

	// ★ 路徑 D2：【2x → 1x 季線上動能持續惡化 (4x -> 2x -> 1x 階梯減碼)】
	if in.Close0050 < in.MA20 && in.RSI14 < 45.0 {
		return Tier1x, fmt.Sprintf("【2x→1x 季線上續弱減碼】雖在60MA上，但跌破20MA且RSI14(%.1f)進一步跌破45，退守1倍底倉", in.RSI14)
	}

	return Tier2x, fmt.Sprintf("【維持 2x 中繼半倉】多空緩衝觀察中 (RSI14=%.1f)", in.RSI14)
}

// 4. 當前處於【4x 全速主升倉】之轉出路徑：4x -> 0x | 4x -> 1x | 4x -> 2x | 4x -> 4x
func (sm *LeverageStateMachine) evalFrom4x(in *DailyMarketInput, isMA60Falling bool) (LeverageTier, string) {
	if in.Close0050 < in.MA60 {
		if isMA60Falling && in.RSI14 < 42.0 {
			return Tier0x, fmt.Sprintf("【4x→0x 長黑破下彎季線空倉】直接跌破下彎60MA且RSI14(%.1f)小於42", in.RSI14)
		}
		return Tier1x, fmt.Sprintf("【4x→1x 跌破上揚季線退守】跌破60MA但季線未下彎或RSI14(%.1f)大於等於42，退守1x底倉", in.RSI14)
	}
	// 位於 60MA 之上，檢查是否跌破 20MA 觸發階梯減碼
	if in.Close0050 < in.MA20 {
		if in.RSI14 < 45.0 {
			return Tier1x, fmt.Sprintf("【4x→1x 高檔重挫跳級減碼】跌破20MA且RSI14(%.1f)直接殺破45，退守1x", in.RSI14)
		}
		if in.RSI14 < 48.0 {
			return Tier2x, fmt.Sprintf("【4x→2x 高檔轉弱減碼】跌破20MA且RSI14(%.1f)小於48，砍半倉降至2x", in.RSI14)
		}
	}
	return Tier4x, fmt.Sprintf("【維持 4x 全速主升】站穩60MA上且未觸發減碼 (RSI14=%.1f，忽略超買鈍化)", in.RSI14)
}
