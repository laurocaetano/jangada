[![Go](https://github.com/laurocaetano/jangada/actions/workflows/go.yml/badge.svg?branch=main)](https://github.com/laurocaetano/jangada/actions/workflows/go.yml)

# Jangada

This project is a learning project!

It is a work in progress to build a Go implementation of [Raft](https://raft.github.io/raft.pdf). This implementation will
be used as a library inside [Foxtail](https://github.com/laurocaetano/foxtail), a KV database
created for educational purposes.

## Steps

This project is being developed with simplicity in mind. It will bundle all logic in a single file, without being too clever
to find abstractions upfront. Once the functional parts are working, abstractions will naturally emerge as the code becomes
stable.

**TODO**

- [x] Basic leader election
- [x] Basic append entries RPC hander
- [ ] Implement timer logic
- [ ] Implement concrete RPC calls
- [ ] Implement the server loop
- [ ] Add option to persist log to disk
- [ ] Snapshot
- [ ] Implement cluster configuration changes (add/remove nodes)
