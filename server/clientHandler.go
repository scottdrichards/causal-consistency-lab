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

	log.Println("Client connected that listens on " + clientListenAddressPort)

	outGoingConn, err := net.Dial("tcp", clientListenAddressPort)
	if err != nil {
		fmt.Println("Error connecting to client:", err.Error())
		conn.Close()
		return
	}

	// fmt.Println("Registering client handler")
	localFromBroker := make(chan MessageFull, 100)
	localToBroker := make(chan MessageFull, 100)

	registrationChannel <- Registration{
		toBroker:   localToBroker,
		fromBroker: localFromBroker,
	}

	csSubscribeFn, csUpdateFn := clientSateManager()
	// clientStateSubscribe := clientStateFanout(clientStateChan)
	clientToLocal := make(chan MessageBasic, 100)
	go clientListener(conn, reader, clientToLocal)
	go addDeps(clientToLocal, csSubscribeFn(), csUpdateFn, localToBroker)

	messagesReady := make(chan MessageBasic, 10)
	go clientStaging(localFromBroker, csSubscribeFn(), messagesReady)
	go clientSender(outGoingConn, messagesReady, csUpdateFn)
}

func clientSateManager() (func() chan ClientState, func(MessageID)) {
	newIDChan := make(chan MessageID, 10)
	clientStateChan := make(chan ClientState, 100)

	// This will take new ids and generate a new state from them
	go func() {
		clientState := ClientState{}
		for newID := range newIDChan {
			found := false
			for i, oldID := range clientState {
				if newID.Host == oldID.Host {
					found = true
					if newID.Clock > oldID.Clock {
						clientState[i] = newID
					} else {
						fmt.Println("State already newer")
					}
					break
				}
			}
			if !found {
				clientState = append(clientState, newID)
			}
			clientStateChan <- clientState
		}
	}()

	updateClientState := func(newID MessageID) {
		newIDChan <- newID
	}

	addSubscriber := make(chan chan ClientState, 5)

	// This will add subscribers and will also send updates to them when the come
	go func() {
		subscribers := []chan ClientState{}
		for {
			select {
			case newSub := <-addSubscriber:
				subscribers = append(subscribers, newSub)
			case newState := <-clientStateChan:
				for _, subscriber := range subscribers {
					subscriber <- newState
				}
			}
		}
	}()

	csSubscribeFn := func() chan ClientState {
		localCSChan := make(chan ClientState, cap(clientStateChan))
		addSubscriber <- localCSChan
		return localCSChan
	}
	return csSubscribeFn, updateClientState
}

func addDeps(msgsIn <-chan MessageBasic, clientStateChan <-chan ClientState, updateCS func(MessageID), msgsOut chan<- MessageFull) {
	clientState := ClientState{}
	for {
		select {
		case message := <-msgsIn:
			csCopy := append(ClientState{}, clientState...)
			msgsOut <- MessageFull{
				MessageBasic: message,
				Dependencies: csCopy,
			}
			updateCS(message.ID)
		case clientState = <-clientStateChan:

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
		// Remove delimiter
		msgBody = msgBody[:len(msgBody)-1]
		message := MessageBasic{
			ID: MessageID{
				Host:  clientID,
				Clock: messageCounter,
			},
			Body: []byte(msgBody),
		}
		fmt.Println("Received message from client:", message.ToString())
		messageChannel <- message
		messageCounter++
	}
}

func dependenciesSatisfied(dependencies []MessageID, seen ClientState) bool {
	for _, dependency := range dependencies {
		found := false
		for _, stateDatum := range seen {
			if stateDatum.Host == dependency.Host && stateDatum.Clock >= dependency.Clock {
				found = true
				break
			}
		}
		if !found {
			fmt.Println("State: ", seen.ToString(), ". Missing: "+dependency.ToString())
			return false
		}
	}
	return true
}

func clientStaging(availableMessages <-chan MessageFull, clientStateChan <-chan ClientState, messagesReady chan<- MessageBasic) {
	clientState := ClientState{}
	queuedMessages := []MessageFull{}

	trySendingMessage := func(message MessageFull) bool {
		if dependenciesSatisfied(message.Dependencies, clientState) {
			messagesReady <- message.MessageBasic
			return true
		} else {
			fmt.Println("-----------------------\nDepedencies not satisfied for msg: " + message.ToString())
		}
		return false
	}
	trySendingMessages := func() {
		newMessages := []MessageFull{}
		for _, message := range queuedMessages {
			if !trySendingMessage(message) {
				newMessages = append(newMessages, message)
			}
		}
		queuedMessages = newMessages
	}

	for {
		select {
		case message := <-availableMessages:
			fmt.Println("New messgae: ", message.ToString())
			if !trySendingMessage(message) {
				queuedMessages = append(queuedMessages, message)
			}
		case cs := <-clientStateChan:
			fmt.Println("New state: ", cs.ToString())
			clientState = append(ClientState{}, cs...)
			trySendingMessages()
		}
	}
}

// This function just sends messages
func clientSender(conn net.Conn, messages <-chan MessageBasic, updateState func(MessageID)) {
	// I control the connection, so close it when I'm done
	defer conn.Close()
	writer := bufio.NewWriter(conn)

	// Wait for new messages to come in to the messageChannel
	for message := range messages {
		fmt.Println("Sending message: " + message.ToString())
		_, err := writer.Write(append(message.Body, '\n'))
		if err != nil {
			fmt.Println(err)
			return
		}

		if writer.Flush() != nil {
			fmt.Println("Couldn't flush", err)
		} else {
			// Let everyone know it has been sent
			updateState(message.ID)
		}
	}
}
