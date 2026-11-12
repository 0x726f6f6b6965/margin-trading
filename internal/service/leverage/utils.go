package leverage

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/AmpyFin/yfinance-go"
)

const tsmcSymbol string = "2330.TW"

// CalculateWilderRSI 計算標準 Wilder's Smoothing RSI，回傳 (昨日RSI, 今日RSI)
func CalculateWilderRSI(closes []float64, period int) (prevRSI, currRSI float64, err error) {
	if len(closes) <= period+1 {
		return 0, 0, fmt.Errorf("歷史收盤價筆數不足，至少需要 %d 筆", period+2)
	}

	var gainSum, lossSum float64
	for i := 1; i <= period; i++ {
		diff := closes[i] - closes[i-1]
		if diff > 0 {
			gainSum += diff
		} else {
			lossSum -= diff
		}
	}

	avgGain := gainSum / float64(period)
	avgLoss := lossSum / float64(period)

	calcRSI := func(ag, al float64) float64 {
		if al == 0 {
			return 100.0
		}
		rs := ag / al
		return 100.0 - (100.0 / (1.0 + rs))
	}

	for i := period + 1; i < len(closes); i++ {
		if i == len(closes)-1 {
			prevRSI = calcRSI(avgGain, avgLoss)
		}
		diff := closes[i] - closes[i-1]
		gain, loss := 0.0, 0.0
		if diff > 0 {
			gain = diff
		} else {
			loss = -diff
		}
		avgGain = (avgGain*float64(period-1) + gain) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + loss) / float64(period)
	}

	currRSI = calcRSI(avgGain, avgLoss)
	return prevRSI, currRSI, nil
}

// requiredBars 計算今日 60MA 與昨日 60MA 共需 61 根日K
const requiredBars = 61

func scaledToFloat(scaled int64, scale int) float64 {
	return float64(scaled) / math.Pow10(scale)
}

func average(values []float64) float64 {
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// intradayLookbackDays 用 60 分K 回補近期日K缺漏的天數 (涵蓋長假)
const intradayLookbackDays = 14

type dailyBar struct {
	date  string
	close float64
	low   float64
}

// newDailyBar yfinance-go 會把 Start 往前位移 2 天 (60 分K 亦同，且會丟失盤中時間但保留順序)，這裡補回以取得實際交易日
func newDailyBar(start time.Time, closeScaled int64, closeScale int, lowScaled int64, lowScale int) dailyBar {
	return dailyBar{
		date:  start.AddDate(0, 0, 2).Format(time.DateOnly),
		close: scaledToFloat(closeScaled, closeScale),
		low:   scaledToFloat(lowScaled, lowScale),
	}
}

// aggregateIntradayBars 將依時間排序的 60 分K 依交易日彙整成日K：收盤取當日最後一根，最低取當日最小值。
func aggregateIntradayBars(bars []dailyBar) map[string]dailyBar {
	result := make(map[string]dailyBar)
	for _, bar := range bars {
		if d, ok := result[bar.date]; ok {
			d.close = bar.close
			d.low = math.Min(d.low, bar.low)
			result[bar.date] = d
			continue
		}
		result[bar.date] = bar
	}
	return result
}

// mergeDailyBars 以日K為主，並用 60 分K 彙整結果補上日K缺漏的交易日 (含盤中的今日)，回傳由舊到新排序。
func mergeDailyBars(daily []dailyBar, intraday map[string]dailyBar) []dailyBar {
	merged := make(map[string]dailyBar, len(daily)+len(intraday))
	for date, bar := range intraday {
		merged[date] = bar
	}
	for _, bar := range daily {
		merged[bar.date] = bar
	}

	result := make([]dailyBar, 0, len(merged))
	for _, bar := range merged {
		result = append(result, bar)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].date < result[j].date })
	return result
}

// GetDailyMarketInputFromYahoo 從 Yahoo 抓取日K並計算 MA/RSI 指標。
// 可於盤中執行：盤中當日的即時資料會被當成最後一根日K。
// Yahoo 盤中回傳的日K常缺少前一交易日 (OHLC 為 null)，因此另抓 60 分K 回補近期缺漏。
// TaiexFuturesPrice、TsmcFuturesPrice、AccountEquity 不在此填入，需由呼叫端補上。
func GetDailyMarketInputFromYahoo(ctx context.Context, client *yfinance.Client, symbol string) (*DailyMarketInput, error) {
	end := time.Now()
	// 抓 120 個日曆天，確保扣除假日後仍有至少 61 個交易日
	start := end.Add(-120 * 24 * time.Hour)
	batch, err := client.FetchDailyBars(ctx, symbol,
		start,
		end, false,
		fmt.Sprintf("daily-%s-%s", symbol, end.Format(time.DateOnly)))
	if err != nil {
		return nil, err
	}

	intradayBatch, err := client.FetchIntradayBars(ctx, symbol,
		end.AddDate(0, 0, -intradayLookbackDays),
		end, "60m",
		fmt.Sprintf("intraday-%s-%s", symbol, end.Format(time.DateOnly)))
	if err != nil {
		return nil, fmt.Errorf("抓取 %s 60分K失敗: %w", symbol, err)
	}

	daily := make([]dailyBar, len(batch.Bars))
	for i, bar := range batch.Bars {
		daily[i] = newDailyBar(bar.Start, bar.Close.Scaled, bar.Close.Scale, bar.Low.Scaled, bar.Low.Scale)
	}
	intraday := make([]dailyBar, len(intradayBatch.Bars))
	for i, bar := range intradayBatch.Bars {
		intraday[i] = newDailyBar(bar.Start, bar.Close.Scaled, bar.Close.Scale, bar.Low.Scaled, bar.Low.Scale)
	}

	allBars := mergeDailyBars(daily, aggregateIntradayBars(intraday))
	if len(allBars) < requiredBars {
		return nil, fmt.Errorf("%s 日K筆數不足，需要 %d 筆，實際 %d 筆", symbol, requiredBars, len(allBars))
	}

	// 從最尾端取最近 61 根日K (由舊到新)
	bars := allBars[len(allBars)-requiredBars:]
	closes := make([]float64, len(bars))
	for i, bar := range bars {
		closes[i] = bar.close
	}

	last := bars[len(bars)-1]
	n := len(closes)

	prevRSI6, rsi6, err := CalculateWilderRSI(closes, 6)
	if err != nil {
		return nil, fmt.Errorf("計算 RSI6 失敗: %w", err)
	}
	_, rsi14, err := CalculateWilderRSI(closes, 14)
	if err != nil {
		return nil, fmt.Errorf("計算 RSI14 失敗: %w", err)
	}

	tsmc, err := client.FetchIntradayBars(ctx, tsmcSymbol,
		end.AddDate(0, 0, -intradayLookbackDays),
		end, "60m",
		fmt.Sprintf("intraday-%s-%s", symbol, end.Format(time.DateOnly)))
	if err != nil {
		return nil, fmt.Errorf("抓取 %s 60分K失敗: %w", symbol, err)
	}
	tsmcPrice := scaledToFloat(tsmc.Bars[len(tsmc.Bars)-1].Close.Scaled, tsmc.Bars[len(tsmc.Bars)-1].Close.Scale)

	return &DailyMarketInput{
		Date:             last.date,
		Close0050:        closes[n-1],
		Low0050:          last.low,
		MA20:             average(closes[n-20:]),
		MA60:             average(closes[n-60:]),
		PrevMA60:         average(closes[n-61 : n-1]),
		RSI6:             rsi6,
		PrevRSI6:         prevRSI6,
		TsmcFuturesPrice: tsmcPrice,
		RSI14:            rsi14,
	}, nil
}
