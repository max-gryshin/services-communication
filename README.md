# services communication

### Description
An attempt to realize [Atomic broadcast](Atomic%20broadcast) - method of transferring message to all replicas of the node simultaneously.
Atomic broadcast requires the following properties:
Validity: if a correct node broadcasts a message, then all correct nodes will eventually receive it.
Uniform Agreement: if one correct node receives a message, then all correct nodes will eventually receive that message.
Uniform Integrity: a message is received by each node at most once, and only if it was previously broadcast.
Uniform Total Order: the messages are totally ordered in the mathematical sense; that is, if any correct node receives message 1 first and message 2 second, then every other correct node must receive message 1 before message 2.

### Node definition
Node means scope of services responsible for its purposes, for example backend service in usual sense, database, file storage.
It is also means one instance, one replica.

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

# Run
```shell
docker-compose up -d --build
```

# store logs
```shell
docker-compose logs -f > dev.log
```