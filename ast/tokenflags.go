package ast

type TokenFlags int32

const (
	TokenFlagsNone         TokenFlags = 0
	TokenFlagsUnterminated TokenFlags = 1 << 2
)
