package sbyymx2

// 默认配置：与策划「符号概率」四套权重表 + 「倍数说明」一致；不配 reels 时走权重落盘。
// Redis 覆盖时可只发权重 + pay_table + plain_wild_*，或保留旧版 reels 条带（无权重字段时）。
const _gameJsonConfigsRaw = `
{
  "pay_table": [2, 4, 6, 8, 10, 50, 100],
  "weight_side_middle_row": {
    "symbols": [1, 2, 3, 4, 5, 6, 7],
    "weights": [0.3, 0.2, 0.15, 0.12, 0.1, 0.08, 0.05]
  },
  "weight_middle_middle_row": {
    "symbols": [1, 2, 3, 4, 5, 6, 7, 100, 9],
    "weights": [0.3, 0.2, 0.13, 0.1, 0.08, 0.06, 0.03, 0.1, 0.1]
  },
  "weight_side_upper_lower_row": {
    "symbols": [1, 2, 3, 4, 5, 6, 7],
    "weights": [0.3, 0.2, 0.15, 0.12, 0.1, 0.08, 0.05]
  },
  "weight_middle_upper_lower_row": {
    "symbols": [1, 2, 3, 4, 5, 6, 7, 100],
    "weights": [0.3, 0.2, 0.13, 0.1, 0.08, 0.06, 0.03, 0.1]
  },
  "plain_wild_multipliers": [2, 3, 5, 10, 20, 50, 100],
  "plain_wild_multiplier_probs": [0.3, 0.3, 0.2, 0.1, 0.06, 0.03, 0.01]
}
`
