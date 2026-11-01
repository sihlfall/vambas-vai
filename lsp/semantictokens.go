package lsp

// Keep the server's stable ordering, filtering types/modifiers the client cannot
// interpret, as in typescript-go's capability-negotiated semantic token legend.
func supportedSemanticTokens(server, client []string) []string {
	supported := make(map[string]bool, len(client))
	for _, name := range client {
		supported[name] = true
	}
	result := []string{}
	for _, name := range server {
		if supported[name] {
			result = append(result, name)
		}
	}
	return result
}
