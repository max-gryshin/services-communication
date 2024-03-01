# services communication

### Description
An attempt to realize [Atomic broadcast](Atomic%20broadcast) - method of transferring message to all replicas of the 
node simultaneously.

Atomic broadcast requires the following properties:
Validity: if a correct node broadcasts a message, then all correct nodes will eventually receive it.
Uniform Agreement: if one correct node receives a message, then all correct nodes will eventually receive that message.
Uniform Integrity: a message is received by each node at most once, and only if it was previously broadcast.
Uniform Total Order: the messages are totally ordered in the mathematical sense; that is, if any correct node receives
message 1 first and message 2 second, then every other correct node must receive message 1 before message 2.

### Node
Correct node is a participant of the system which maintain all properties of atomic broadcast.

### Fault tolerance
By realizing atomic broadcast we should think about fault tolerance - because real computers are faulty, they fail, 
they recover, they could be redeployed or scaled. It means we should maintain correctness of the system during the fault
nodes and consider possibility to add or remove nodes. Furthermore it means if one node became fault and then recovered,
it need to recover whole state (lost history of the messages)   

Example of atomic (generic) broadcast:
https://github.com/jabolina/go-mcast
https://github.com/jabolina/relt
https://github.com/etcd-io/etcd
https://en.wikipedia.org/wiki/State_machine_replication#Auditing_and_Failure_Detection

# Formalization

Since service communication could be described by atomic broadcast distributed computing primitive that include such a
properties as Validity, Uniform Agreement, Uniform Integrity, Uniform Total Order as well atomic broadcast and consensus
are equivalent we should consider it during realization of service communication.

To consider properties of Validity and Uniform Agreement we must to be sure that if correct node has sent a message, 
eventually all nodes will receive a message. It means that before broadcasting node we should know a correct state of 
the system - set of participants are in the system. To consider Uniform Integrity property we need to maintain 
at most once delivery semantic of delivery a message, to be sure we do not duplicate messages. And we need to consider 
Uniform Total Order property to be sure about the correct ordering of the messages, to realizing that we could use
logical clock.

Distributed system is a dynamic system where nodes could be added or removed, they also could be not available because 
of the network systems, they could recover after the failure. It means that service communication should maintain 
fault-tolerant distributed coordination and has mechanism to add/remove/recover a node. Under recover means not only
recover by restarting a server, but also reestablish connection with whole participants and upload all missed 
messages (recover a log). We also need to specify meaning of faulty node - period of the time after which node become faulty.

When new nodes creates, it should start a mechanism of notifying system about himself.
When node removes, it should start a mechanism of notifying system about stopping
Such a mechanism start a broadcast6 with the signaling about starting/stopping node

We need to be sure every participant know about new statement of the system briefly added a new node or removed existing one
Consensus algorithms include rounds of communication and epochs in which desirable state is reached
By maintaining consensus algorithm we have a leader which could provide a meta data to the new node about statement
of the system (amount of the participants, etc.) and send a broadcast message to the participants to notify about new node.
The same with deleting.
