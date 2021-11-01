package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
)

type BasicMessage struct {
	MessageID string
	Body      []byte
}
type MessageWithDependencies struct {
	BasicMessage
	Dependencies []string
}

func main() {

	// Listen
	host := "localhost"
	datacenterPorts := os.Args[1:]
	var listener net.Listener
	var err error
	found := false
	var localPort string = ""
	for _, localPort = range datacenterPorts {
		fmt.Println("Trying server on " + host + ":" + localPort)
		listener, err = net.Listen("tcp", host+":"+localPort)
		// Defer tells main to close the socket when exiting the function
		if err != nil {
			fmt.Println("... port taken")
			continue
		} else {
			found = true
			break
		}
	}
	if !found {
		fmt.Println("Could not get a port")
		os.Exit(-1)
	}

	fmt.Println("Listening on port:", localPort)
	defer listener.Close()

	registrationChannel := make(chan Registration, 10)

	go messageBroker(registrationChannel)

	// Connect to other datacenters
	for _, remotePort := range datacenterPorts {
		if remotePort != localPort {
			go datacenterOutgoing(host, remotePort, registrationChannel)
		}
	}

	for {
		connection, err := listener.Accept()

		if err != nil {
			fmt.Println("Error connecting:", err.Error())
		} else {
			fmt.Print("Connection Received from ", connection.RemoteAddr().String())
			reader := bufio.NewReader(connection)
			endpointType, err := reader.ReadString('\n')
			if err != nil {
				fmt.Println("Error reading data", err.Error())
				connection.Close()
			}
			endpointType = endpointType[:len(endpointType)-1]
			fmt.Println(" of type " + endpointType)
			if endpointType == "client" {
				registerClient(connection, reader, registrationChannel)
			} else if endpointType == "datacenter" {
				datacenterIncoming(connection, reader, registrationChannel)
			} else {
				fmt.Println("Invalid endpoint type", endpointType, err)
				connection.Close()
			}
		}
	}
}

// func handleUpdates