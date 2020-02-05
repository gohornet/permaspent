# Permaspent

A server application collecting spent addresses from nodes via MQTT and exposing an HTTP API to query them.
It substitutes the fact that every node has to keep its own copy of spent addresses, by providing a service
doing the exact same. This application works mainly in conjunction with Hornet.

If you're running a productive system of IOTA nodes, it is recommended to also host a Permaspent node yourself.

Features:
* Define multiple MQTT streams from which to collect spent addresses
* Online exports of the spent addresses database via an HTTP call
* Authentication via an API key for certain HTTP calls
* Limit for the amount of addresses to be queried via an `wereAddressesSpentFrom` call
* Import a `spent_addresses.bin` generated via [iri-ls-sa-merger](https://github.com/iotaledger/iri-ls-sa-merger)

It is recommended to run the application behind a reverse proxy which can be accessed via TLS from the Internet.