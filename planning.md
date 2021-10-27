# Project Notes

## Description
- Clients
    - connected to representative data centers.
    - can read and write data (we're doing text messages)
- Data Center
    - Track dependencies of all of its clients (?)
    - Send replicated updates to other data centers
    - Check received writes (from other data centers) before committing the write
        - Delay until dependency is satisfied
    - __Intentionally delay__ sending writes to other data centers by random amount

## Implementation
 - Data Center
    - State:
        - Client
            - Current "dependency" - what message (or messages if merging) it has last received
            - Socket (ip/port as well)
        - Message/state tree
            - Each message has a single dependency (i.e., a message chain) or multiple (i.e., two indpenedent chains that are not causally linked)
            - Each message has an ID (used for dependencies) made frm client + client clock (really just a hash)
            - Message data
    - Thread listening for socket connections and spin off another thread whenever it gets a connection
        - Connection will identify Type: server/client, and will spin off a listener thread        
    - Connection threads (from other database)
        - Buffer message for random amount of time
        - Receive messages which are stored in the database MAP (should share objects with the trees)
        - Update cache of message trees and disconnected trees
        - Find if the updates will advance the position of any client, hold onto messages that are disconnected for the client
    - Connection threads (from clients)
        - Update database MAP and tree

## Objects
- Database message object:
    <code> NODE:{
        parent_nodes:NODE[],
        ID_hash:string,
        data,
        children: NODE[],
        read/write:mutex
        }
    </code>