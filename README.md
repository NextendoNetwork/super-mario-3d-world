# Super Mario 3D World + Bowser's Fury

An experimental Nextendo server for Super Mario 3D World + Bowser's Fury. It supports three explicit network profiles:

- `same-host`: two isolated Ryujinx-Nextendo instances on one computer.
- `lan`: clients and services reachable through a routed local network.
- `public-internet`: NEX and NNCS behind one-to-one NAT or directly assigned public addresses.

The server was validated with game update 1.2.2, NEX 4.6.4, PRUDP minor 6, and Ryujinx-Nextendo 1.8.2. A two-client same-host test reached character selection and loaded the same level. Internet deployment still requires an acceptance test from two independent networks; seeing a room is not sufficient proof of a working PIA mesh.

## What changed

The server keeps bind addresses separate from advertised addresses. This is required when a private interface is mapped to a public address. NNCS also requires two distinct, client-reachable identities; mixing an overlay address with a SNATed LAN address makes the same UDP socket appear behind different NATs and leads to strict/symmetric classification and zero-RTT failures.

The game-specific matchmaking policy also:

- delays `Participate` notifications by 100 ms;
- reports the number of participants that existed before a join;
- preserves each emulator's registered loopback identity in same-host mode;
- suppresses loopback-only NAT probes while retaining the reachable LAN candidate.

The small NEX additions are opt-in and are proposed in [NextendoNetwork/nextendo-nex#12](https://github.com/NextendoNetwork/nextendo-nex/pull/12). Until that pull request is merged, the module replacement is pinned to the exact reviewed fork commit for reproducible builds.

## Build and test

Go 1.23 or newer is required.

```sh
go test ./...
go build -o sm3dw-server .
go build -o sm3dw-nncs ./cmd/nncs
```

Set the variables shown in `example.env`, then run the NEX server and NNCS responder as separate processes with the same environment. The account service must share `NEXTENDO_SECRET` and `NEXTENDO_INTERNAL_KEY` with this server. Signed `nx2` tokens are required outside same-host mode.

No emulator patch, game file, firmware, key, account, certificate, token, or packet capture is included.

See [DEPLOYMENT.md](DEPLOYMENT.md) for the network contract and production acceptance checklist.
