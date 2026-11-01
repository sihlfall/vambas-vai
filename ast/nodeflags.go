package ast

// NodeFlags uses the same underlying type as typescript-go's AST node flags.
type NodeFlags uint32

const (
	NodeFlagsNone                          NodeFlags = 0
	NodeFlagsConst                         NodeFlags = 1 << 1
	NodeFlagsContainsThis                  NodeFlags = 1 << 7
	NodeFlagsHasImplicitReturn             NodeFlags = 1 << 8
	NodeFlagsThisNodeHasError              NodeFlags = 1 << 15
	NodeFlagsThisNodeOrAnySubNodesHasError NodeFlags = 1 << 17
	NodeFlagsUnreachable                   NodeFlags = 1 << 27
)
