package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
)

// Registers a client newly connected on conn
func registerClient(conn net.Conn, registrationChannel chan Registration) {

	// Read the address and port of the client (that the client will listen for updates on)
	buffer := bufio.NewReader(conn)
	clientListenAddressPort, err := buffer.ReadString('\n')
	if err != nil {
		fmt.Println("Client left.")
		conn.Close()
		return
	}

	log.Println("Client connection from " + conn.RemoteAddr().String() + " that listens on " + clientListenAddressPort)

	outGoingConn, err := net.Dial("tcp", clientListenAddressPort)
	if err != nil {
		fmt.Println("Error connecting to client:", err.Error())
		conn.Close()
		return
	}

	clientIn := make(chan Message, 4)
	clientOut := make(chan Message, 4)

	registrationChannel <- Registration{
		toBroker:   clientIn,
		fromBroker: clientOut,
	}

	go receiveClientUpdate(conn, clientIn)
	go sendClientUpdate(outGoingConn, clientOut)
}

func receiveClientUpdate(conn net.Conn, messageChannel chan<- Message) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	for {
		jsonString, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Error receiving message from client", err)
			return
		} else {
			var clientMessage Message
			if json.Unmarshal([]byte(jsonString), &clientMessage) != nil {
				fmt.Println("Error parsing message from client:", jsonString, err)
				return
			}

			var message Message
			message.Dependencies = clientMessage.Dependencies
			message.Body = clientMessage.Body
			message.MessageID = clientMessage.MessageID

			fmt.Println("Received message " + clientMessage.MessageID + " from " + conn.LocalAddr().String() + ":" + string(clientMessage.Body))
			messageChannel <- message
		}
	}
}

// This function keeps track of client state and sends messages when appropriate
func sendClientUpdate(conn net.Conn, messageChannel <-chan Message) {
	seenMsgs := map[string]bool{}
	var queuedMsgs []Message
	// I control the connection, so close it when I'm done
	defer conn.Close()
	writer := bufio.NewWriter(conn)

	// Wait for new messages to come in to the messageChannel
	for newMessage := range messageChannel {
		_, alreadySent := seenMsgs[newMessage.MessageID]
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
			var stillQueued []Message
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
					// Send to client
					jsonBytes, err := json.Marshal(newMessage)
					if err != nil {
						fmt.Println(err)
						return
					} else {
						_, err := writer.Write(jsonBytes)
						if err != nil {
							fmt.Println(err)
							return
						}
					}
					seenMsgs[message.MessageID] = true
					progress = true
				} else {
					// We can't process it, so put it back in a queue
					stillQueued = append(stillQueued, message)
				}
			}
			queuedMsgs = stillQueued
		}
	}
}
