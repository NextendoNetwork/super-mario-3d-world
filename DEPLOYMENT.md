# Deployment contract

## Address model

`BIND_IP` and both `NNCS*_BIND_IP` values name interfaces on the server. `NEXTENDO_HOST` and both `NNCS*_PUBLIC_IP` values are sent to clients. They may differ behind DNAT.

The `public-internet` profile rejects loopback, RFC 1918, RFC 6598/CGNAT, link-local, benchmark, documentation, multicast, and reserved advertised addresses. This prevents a private or overlay address from being accidentally published as a working Internet endpoint.

NNCS1 and NNCS2 must remain distinct all the way through the network path. A reply emitted from the NNCS2 bind must leave the edge router with the NNCS2 public source address.

| Service | Transport | Public destination | Local bind |
|---|---:|---|---|
| NEX authentication | TCP 443 | `NEXTENDO_HOST` | `BIND_IP` |
| NEX secure | TCP 60003 | `NEXTENDO_HOST` | `BIND_IP` |
| NNCS1 | UDP 10025 and 10125 | `NNCS1_PUBLIC_IP` | `NNCS1_BIND_IP` |
| NNCS2 | UDP 10025 and 10125 | `NNCS2_PUBLIC_IP` | `NNCS2_BIND_IP` |
| NNCS alternate replies | dynamic UDP | matching NNCS public identity | matching NNCS bind |
| PIA sinks | UDP 33334 and 33335 | NNCS1 identity | NNCS1 bind |

The account API should be published behind valid HTTPS. Its `/internal/*` routes must be restricted to the game server at both the reverse proxy and application layers.

## Profiles

### Same host

Use the defaults in `example.env`. Run two emulator instances with separate data roots and separate accounts. This profile deliberately preserves the loopback public station as identity metadata while the LAN station carries the per-client UDP port.

### LAN

Assign two reachable addresses to the NNCS host, set `TRANSPORT_PROFILE=lan`, and set all bind and advertised values explicitly. Set `NNCS_NAT_FILE` to a path shared by the NEX and NNCS processes. Do not use same-host PID compatibility.

### Public Internet

Set `TRANSPORT_PROFILE=public-internet`. Provide two distinct public NNCS addresses, either directly on interfaces or through one-to-one NAT. Port forwarding a single public address to two private addresses does not satisfy the NNCS source-identity requirement.

## Acceptance checklist

A production deployment is accepted only after two clients on independent external networks complete all of the following:

1. Both NNCS identities answer from the expected public source address and port behavior.
2. Host and guest receive their `Participate` events.
3. PIA establishes a non-zero RTT mesh without 2618-0513 or 2618-0502.
4. Both clients reach character selection and load the same level.
5. Gameplay remains stable for at least 15 minutes.
6. Leave, reconnect, and a second room complete without stale participants or cross-room traffic.

Direct P2P can still fail for incompatible symmetric NAT pairs. Relay support must be tested explicitly before claiming support for those pairs. The current repository does not advertise an unverified relay.

The NNCS observation file is keyed by public IP because the protocol carries no account identifier. Multiple players behind the same public NAT can therefore be ambiguous. This is a known limitation and should be resolved with an authenticated client-to-observation correlation mechanism before a broad public rollout.
