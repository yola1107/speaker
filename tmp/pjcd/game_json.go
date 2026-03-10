package pjcd

// _gameJsonConfigsRaw 游戏配置JSON
// 根据文档 https://9x04qv.axshare.com/?g=14&id=8075wd&p=%E9%A1%B9%E7%9B%AE%E8%AF%B4%E6%98%8E_1&sc=3
const _gameJsonConfigsRaw = `
{
  "pay_table": [
    [0, 0, 2, 5, 10],
    [0, 0, 2, 5, 10],
    [0, 0, 5, 8, 20],
    [0, 0, 5, 8, 20],
    [0, 0, 8, 10, 50],
    [0, 0, 10, 20, 100],
    [0, 0, 20, 50, 200]
  ],
  "lines": [
    [6, 7, 8, 9, 10],
    [1, 2, 3, 4, 5],
    [11, 12, 13, 14, 15],
    [1, 7, 13, 9, 5],
    [11, 7, 3, 9, 15],
    [1, 2, 8, 4, 5],
    [11, 12, 8, 14, 15],
    [6, 12, 13, 14, 10],
    [6, 2, 3, 4, 10],
    [1, 7, 8, 9, 5],
    [11, 7, 8, 9, 15],
    [6, 7, 3, 9, 10],
    [6, 7, 13, 9, 10],
    [6, 2, 8, 4, 10],
    [6, 12, 8, 14, 10],
    [1, 7, 3, 9, 5],
    [11, 7, 13, 9, 15],
    [1, 2, 8, 14, 15],
    [11, 12, 8, 4, 5],
    [1, 12, 3, 14, 5]
  ],
  "base_symbol_weights": [1900, 1900, 1500, 1500, 1225, 1050, 925],
  "free_symbol_weights": [1600, 1600, 1500, 1500, 1350, 1250, 1200],
  "symbol_permutation_weights": [8100, 1700, 200],
  "base_scatter_prob": 183,
  "base_wild_prob": 275,
  "free_scatter_prob": 200,
  "free_wild_prob": 590,
  "base_round_multipliers": [1, 2, 3, 5],
  "free_round_multipliers": [3, 6, 9, 15],
  "wild_add_fourth_multiplier": 5,
  "base_reel_generate_interval": 10,
  "free_game_times": 8,
  "free_game_scatter_min": 3,
  "free_game_add_times_per_scatter": 2,
  "free_game_add_times": 3,
  "free_game_add_times_scatter_min": 2,
  "free_game_add_more_times_per_scatter": 2
}
`

// 赔付线位置映射说明：
// 位置编号（3行5列）：
// ┌─────┬─────┬─────┬─────┬─────┐
// │  1  │  2  │  3  │  4  │  5  │  ← 顶行（row=0）
// ├─────┼─────┼─────┼─────┼─────┤
// │  6  │  7  │  8  │  9  │ 10  │  ← 中行（row=1）
// ├─────┼─────┼─────┼─────┼─────┤
// │ 11  │ 12  │ 13  │ 14  │ 15  │  ← 底行（row=2）
// └─────┴─────┴─────┴─────┴─────┘
