package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
)

var osNewLine string = "\r\n"

func main() {

	fmt.Println("##################")
	fmt.Println("##### CLIENT #####")
	fmt.Println("##################")
	// Listen
	host := "localhost"

	portOptions := os.Args[2:]
	var listener net.Listener
	var err error
	found := false
	var localPort string = ""
	// Find a port that is available to listen on
	for _, localPort = range portOptions {
		listener, err = net.Listen("tcp", host+":"+localPort)
		if err != nil {
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

	fmt.Print("Listening on port ", localPort, "\n\n\n")
	defer listener.Close()

	// Connect to datacenter and tell it your address that you listen on
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

	fmt.Println("Waiting for datacenter connection...")
	dsListen, err := listener.Accept()
	if err != nil {
		fmt.Println("Couldn't accept DS connection", err)
		os.Exit(-1)
	}
	dsReader := bufio.NewReader(dsListen)

	// This is the loop, just wait for input and send it to the datacenter
	go func() {
		reader := bufio.NewReader(os.Stdin)
		fmt.Println("Ready to go, start chatting")
		for {
			text, err := reader.ReadString('\n')
			// Remove new line characters
			for i := 0; text[len(text)-1-i] == osNewLine[len(osNewLine)-1-i]; i++ {
				text = text[:len(text)-1]
			}
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

	// This is where we deal with incoming messages from the datacenter. The datacenter
	// takes care of dependencies etc.
	for {
		msg, err := dsReader.ReadString('\n')
		if err != nil {
			fmt.Println("Couldn't read message from datacenter", err)
			os.Exit(-1)
		}
		msg = msg[:len(msg)-1]
		fmt.Println(msg)
	}
}
