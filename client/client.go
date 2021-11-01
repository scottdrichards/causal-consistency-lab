package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
)

func main() {

	// Listen
	host := "localhost"

	portOptions := os.Args[2:]
	var listener net.Listener
	var err error
	found := false
	var localPort string = ""
	for _, localPort = range portOptions {
		fmt.Println("Trying client on " + host + ":" + localPort)
		listener, err = net.Listen("tcp", host+":"+localPort)
		// Defer tells main to close the socket when exiting the function
		if err != nil {
			fmt.Println("Port taken")
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

	fmt.Println("Got port", localPort)
	defer listener.Close()

	datacenterAddress := "localhost"
	datacenterPort := os.Args[1]
	datacenterConn, err := net.Dial("tcp", datacenterAddress+":"+datacenterPort)
	if err != nil {
		fmt.Println("Error, couldn't connect to datacenter", err)
		os.Exit(-1)
	}

	// Send DS your listening address
	dsWriter := bufio.NewWriter(datacenterConn)
	_, err = dsWriter.WriteString("client\n")
	if err != nil {
		fmt.Println("Couldn't write to datacenter", err)
		os.Exit(-1)
	}
	_, err = dsWriter.WriteString(host + ":" + localPort + "\n")
	if err != nil {
		fmt.Println("Couldn't write to datacenter", err)
		os.Exit(-1)
	}

	if dsWriter.Flush() != nil {
		fmt.Println("Couldn't flush", err)
		os.Exit(-1)
	}

	dsListen, err := listener.Accept()
	if err != nil {
		fmt.Println("Couldn't accept DS connection", err)
		os.Exit(-1)
	}
	dsReader := bufio.NewReader(dsListen)

	// Send messages to DS
	go func() {
		reader := bufio.NewReader(os.Stdin)
		fmt.Println("Ready to go, start chatting")
		for {
			text, err := reader.ReadString('\n')
			if err != nil {
				fmt.Println("Couldn't write message to datacenter", err)
			} else {
				// Remove read delimiter
				text = text[:len(text)-1]
				// add send delimiter
				_, err := dsWriter.WriteString(text + "\n")
				if err != nil {
					fmt.Println("Couldn't write message to datacenter", err)
				}
				if dsWriter.Flush() != nil {
					fmt.Println("Couldn't flush", err)
					os.Exit(-1)
				}
			}
		}
	}()

	for {
		msg, err := dsReader.ReadString('\n')
		if err != nil {
			fmt.Println("Couldn't read message from datacenter", err)
			os.Exit(-1)
		}
		fmt.Println(msg)
	}
}
