package main

type ConsolidationMessage struct {
	channelID int
	message   Message
}

type DistributorReg struct {
	channelID      int
	messageChannel chan Message
}

type Registration struct {
	toBroker   chan Message // Messages going to broker
	fromBroker chan Message // Messages leaving datacenter
}

func messageBroker(channelRegister <-chan Registration) {
	var endpointChan chan DistributorReg
	var aggregateMsgChannel chan ConsolidationMessage
	distributor(aggregateMsgChannel, endpointChan)
	currentID := 0
	for newChan := range channelRegister {
		if newChan.toBroker != nil {
			go consolidator(newChan.toBroker, aggregateMsgChannel, currentID)
		}
		if newChan.fromBroker != nil {
			endpointChan <- DistributorReg{channelID: currentID, messageChannel: newChan.fromBroker}
		}
		currentID++
	}
}

func consolidator(fromSource <-chan Message, aggregateMsgChannel chan<- ConsolidationMessage, channelID int) {
	for message := range fromSource {
		aggregateMsgChannel <- ConsolidationMessage{channelID: channelID, message: message}
	}
}

func distributor(messagesForDistribution <-chan ConsolidationMessage, receiveNewEndpoint chan DistributorReg) {
	seenMsgs := map[string]bool{}

	distributionList := []DistributorReg{}
	select {
	case consolidationMsg := <-messagesForDistribution:
		// To avoid cyclic messaging between datacenters, only send on messages we haven't seen
		_, seen := seenMsgs[consolidationMsg.message.MessageID]
		if !seen {
			seenMsgs[consolidationMsg.message.MessageID] = true
			// Now send this to every endpoint
			for _, endpoint := range distributionList {
				if consolidationMsg.channelID != endpoint.channelID {
					endpoint.messageChannel <- consolidationMsg.message
				}
			}
		}
	case endpoint := <-receiveNewEndpoint:
		distributionList = append(distributionList, endpoint)
	}
}
