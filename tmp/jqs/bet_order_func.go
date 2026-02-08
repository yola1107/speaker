package jqs

import (
	"egame-grpc/game/common/pb"
	"egame-grpc/utils/json"

	"google.golang.org/protobuf/proto"
)

func (s *betOrderService) int64GridToPbBoard(grid int64Grid) *pb.Board {
	elements := make([]int64, _rowCount*_colCount)
	for row := 0; row < _rowCount; row++ {
		for col := 0; col < _colCount; col++ {
			elements[row*_colCount+col] = grid[row][col]
		}
	}
	return &pb.Board{Elements: elements}
}

func (s *betOrderService) MarshalData(data proto.Message) ([]byte, string, error) {
	pbData, err := proto.Marshal(data)
	if err != nil {
		return nil, "", err
	}
	jsonData, err := json.CJSON.MarshalToString(data)
	if err != nil {
		return nil, "", err
	}
	return pbData, jsonData, nil
}
