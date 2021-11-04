package main

import "fmt"

// These are the messages that are placed on the aggregate message
// channel, they include some extra stuff for bookkeeping purposes
type ConsolidationMessage struct {
	channelID    int
	isDataCenter bool
	message      MessageFull
}

type DistributorReg struct {
	channelID      int
	isDatacenter   bool
	messageChannel chan MessageFull
}

type Registration struct {
	toBroker   chan MessageFull
	fromBroker chan MessageFull
}

// This sends/receives messages to other components that are registered with the broker
// through the channelRegister channel
func messageBroker(channelRegister <-chan Registration) {
	// This is a helper channel to translate registration requests to add some contextual detail
	// for tracking (assign an ID to the channel and determine if it is a datacenter)
	endpointChan := make(chan DistributorReg, 100)
	// All messages go through the aggregateMsgChannel from fanin to fanout
	aggregateMsgChannel := make(chan ConsolidationMessage, 100)
	// Fanout
	go distributor(aggregateMsgChannel, endpointChan)

	// currentID is used to ensure we don't loopback during fanout - we only send to other endpoints
	currentID := 0
	for newClient := range channelRegister {
		isServer := newClient.fromBroker == nil || newClient.toBroker == nil
		if newClient.toBroker != nil {
			// Ingest route, give it its own go routine
			go consolidator(newClient.toBroker, aggregateMsgChannel, currentID, isServer)
		}
		if newClient.fromBroker != nil {
			// Distribution route, just register it with the endpointChan (picked up by the distributor
			// go routine)
			endpointChan <- DistributorReg{channelID: currentID, isDatacenter: isServer, messageChannel: newClient.fromBroker}
		}
		currentID++
	}
}

// Each message source will have a respective consolidator go function running
func consolidator(fromSource <-chan MessageFull, aggregateMsgChannel chan<- ConsolidationMessage, channelID int, isServer bool) {
	defer fmt.Println("Consolidator ended")
	for message := range fromSource {
		// Place messages on the aggregateMsgChannel
		aggregateMsgChannel <- ConsolidationMessage{channelID: channelID, isDataCenter: isServer, message: message}
	}
}

func distributor(messagesForDistribution <-chan ConsolidationMessage, receiveNewEndpoint chan DistributorReg) {

	distributionList := []DistributorReg{}
	for {
		select {
		case consolidationMsg := <-messagesForDistribution:
			// Send this to every endpoint
			for _, endpoint := range distributionList {
				// Datacenters only send and receive messages with clients (no DC<->DC communication)
				if !consolidationMsg.isDataCenter || (consolidationMsg.isDataCenter && !endpoint.isDatacenter) {
					// Make sure we don't loopback and send messages back to the client that sent them
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
