package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
)

// Registers a client newly connected on conn
func registerClient(conn net.Conn, reader *bufio.Reader, registrationChannel chan Registration) {

	clientListenAddressPort, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("Client left.")
		conn.Close()
		return
	}
	clientListenAddressPort = clientListenAddressPort[:len(clientListenAddressPort)-1]

	log.Println("Client connection from " + conn.RemoteAddr().String() + " that listens on " + clientListenAddressPort)

	outGoingConn, err := net.Dial("tcp", clientListenAddressPort)
	if err != nil {
		fmt.Println("Error connecting to client:", err.Error())
		conn.Close()
		return
	}

	seenForDeps := make(chan MessageFull, 10)
	seenForStaging := make(chan MessageFull, 10)
	seenWatcher := func(channel chan MessageFull) chan MessageFull {
		out := make(chan MessageFull, 10)
		go func() {
			for message := range channel {
				out <- message
				seenForDeps <- message
				seenForStaging <- message
			}
		}()
		return out
	}

	fmt.Println("Registering client handler")
	localFromBroker := make(chan MessageFull, 10)
	localToBroker := make(chan MessageFull, 10)

	registrationChannel <- Registration{
		toBroker:   seenWatcher(localToBroker),
		fromBroker: localFromBroker,
	}

	clientToLocal := make(chan MessageBasic, 10)
	go clientListener(conn, reader, clientToLocal)
	go addDeps(clientToLocal, seenForDeps, localToBroker)

	readyMessages := make(chan MessageFull, 10)
	go clientStaging(localFromBroker, seenForStaging, readyMessages)

	go clientSender(outGoingConn, fullToBasic(seenWatcher(readyMessages)))
}

func addDeps(msgsIn <-chan MessageBasic, seenIn <-chan MessageFull, msgsOut chan<- MessageFull) {
	depedencies := []string{}
	for {
		select {
		case message := <-msgsIn:
			msgsOut <- MessageFull{
				MessageBasic: message,
				Dependencies: depedencies,
			}
		case message := <-seenIn:
			updateDependencies := []string{message.MessageID}
			for _, oldDep := range depedencies {
				found := false
				for _, redundantDep := range message.Dependencies {
					if redundantDep == oldDep {
						found = true
						break
					}
				}
				if !found {
					updateDependencies = append(updateDependencies, oldDep)
				}
			}
			depedencies = updateDependencies
		}
	}
}

func clientListener(conn net.Conn, reader *bufio.Reader, messageChannel chan<- MessageBasic) {
	clientID := conn.RemoteAddr().String()[10:]
	defer conn.Close()

	messageCounter := 0

	for {
		msgBody, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Error receiving message from client", err)
			return
		}
		msgBody = msgBody[:len(msgBody)-1]
		message := MessageBasic{
			MessageID: clientID + "M" + fmt.Sprint(messageCounter),
			Body:      []byte(msgBody),
		}
		fmt.Println("Received message:", basicMsgToString(message))
		messageChannel <- message
		messageCounter++
	}
}

func dependenciesSatisfied(dependencies []string, seen map[string]bool) bool {
	for _, dependency := range dependencies {
		if _, found := seen[dependency]; !found {
			return false
		}
	}
	return true
}

func clientStaging(availableMessages <-chan MessageFull, seenMsgChan <-chan MessageFull, readyMessages chan<- MessageFull) {
	seenMessages := map[string]bool{}
	queuedMessages := []MessageFull{}

	for {
		select {
		case message := <-availableMessages:
			queuedMessages = append(queuedMessages, message)
			progress := true
			for progress {
				progress = false
				messagesStillRemaining := []MessageFull{}
				for _, message := range queuedMessages {
					if dependenciesSatisfied(message.Dependencies, seenMessages) {
						progress = true
						readyMessages <- message
					} else {
						messagesStillRemaining = append(messagesStillRemaining, message)
					}
				}
				queuedMessages = messagesStillRemaining
			}
		case message := <-seenMsgChan:
			seenMessages[message.MessageID] = true
		}
	}
}

// This function just sends messages
func clientSender(conn net.Conn, messages <-chan MessageBasic) {
	// I control the connection, so close it when I'm done
	defer conn.Close()
	writer := bufio.NewWriter(conn)

	// Wait for new messages to come in to the messageChannel
	for message := range messages {
		_, err := writer.Write(append(message.Body, '\n'))
		if err != nil {
			fmt.Println(err)
			return
		}

		if writer.Flush() != nil {
			fmt.Println("Couldn't flush", err)
		}
	}
}
