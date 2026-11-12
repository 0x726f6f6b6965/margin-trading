package leverage

import "fmt"

// LeverageTier 定義四段式槓桿檔位 (0x, 1x, 2x, 4x)
type LeverageTier int

const (
	Tier0x LeverageTier = 0 // 0倍：空頭主跌段空倉
	Tier1x LeverageTier = 1 // 1倍：多頭回測季線防禦底倉
	Tier2x LeverageTier = 2 // 2倍：左側極速抄底 / 右側半倉緩衝
	Tier4x LeverageTier = 4 // 4倍：主升段全速推進
)

func (t LeverageTier) String() string {
	switch t {
	case Tier0x:
		return "0x (空倉避險)"
	case Tier1x:
		return "1x (防禦底倉)"
	case Tier2x:
		return "2x (中繼半倉)"
	case Tier4x:
		return "4x (全速主升)"
	default:
		return fmt.Sprintf("%dx", int(t))
	}
}

type Contract float64

// ContractSpecs 臺灣期貨交易所 (TAIFEX) 合約乘數規格
const (
	MTXMultiplier Contract = 50.0  // 小型臺指期貨 (MTX)：每點 50 元新台幣
	TMFMultiplier Contract = 10.0  // 微型臺指期貨 (TMF)：每點 10 元新台幣 (1口小台 = 5口微台)
	QFFMultiplier Contract = 100.0 // 小型台積電期貨 (QFF)：每口代表 100 股台積電
)

func (c Contract) Symbol() string {
	switch c {
	case MTXMultiplier:
		return "MTX"
	case TMFMultiplier:
		return "TMF"
	case QFFMultiplier:
		return "QFF"
	default:
		return ""
	}
}

// DailyMarketInput 每日收盤輸入之市場指標與帳戶權益資料
type DailyMarketInput struct {
	Date              string  // 交易日期 (YYYY-MM-DD)
	Close0050         float64 // 0050 今日收盤價 (還原或現貨一致基準)
	Low0050           float64 // 0050 今日最低價 (用於記錄左側抄底停損基準)
	MA20              float64 // 0050 今日 20MA (月線)
	MA60              float64 // 0050 今日 60MA (季線)
	PrevMA60          float64 // 0050 昨日 60MA (判定季線斜率是否下彎)
	RSI6              float64 // 0050 今日 6日 Wilder's RSI
	PrevRSI6          float64 // 0050 昨日 6日 Wilder's RSI (判定曾跌破20後站回25)
	RSI14             float64 // 0050 今日 14日 Wilder's RSI
	TaiexFuturesPrice float64 // 臺指期近月收盤點數 (如 23500 點)
	TsmcFuturesPrice  float64 // 小型台積電期貨近月收盤價 (如 1080 元)
	AccountEquity     float64 // 期貨保證金專戶當前權益總值 (如 1,000,000 元)
}

// PositionDecision 每日收盤狀態機判定結果與目標期貨口數
type PositionDecision struct {
	Date                 string       // 判定日期
	PreviousTier         LeverageTier // 判定前檔位
	TargetTier           LeverageTier // 判定後目標檔位
	TransitionPath       string       // 狀態轉移路徑標記 (例如 "2x -> 1x" 或 "2x -> 0x")
	TierChanged          bool         // 檔位是否發生切換
	RebalanceTriggered   bool         // 是否需要執行下單調整
	Reason               string       // 觸發條件詳細說明
	Bias60Pct            float64      // 當日 60MA 季線乖離率 (%)
	IsMA60Falling        bool         // 60MA 季線斜率是否下彎
	TargetTotalExposure  float64      // 理論目標總曝險金額 (元)
	TargetTaiexExposure  float64      // 理論台指期目標曝險 (80%)
	TargetTsmcExposure   float64      // 理論小台積期目標曝險 (20%)
	TargetMTXContracts   int          // 建議持有【小台 MTX】口數
	TargetTMFContracts   int          // 建議持有【微台 TMF】口數
	PureTMFContracts     int          // 若全用【微台 TMF】之等效總口數
	TargetQFFContracts   int          // 建議持有【小台積 QFF】口數
	ActualTotalExposure  float64      // 實際口數對應之總曝險市值 (元)
	ActualEffectiveLever float64      // 實際達成之有效槓桿倍數 (倍)
}

func (p PositionDecision) Description() string {
	return fmt.Sprintf("建議持有【小台 MTX】口數:%d\n建議持有【微台 TMF】口數:%d\n建議持有【小台積 QFF】口數:%d\n實際口數對應之總曝險市值 (元):%.2f\n實際達成之有效槓桿倍數 (倍):%.2f",
		p.TargetMTXContracts,
		p.TargetTMFContracts,
		p.TargetQFFContracts,
		p.ActualTotalExposure,
		p.ActualEffectiveLever)
}

// LeverageStateMachine 四段式非對稱槓桿狀態機結構體
type LeverageStateMachine struct {
	CurrentTier         LeverageTier // 當前所處槓桿檔位 (0x, 1x, 2x, 4x)
	IsLeftSide2x        bool         // 當前 2x 是否為「季線下左側超賣抄底」進場
	Left2xPassedMA20    bool         // 左側 2x 抄底後是否已成功反彈站上過 20MA (用於移動停利保護)
	DipBuyStopLow       float64      // 左側抄底觸發當日之 0050 最低價
	LastRebalanceEquity float64      // 上次建倉或 4x 動態再平衡時之帳戶權益數

	// 可配置權重與門檻參數
	TaiexWeight       float64 // 台指期配置權重 (預設 0.80)
	TsmcWeight        float64 // 小台積期配置權重 (預設 0.20)
	RebalanceBand4x   float64 // 4x 檔位內部動態再平衡觸發門檻 (預設 0.10 即權益變動 ±10%)
	LeftDipBiasThresh float64 // 左側抄底 60MA 負乖離率門檻 (預設 -6.0%)
}
