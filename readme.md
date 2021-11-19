# CSE 513 Distributed Systems - Project 2: Enforcing Causal Consistency in Distributed System

## Code Overview

This project was developed using [Go](https://golang.org/) because its channel paradigm marries excellently with the requirements of the project. It is split up into two segments: `client` and `server`. The `client` code is a basic terminal application for connecting with a session on the `server` while the server maintains state and ensures causal consistency.

The `server` section has a `main` section that spins off three other sections: `clientHanlder`, `datacenterHandler`, and `messageBroker`. `messageBroker` is the communication switchboard for broadcasting messages it receives. It is mainly a "fan in/fan out" pattern, though messages from a datacenter only go to attached clients and messages from a client are not looped-back to the originating client.

The `clientHandler` receives messages from the client, tags them with the client's current Lamport clock, and sends them to the `messageBroker`. It also receives messages from the broker and sends them to staging where they are held until the client satisfies the prerequisites for the message. When a message is ready to be sent to a client, the `clientHandler` strips out tag information (clients are unaware about their state, they are mere terminals), sends the message, and updates the state of the client's Lamport clock. When the state of the client changes, staged messages are reviewed to determine if any were waiting for the current state and are ready to send.

The `datacenterHandler` sends and receives messages from other datacenters. When it sends a message, it adds a random delay to the transmission to simulate variabilities of network connections. This might make it so that one datacenter (and client) receive a message before another.

A significant challenge was using the Go channel paradigm to maintain state. Instead of locks and mutexes, Go has the genius idea of communicating with channels where a single process consumes a message that is sent. If there are multiple worker processes, they can grab jobs from a channel to process, however in this project all channels are consumed by a single process. I chose to have registration channels whereby an entity could register with a producer of a channel so that the producer would include the new consumer in the fan-out.

This concept was especially challenging with the `clientStateManager` function/class within the `clientHandler` as the client state is consumed by multiple entities: `clientStaging` as well as `addDeps`. Again, because data from a channel can only be consumed by one subscribed endpoint, I had to create a way to easily add subscribers: the `csSubscribeFn` which would create a new channel and register it with the `clientStateManager`. For example, here is a section from the relevant portion:

```GO
// This is a background function that does the fanout operation, keeping track of
// every subscriber based on the addSubscriber channel and sending them updates
// to the clientStateChan channel
go func() {
  subscribers := []chan ClientState{}
  for {
    select {
      case newSub := <-addSubscriber:
        subscribers = append(subscribers, newSub)
      case newState := <-clientStateChan:
        for _, subscriber := range subscribers {
          subscriber <- newState
        }
    }
  }
}()

// This is a returned utility function for generating a new subscriber and returning
// the relevant fanout channel
csSubscribeFn := func() chan ClientState {
  localCSChan := make(chan ClientState, cap(clientStateChan))
  addSubscriber <- localCSChan
  return localCSChan
}
```

## How to run

First, you must [install Go](https://golang.org/doc/install). Once installed, if you are on a Windows computer, you can simply navigate to the current folder and execute `run.ps1` which will first call `build.ps1` to build the Go executables and second will start three datacenters and three clients and give them appropriate ports to connect to each other. For a linux machine, you can look at the PowerShell scripts and execute those commands (e.g., `go build -o ../bin/client.o -gcflags='all=-N -l`).

## Demonstration of Operation

Let's say Batman (client `57525`) conducts a meeting and starts roll call. Superman (client `57527`) and Robin (client `57528`) chime in from other clients:

```txt
Hi everyone, welcome to our secret spy meeting. Let's do roll-call
Batman here!
Robin here
superman here!
```

Batman, at client `57525` now has the state: `57525{1} 57527{0} 57528{0}`. That is, his Lamport clock shows that he has seen message `1` (0-indexed) from himself, message `0` from Superman and message `0` from Robin. Note that Superman's roll-call was sent before he received Batman's roll-call (his clock was `57525{0}`) so it was concurrent with it and *not* causally dependent. Thus, it is valid for Robin to receive them in a different order:

```txt
Robin here
Batman here!
superman here!
```

## Same Process a → b

Batman then decides to test the ordering of messages. This tests that a->b if a happens before b on the same thread of execution.

```txt
Let's test that our communication is consistent - that none of my messages get to you out of order
1
2
3
4
5
6
7
8
```

Superman's server then received message `"6"` above and the server indicates:

```txt
Dependency not satisfied!
State:  57525{3}57527{0}57528{0} . Missing: 57525{8}
... for message:
-----------------------
Message ID: 57525{9}
Dependencies:
        57525{8}
        57527{0}
        57528{0}
Body: 6
-----------------------
```

In other words, he hasn't received message id `8` from Batman with the message `"5"` (and a few other ones before that, but all this message cares about is the messages immediately seen before it was sent). The server then held onto the message and queued it until the appropriate messages were received. When a message is sent and the client's state is changed, the system will automatically look to see if another message can be sent, thus making it a *responsive* implementation. It could be improved by generating a graph of queued messages so that when the state changes, we can send the appropriate queued message and waterfall down without trying each message on every state change.

## Separate Processes a → b

Batman and Superman now decide to test simulatneous communication. Some messages are "comtemporaneous" (written with the same clock) while others are causal. Batman sends a,b,c while Superman sends 1,2,3. Batman sees:

```txt
a
b
c
1
2
3
```

While Superman sees:

```txt
a
b
1
2
3
c
```

As you can see with Superman, he sent 1,2,3 after a,b but before c - thus a→b→1→2→3 but 1,2, and 3 are concurrent with c. This means that it is valid for Robin to receive c any place between 1, 2, or 3.
Looking at the server log we see that c looks like this:

```txt
Message ID: 57525{29}
Dependencies:
        57525{28}
        57527{1}
        57528{7}
```

... while 1 looks like the following. Notice they have the same dependencies (i.e., associated clock).

```txt
-----------------------
Message ID: 57528{8}
Dependencies:
        57525{28}
        57527{1}
        57528{7}
Body: 1
-----------------------
```

 This results in the output that Robin sees below:

```txt
a
b
1
2
c ←
3
```

Note that c could come after in any order with 1, 2, and 3 because if 1, 2, or 3's dependencies are satisfied, then c's are as well. Likewise if c is satisfied, 1 will be satisfied while 2 or 3 may be (conditional on 1 and/or 2 also being received). Note, I did not implement a tie-breaker for write conflicts (concurrent messages). If I had a more robust GUI I had considered presenting them side-by-side or otherwise indicating that they were concurrent.

## Possible Extra Credit

I made it so that multiple clients could connect to a single server and that server would appropriately maintain state for the multiple clients. The communication system still works when a latecomer tries to join the conversation.

For example, let's say Alfred comes and joins on Superman's server. Alfred sends this message:

```txt
-----------------------
Message ID: 57589{0}
Dependencies:
Body: Hey, it's alfred!
-----------------------
```

Everybody receives this message as it has no dependencies. However, there's a long conversation going between Batman, Superman, and Robin which is causally linked to any messages they send for which Alfred cannot satisfy the prerequisites. Batman then responds:

```txt
-----------------------
Message ID: 57525{22}
Dependencies:
        57525{21}
        57527{1}
        57528{7}
        57589{0}
Body: Hey there silly goof!
-----------------------
```

And the server blocks the message so that it does not get sent to poor Alfred:

```txt
Dependency not satisfied!
State:  57589{4} . Missing: 57525{21}
... for message:

-----------------------
Message ID: 57525{22}
Dependencies:
        57525{21}
        57527{1}
        57528{7}
        57589{0}
Body: Hey there silly goof!
-----------------------
```

Continuing on the conversation, Batman and Superman keep teasing him and ultimately see the following:

```txt
Hey, it's alfred!
Hey there silly goof!
You can't hear any of our messages because we have a history together and our messages are tagged with lamport clock values that you don't have
So you wouldn't understand what we are talking about
it's an inside joke! :D
uh..
guys?
It's meeeee
cya
```

While Alfred only has a conversation with himself:

```txt
Hey, it's alfred!
uh..
guys?
It's meeeee
cya
```
