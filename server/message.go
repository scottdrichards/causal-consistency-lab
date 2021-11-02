package main

func fullToBasic(in chan MessageFull) chan MessageBasic {
	out := make(chan MessageBasic, 10)
	go func() {
		for message := range in {
			out <- message.MessageBasic
		}
	}()
	return out
}

func msgToString(message MessageFull) string {
	out := basicMsgToString(message.MessageBasic)
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

func basicMsgToString(message MessageBasic) string {
	out := ""
	out += message.MessageID + ">> "
	out += string(message.Body) + "<<"
	return out
}
