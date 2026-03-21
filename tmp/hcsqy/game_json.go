package hcsqy

const _gameJsonConfigsRaw = `
{
  "pay_table": [2, 3, 5, 10, 15, 25, 50, 100],
  "lines": [
    [0, 1, 2],
    [3, 4, 5],
    [6, 7, 8],
    [0, 4, 8],
    [6, 4, 2]
  ],
  "free_trigger_count": 3,
  "free_base_times": 10,
  "free_extra_per_scatter": 2,
  "buy_free_multiplier": 75,
  "must_win_prob": 0.03,
  "wild_expand_prob": 0.05,
  "long_wild_multipliers": [2, 3, 4, 5, 6, 8, 10, 12, 15, 18, 20],
  "long_wild_multiplier_probs": [0.23, 0.20, 0.16, 0.12, 0.10, 0.08, 0.05, 0.03, 0.015, 0.01, 0.005],
  "real_data": [
    [
      [1, 2, 3, 4, 5, 6, 7, 8, 9],
      [1, 2, 3, 4, 5, 6, 7, 8, 9],
      [1, 2, 3, 4, 5, 6, 7, 8, 9]
    ],
    [
      [1, 2, 3, 4, 5, 6, 7, 8, 9],
      [1, 2, 3, 4, 5, 6, 7, 8, 9],
      [1, 2, 3, 4, 5, 6, 7, 8, 9]
    ]
  ]
}
`
