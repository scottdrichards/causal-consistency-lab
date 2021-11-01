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

	clientIn := make(chan MessageWithDependencies, 4)
	clientOut := make(chan MessageWithDependencies, 4)
	// Used to keep rxer apprised of dependencies of sender
	clientPrivate := make(chan MessageWithDependencies, 4)

	registrationChannel <- Registration{
		toBroker:   clientIn,
		fromBroker: clientOut,
	}

	go receiveClientUpdate(conn, reader, clientIn, clientPrivate)
	go sendClientUpdate(outGoingConn, clientOut, clientPrivate)
}

func receiveClientUpdate(conn net.Conn, reader *bufio.Reader, messageChannel chan<- MessageWithDependencies, clientMessagesSeen <-chan MessageWithDependencies) {
	defer conn.Close()
	internalChannel := make(chan BasicMessage, 4)

	// We keep track of dependencies here
	addDependenciesToMsg := func() {
		// Bookkeeping to keep track of what the most current dependency list is
		dependencies := []string{}
		updateDependencies := func(message MessageWithDependencies) {
			newDependencies := []string{}
			// Filter out redundant dependencies
			for _, oldDep := range dependencies {
				// Does this message depend on old messages? If so, we can delete the old dependency
				found := false
				for _, newDep := range message.Dependencies {
					if newDep == oldDep {
						found = true
						break
					}
				}
				if !found {
					// The old dependency is still relevant
					newDependencies = append(newDependencies, oldDep)
				}
			}
			newDependencies = append(newDependencies, message.MessageID)
			dependencies = newDependencies
		}

		// This is where we do the magic
		select {
		case message := <-internalChannel:
			messageChannel <- MessageWithDependencies{
				BasicMessage: message,
				Dependencies: dependencies,
			}
		case messageSeen := <-clientMessagesSeen:
			updateDependencies(messageSeen)
		}
	}
	go addDependenciesToMsg()

	clientID := conn.RemoteAddr().String()
	messageCounter := 0

	for {
		msgBody, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Error receiving message from client", err)
			return
		} else {
			msgBody = msgBody[:len(msgBody)-1]
			message := BasicMessage{
				MessageID: clientID + "M" + fmt.Sprint(messageCounter),
				Body:      []byte(msgBody),
			}
			fmt.Println("Received message " + message.MessageID + " from " + clientID + ":" + string(message.Body))
			internalChannel <- message
			messageCounter++
		}

	}
}

// This function keeps track of client state and sends messages when appropriate
func sendClientUpdate(conn net.Conn, messageChannel <-chan MessageWithDependencies, clientMessagesSeen chan<- MessageWithDependencies) {
	seenMsgs := map[string]bool{}
	var queuedMsgs []MessageWithDependencies
	// I control the connection, so close it when I'm done
	defer conn.Close()
	writer := bufio.NewWriter(conn)

	// Wait for new messages to come in to the messageChannel
	for newMessage := range messageChannel {
		_, alreadySent := seenMsgs[newMessage.MessageID]
		clientMessagesSeen <- newMessage
		if alreadySent {
			// The client already has this one
			break
		}
		// Add this message to the queue
		queuedMsgs = append(queuedMsgs, newMessage)

		// Keep looping over queued messages until we don't make progress
		progress := true
		for progress {
			progress = false
			var stillQueued []MessageWithDependencies
			// Try to send each queued message
			for _, message := range queuedMsgs {
				// See if the dependencies are satisfied
				satisfiedDeps := true
				for _, dependency := range message.Dependencies {
					_, seen := seenMsgs[dependency]
					if !seen {
						satisfiedDeps = false
						break
					}
				}

				if satisfiedDeps {
					fmt.Println("Satisfied deps for " + message.MessageID + ", sending...")

					_, err := writer.Write(append(message.Body, '\n'))
					if err != nil {
						fmt.Println(err)
						return
					}

					if writer.Flush() != nil {
						fmt.Println("Couldn't flush", err)
					}
					seenMsgs[message.MessageID] = true
					progress = true
				} else {
					fmt.Println("Cannot send " + message.MessageID)

					// We can't process it, so put it back in a queue
					stillQueued = append(stillQueued, message)
				}
			}
			fmt.Println("There are still " + fmt.Sprint(len(stillQueued)) + " queued messages waiting for dependencies")
			queuedMsgs = stillQueued
		}
	}
}
