package xfsgo

import (
	"fmt"
	"math/big"
)

type GasPool big.Int

func (gp *GasPool) AddGas(v *big.Int) {
	i := (*big.Int)(gp)
	i.Add(i, v)
}

func (gp *GasPool) GetGas() *big.Int {
	return (*big.Int)(gp)
}

func (gp *GasPool) String() string {
	i := (*big.Int)(gp)
	return fmt.Sprintf("%s", i)
}
func (gp *GasPool) SubGas(v *big.Int) error {
	i := (*big.Int)(gp)
	if i.Cmp(v) < 0 {
		return GasPoolOutErr
	}
	i.Sub(i, v)
	return nil
}
