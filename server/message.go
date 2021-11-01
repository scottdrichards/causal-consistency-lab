package main

func msgToString(message MessageWithDependencies) string {
	out := basicMsgToString(message.BasicMessage)
	out += " deps["
	for i, dep := range message.Dependencies {
		out += dep
		if i < len(message.Dependencies)-1 {
			out += ", "
		}
	}
	out += "]"
	return out
}

func basicMsgToString(message BasicMessage) string {
	out := ""
	out += message.MessageID + ">> "
	out += string(message.Body) + "<<"
	return out
}
