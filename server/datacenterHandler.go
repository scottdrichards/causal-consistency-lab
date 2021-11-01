package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"time"
)

const maxSecondsWait = 10

// Sends message updates from messageChannel to specific datacenter specified by address and port
func datacenterOutgoing(address string, port string, registrationChannel chan<- Registration) {

	var conn net.Conn
	var err error
	backoff := 0
	for {
		conn, err = net.Dial("tcp", address+":"+port)
		if err == nil {
			break
		}
		// Exponential backoff
		time.Sleep(time.Duration(2^backoff) * time.Second)
		backoff++
	}
	sendChannel := make(chan MessageWithDependencies, 5)
	registrationChannel <- Registration{
		toBroker:   nil,
		fromBroker: sendChannel,
	}

	defer conn.Close()
	writer := bufio.NewWriter(conn)
	writer.WriteString("datacenter\n")

	if writer.Flush() != nil {
		fmt.Println("Couldn't flush", err)
	}
	randomDelay := func(maxDelay uint32) {
		rng := rand.New(rand.NewSource(time.Now().UnixNano()))
		waitSeconds := rng.Uint32() % maxSecondsWait
		time.Sleep(time.Duration(waitSeconds) * time.Second)
	}

	for message := range sendChannel {
		fmt.Println("Received message from another DS", message)
		fmt.Println("delaying...")
		randomDelay(maxSecondsWait)
		jsonMsg, err := json.Marshal(message)
		if err != nil {
			fmt.Println("Error creating message", message, err)
		} else {
			_, err := writer.WriteString(string(jsonMsg) + "\n")
			if err != nil {
				fmt.Println("Error creating message", message, err)
			}
			fmt.Println("Sending message to other datacenter")

			if writer.Flush() != nil {
				fmt.Println("Couldn't flush", err)
			}

		}
	}
}

// Receives updates from a specific datacenter and sends the result along messagechannel
func datacenterIncoming(conn net.Conn, reader *bufio.Reader, registrationChannel chan<- Registration) {
	receiveChannel := make(chan MessageWithDependencies, 5)
	defer close(receiveChannel)
	registrationChannel <- Registration{
		toBroker:   receiveChannel,
		fromBroker: nil,
	}

	defer conn.Close()
	for {
		jsonStr, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Trouble receiving message", err)
			return
		}
		var message MessageWithDependencies
		if json.Unmarshal([]byte(jsonStr), &message) != nil {
			fmt.Println("Could not unpack JSON message", err)
			return
		}
		fmt.Println("Received message from other datacenter", message)

		receiveChannel <- message
	}
}
