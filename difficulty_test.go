package xfsgo

import (
	"fmt"
	"math/big"
	"testing"
)

func fullhex(a *big.Int) string {
	ab := a.Bytes()
	var t [32]byte
	copy(t[len(t)-len(ab):], ab)
	return fmt.Sprintf("%x", t)
}

func TestAccumulateRewards(t *testing.T) {
	target1 := new(big.Int).Lsh(big0xff, (32-4)*8+16)
	target2 := new(big.Int).Lsh(big0xff, (32-4)*8)
	_ = target2
	tb := target1.Bytes()
	tb2 := target2.Bytes()
	var targetBs [32]byte
	var targetBs2 [32]byte
	copy(targetBs[len(targetBs)-len(tb):], tb)
	copy(targetBs2[len(targetBs2)-len(tb2):], tb2)
	bits := BigByZip(target1)
	bits2 := BigByZip(target2)
	t.Logf("bits: %d", bits)
	t.Logf("bits2: %d", bits2)
	t.Logf(" target: %x", targetBs)
	t.Logf("target2: %x", targetBs2)
}

func Test_mainnetG(t *testing.T) {
	bign0 := BitsUnzip(4278190109)
	t.Logf("target: %s", fullhex(bign0))
	// blocksPerRetarget := uint64(targetTimespan / targetTimePerBlock)
	// t.Logf("blocksPerRetarget: %d", blocksPerRetarget)
	//minRetargetTimespan := targetTimespan / adjustmentFactor
	//maxRetargetTimespan := targetTimespan * adjustmentFactor
	//minTarget := new(big.Int).Mul(bign0, big.NewInt(minRetargetTimespan))
	//maxTarget := new(big.Int).Mul(bign0, big.NewInt(maxRetargetTimespan))
	//newTargetMin := new(big.Int).Mul(bign0, minTarget)
	//newTargetMax := new(big.Int).Mul(bign0, maxTarget)
	//newTargetMin.Div(newTargetMin, big.NewInt(targetTimespan))
	//newTargetMax.Div(newTargetMax, big.NewInt(targetTimespan))
	//t.Logf("min: %s", fullhex(newTargetMin))
	//t.Logf("max: %s", fullhex(newTargetMax))
}

func Test_mainnetA(t *testing.T) {
	hr := CalcHashRateByBits(4278190109)
	t.Logf("x: %s", hr.String())
	bn := CalcWorkloadByBits(4278190109)
	t.Logf("x: %s", bn.Text(16))
	n := CalcDifficultyByBits(4278190109)

	t.Logf("n: %f", n)
	//bign0 := BitsUnzip(4278190109)
	////t.Logf("target: %s", fullhex(bign0))
	//n := BigByZip(bign0)
	//t.Logf("target: %s", bign0.Text(16))
	//t.Logf("n: %d", n)
}
