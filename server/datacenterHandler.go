package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"strconv"
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
	sendChannel := make(chan MessageFull, 100)
	registrationChannel <- Registration{
		toBroker:   nil,
		fromBroker: sendChannel,
	}

	prtNum, _ := strconv.Atoi(port)
	rng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(prtNum)))
	randomDelay := func(maxDelay uint32) {
		waitSeconds := rng.Uint32() % maxSecondsWait
		fmt.Println("delaying ", waitSeconds, " seconds...")

		time.Sleep(time.Duration(waitSeconds) * time.Second)
	}

	readyMessages := make(chan MessageFull, 100)
	go datacenterSendMessage(conn, readyMessages)
	for message := range sendChannel {
		fmt.Println("Received message to forward to other datacenter " + message.ToString())
		go func(message MessageFull) {
			randomDelay(maxSecondsWait)
			fmt.Println("Delay over")
			readyMessages <- message
		}(message)
	}
}

func datacenterSendMessage(conn net.Conn, readyMessages <-chan MessageFull) {

	defer conn.Close()
	writer := bufio.NewWriter(conn)
	writer.WriteString("datacenter\n")

	if writer.Flush() != nil {
		fmt.Println("Couldn't flush")
	}
	for message := range readyMessages {
		fmt.Println("Sending message: " + message.ToString())
		jsonMsg, err := json.Marshal(message)
		if err != nil {
			fmt.Println("Error creating message", message, err)
		} else {
			_, err := writer.WriteString(string(jsonMsg) + "\n")
			if err != nil {
				fmt.Println("Error creating message", message, err)
			}
			fmt.Println("Sending message to other datacenter", message.ToString())

			if writer.Flush() != nil {
				fmt.Println("Couldn't flush", err)
			}

		}
	}
}

// Receives updates from a specific datacenter and sends the result along messagechannel
func datacenterIncoming(conn net.Conn, reader *bufio.Reader, registrationChannel chan<- Registration) {
	receiveChannel := make(chan MessageFull, 100)
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
		var message MessageFull
		if json.Unmarshal([]byte(jsonStr), &message) != nil {
			fmt.Println("Could not unpack JSON message", err)
			return
		}
		fmt.Println("Received message from other datacenter" + message.ToString())

		receiveChannel <- message
	}
}
