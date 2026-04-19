package jqs

const _gameJsonConfigsRaw = `
{
  "pay_table": [
    [0,0,3],
    [0,0,5],
    [0,0,15],
    [0,0,30],
    [0,0,50],
    [0,0,100],
    [0,0,300]
  ],

  "lines": [
    [3,4,5],
    [6,7,8],
    [0,1,2],
    [6,4,2],
    [0,4,8]
  ],

  "mid_symbol_weight":[2700,2400,1500,1200,800,1100,300],
  "base_two_consecutive_prob": 6000,
  "base_complement_symbol_weight":[2700,2400,1500,1200,1050,800,350],

  "respin_trigger_rate": 102,
  "fake_respin_trigger_rate": 300,

  "free_three_consecutive_prob": 6000,
  "free_three_symbol_weight": [1300,1300,1500,1800,1900,1997,3],
  "free_two_symbol_weight": [1400,1400,1600,1600,1800,1800,400],
  "free_complement_symbol_weight": [1500,1500,1500,1500,1500,1500,1000],

  "max_pay_multiple": 1000

}
`
