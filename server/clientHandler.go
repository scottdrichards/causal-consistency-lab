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
	// Remove delimiter
	clientListenAddressPort = clientListenAddressPort[:len(clientListenAddressPort)-1]

	log.Println("Client connected that listens on " + clientListenAddressPort)

	// Call the client for outgoing communications
	outGoingConn, err := net.Dial("tcp", clientListenAddressPort)
	if err != nil {
		fmt.Println("Error connecting to client:", err.Error())
		conn.Close()
		return
	}

	// Build channels to communicate with the message broker
	localFromBroker := make(chan MessageFull, 100)
	localToBroker := make(chan MessageFull, 100)

	registrationChannel <- Registration{
		toBroker:   localToBroker,
		fromBroker: localFromBroker,
	}

	// The client state manager creates channels and state managers
	// which are accessible via the csSubscribeFn and csUpdateFn
	// csSubscribeFn: generates a channel that will spit out updates to state
	// csUpdateFn: takes in a MessageID and updates the state accordingly
	csSubscribeFn, csUpdateFn := clientSateManager()

	// A simple channle for the client listener to communicate to the
	// add dependency function
	clientToLocal := make(chan MessageBasic, 100)

	// Basic function that listens for messages from the client
	go clientListener(conn, reader, clientToLocal)

	// Adds client dependencies based on client state, also updates
	// client state for outgoing messages
	go addDeps(clientToLocal, csSubscribeFn(), csUpdateFn, localToBroker)

	// Outgoing messages to the client. messagesReady is a channel to communicate
	// messages between the staging area and the sending process
	messagesReady := make(chan MessageBasic, 100)
	// This is where messages are staged, awaiting for any dependencies to arrive
	go clientStaging(localFromBroker, csSubscribeFn(), messagesReady)
	// Simple function that sends a message over the connection
	go clientSender(outGoingConn, messagesReady, csUpdateFn)
}

// This builds a client state management system, returning a tuple of methods to operate
// on the system. The first of the tuple is a function that generates a channel
// that is subscribed to updates of the client state. The second function receives a messageID
// which will then generate a new state based on the messageID. This should be called
// whenever the client sees a new message
func clientSateManager() (func() chan ClientState, func(MessageID)) {
	// This channel is for updating the client state based on new IDs
	newIDChan := make(chan MessageID, 100)
	// This channel is the core channel for distributing state changes
	// Other "subscription" channels will branch off of this
	clientStateChan := make(chan ClientState, 100)

	// This is an autonomous function that will run in the background,
	// that is the core state tracker. It ingests newIDChan and
	// pushes new states onto clientStateChan
	go func() {
		clientState := ClientState{}
		for newID := range newIDChan {
			found := false
			// Find out if the new clock is relevant to any clocks
			// of the current state
			for i, oldID := range clientState {
				if newID.Host == oldID.Host {
					found = true
					// .. and update the clock if the new one is
					// more recent (it should always be so, but just in
					// case)
					if newID.Clock > oldID.Clock {
						clientState[i] = newID
					} else {
						fmt.Println("State already newer, no need to update state")
					}
					break
				}
			}
			if !found {
				clientState = append(clientState, newID)
			}
			// This is a new clock (i.e., the client has a
			// dependency relevant to a new other client's message)
			clientStateChan <- clientState
		}
	}()

	// This is a returned function for updating the client state. An operator
	// will just pass it a new MessageID and it will update the state
	updateClientState := func(newID MessageID) {
		newIDChan <- newID
	}

	// addSubscriber is a bookkeeping channel to add new subscribers to the client
	// state channel (fanout paradigm). It is only used internally
	addSubscriber := make(chan chan ClientState, 5)

	// This is a background function that does the fanout operation, keeping track of
	// every subscriber based on the addSubscriber channel and sending them updates
	// to the clientStateChan channel
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

	// This is a returned utility function for generating a new subscriber and returning
	// the relevant fanout channel
	csSubscribeFn := func() chan ClientState {
		localCSChan := make(chan ClientState, cap(clientStateChan))
		addSubscriber <- localCSChan
		return localCSChan
	}
	return csSubscribeFn, updateClientState
}

// This function ingests MessageBasic items - ie those received from the client
// and applies dependencies based on the client's current state. It will also
// update the client state based on the messages that are sent
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

// Ingests messages over the socket from the client and posts them on the messageChannel
func clientListener(conn net.Conn, reader *bufio.Reader, messageChannel chan<- MessageBasic) {
	clientID := conn.RemoteAddr().String()[10:]
	defer conn.Close()

	// messageCounter is used as a lambart clock for how many messages this client has received
	// it is also the identifier for the message
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

// Determines if a messageID's dependencies are satisfied
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
			fmt.Println("Dependency not satisfied!\nState: ", seen.ToString(), ". Missing: "+dependency.ToString())
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
			fmt.Println("... for message:", message.ToString())
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
			fmt.Println("Staging-new message: ", message.ToString())
			if !trySendingMessage(message) {
				queuedMessages = append(queuedMessages, message)
			}
		case cs := <-clientStateChan:
			fmt.Println("Staging-New state: ", cs.ToString())
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
		fmt.Println("Sending message to client: " + message.ToString())
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
