package sjnws3

import (
	"errors"
	"fmt"
)

var symbolList = []int{_Q, _K, _A, _cangBaoTu, _zhiNanZhen, _qiang, _nvWang}

var _colList_5 = []string{"4,0", "4,1", "4,2", "4,3"}
var _colList_4 = []string{"3,0", "3,1", "3,2", "3,3"}
var _colList_3 = []string{"2,0", "2,1", "2,2", "2,3"}
var _colList_2 = []string{"1,0", "1,1", "1,2", "1,3"}
var _colList_1 = []string{"0,0", "0,1", "0,2", "0,3"}
var mapColList = map[int][]string{
	0: _colList_1,
	1: _colList_2,
	2: _colList_3,
	3: _colList_4,
	4: _colList_5,
}
var PositionMap = map[string]int{
	"0,0": 0,
	"0,1": 1,
	"0,2": 2,
	"0,3": 3,
	"1,0": 0,
	"1,1": 1,
	"1,2": 2,
	"1,3": 3,
	"2,0": 0,
	"2,1": 1,
	"2,2": 2,
	"2,3": 3,
	"3,0": 0,
	"3,1": 1,
	"3,2": 2,
	"3,3": 3,
	"4,0": 0,
	"4,1": 1,
	"4,2": 2,
	"4,3": 3,
}

var sceneDataKeyPrefix = fmt.Sprintf("scene-%d:", _gameID)

var (
	InternalServerError  = errors.New("internal server error")
	InvalidRequestParams = errors.New("invalid request params")
	InsufficientBalance  = errors.New("insufficient balance")
	ActError             = errors.New("bonusNum must be select")
)

var rowList = []int{_row1, _row2, _row3, _row4}
