package main

import "fmt"

type MessageID struct {
	Host  string
	Clock int
}

func (id MessageID) ToString() string {
	return id.Host + "{" + fmt.Sprint(id.Clock) + "}"
}

type MessageBasic struct {
	ID   MessageID
	Body []byte
}

func (m MessageBasic) ToString() string {
	return "[" + m.ID.ToString() + "]>>" + string(m.Body)
}

type ClientState []MessageID

func (cs ClientState) ToString() string {
	out := ""
	for _, entry := range cs {
		out += entry.ToString()
	}
	return out
}

type MessageFull struct {
	MessageBasic
	Dependencies ClientState
}

func (m MessageFull) ToString() string {
	depString := "\n\n-----------------------\n"
	depString += "Message ID: " + m.MessageBasic.ID.ToString() + "\n"
	depString += "Dependencies:\n"
	for _, dep := range m.Dependencies {
		depString += "\t" + dep.ToString() + "\n"
	}
	depString += "Body: " + string(m.MessageBasic.Body)

	depString += "\n-----------------------\n\n"
	return depString
}
