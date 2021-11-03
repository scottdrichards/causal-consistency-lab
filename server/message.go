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
	depString := "|"
	for _, dep := range m.Dependencies {
		depString += dep.ToString()
		depString += "|"
	}
	return depString + m.MessageBasic.ToString()
}
