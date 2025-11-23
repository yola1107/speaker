package mahjong

import "time"

// 初始化spin第一个step
func (s *betOrderService) initFirstStepForSpin() error {
	if !s.forRtpBench {
		switch {
		case !s.updateBetAmount():
			return InvalidRequestParams
		case !s.checkBalance():
			return InsufficientBalance
		}
	}
	s.client.SetLastMaxFreeNum(0)
	s.client.ClientOfFreeGame.Reset()
	s.client.ClientOfFreeGame.ResetGeneralWinTotal()
	s.client.ClientOfFreeGame.ResetRoundBonus()
	s.client.ClientOfFreeGame.ResetRoundBonusStaging()
	s.client.ClientOfFreeGame.SetBetAmount(s.betAmount.Round(2).InexactFloat64())
	s.amount = s.betAmount
	s.client.ClientOfFreeGame.SetLastWinId(uint64(time.Now().UnixNano()))
	return nil
}
