package main

import "fmt"

type ConsolidationMessage struct {
	channelID int
	isServer  bool
	message   MessageFull
}

type DistributorReg struct {
	channelID      int
	isServer       bool
	messageChannel chan MessageFull
}

type Registration struct {
	toBroker   chan MessageFull // Messages going to broker
	fromBroker chan MessageFull // Messages leaving datacenter
}

func messageBroker(channelRegister <-chan Registration) {
	endpointChan := make(chan DistributorReg, 100)
	aggregateMsgChannel := make(chan ConsolidationMessage, 100)
	go distributor(aggregateMsgChannel, endpointChan)
	currentID := 0
	// fmt.Println("Waiting for clients")
	for newClient := range channelRegister {
		isServer := newClient.fromBroker == nil || newClient.toBroker == nil
		if newClient.toBroker != nil {
			go consolidator(newClient.toBroker, aggregateMsgChannel, currentID, isServer)
		}
		if newClient.fromBroker != nil {
			endpointChan <- DistributorReg{channelID: currentID, isServer: isServer, messageChannel: newClient.fromBroker}
		}
		currentID++
	}
}

func consolidator(fromSource <-chan MessageFull, aggregateMsgChannel chan<- ConsolidationMessage, channelID int, isServer bool) {
	defer fmt.Println("Consolidator ended")
	for message := range fromSource {
		// fmt.Println("Fan in message received", message)
		aggregateMsgChannel <- ConsolidationMessage{channelID: channelID, isServer: isServer, message: message}
	}
}

func distributor(messagesForDistribution <-chan ConsolidationMessage, receiveNewEndpoint chan DistributorReg) {

	distributionList := []DistributorReg{}
	for {
		select {
		case consolidationMsg := <-messagesForDistribution:
			// Send this to every endpoint
			for _, endpoint := range distributionList {
				// That isn't a server (if this is a server)
				if !consolidationMsg.isServer || (consolidationMsg.isServer && !endpoint.isServer) {
					// And that isn't itself
					if consolidationMsg.channelID != endpoint.channelID {
						endpoint.messageChannel <- consolidationMsg.message
					}
				}
			}
		case endpoint := <-receiveNewEndpoint:
			// fmt.Println("New endpoint received for distribution", endpoint)
			distributionList = append(distributionList, endpoint)
		}
	}
}
