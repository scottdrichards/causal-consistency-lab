package main

import "fmt"

type ConsolidationMessage struct {
	channelID int
	message   MessageFull
}

type DistributorReg struct {
	channelID      int
	messageChannel chan MessageFull
}

type Registration struct {
	toBroker   chan MessageFull // Messages going to broker
	fromBroker chan MessageFull // Messages leaving datacenter
}

func messageBroker(channelRegister <-chan Registration) {
	endpointChan := make(chan DistributorReg, 10)
	aggregateMsgChannel := make(chan ConsolidationMessage, 10)
	go distributor(aggregateMsgChannel, endpointChan)
	currentID := 0
	fmt.Println("Waiting for client")
	for newClient := range channelRegister {
		fmt.Println("New client registered")
		if newClient.toBroker != nil {
			go consolidator(newClient.toBroker, aggregateMsgChannel, currentID)
		}
		if newClient.fromBroker != nil {
			endpointChan <- DistributorReg{channelID: currentID, messageChannel: newClient.fromBroker}
		}
		currentID++
	}
}

func consolidator(fromSource <-chan MessageFull, aggregateMsgChannel chan<- ConsolidationMessage, channelID int) {
	defer fmt.Println("Consolidator ended")
	for message := range fromSource {
		fmt.Println("Fan in message received", message)
		aggregateMsgChannel <- ConsolidationMessage{channelID: channelID, message: message}
	}
}

func distributor(messagesForDistribution <-chan ConsolidationMessage, receiveNewEndpoint chan DistributorReg) {
	seenMsgs := map[string]bool{}

	distributionList := []DistributorReg{}
	for {
		select {
		case consolidationMsg := <-messagesForDistribution:
			fmt.Println("Fan out message received", consolidationMsg)

			// To avoid cyclic messaging between datacenters, only send on messages we haven't seen
			_, seen := seenMsgs[consolidationMsg.message.MessageID]
			if !seen {
				seenMsgs[consolidationMsg.message.MessageID] = true
				// Now send this to every endpoint
				for _, endpoint := range distributionList {
					// That isn't itself
					if consolidationMsg.channelID != endpoint.channelID {
						endpoint.messageChannel <- consolidationMsg.message
					}
				}
			}
		case endpoint := <-receiveNewEndpoint:
			fmt.Println("New endpoint received for distribution", endpoint)
			distributionList = append(distributionList, endpoint)
		}
	}
}
