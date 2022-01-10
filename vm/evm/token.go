package evm

import (
	"math/big"
	"xfsgo/common"
)

type Token interface {
	Name() string
	Symbol() string
	Decimals() uint8
	TotalSupply() *big.Int
	BalanceOf(common.Address) *big.Int
	Transfer(common.Address, common.Address) bool
	TransferFrom(common.Address, common.Address, *big.Int) bool
	Approve(common.Address, *big.Int) bool
	Allowance(common.Address, common.Address) bool
}

type token struct {
	address     common.Address
	name        string
	symbol      string
	decimals    uint8
	totalSupply *big.Int
	owner       common.Address
	sender      common.Address
}

func (t *token) Name() string {
	return t.name
}
func (t *token) Symbol() string {
	return t.symbol
}
func (t *token) Decimals() uint8 {
	return t.decimals
}
func (t *token) TotalSupply() *big.Int {
	return t.totalSupply
}
func (t *token) BalanceOf(common.Address) *big.Int {
	return nil
}
func (t *token) Transfer(common.Address, common.Address) bool {
	return false
}
func (t *token) TransferFrom(common.Address, common.Address, *big.Int) bool {
	return false
}
func (t *token) Approve(common.Address, *big.Int) bool {
	return false
}
func (t *token) Allowance(common.Address, common.Address) bool {
	return false
}
