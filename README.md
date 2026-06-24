# android-resolved

A `systemd-resolved` clone for Android with split DNS. Intercepts DNS queries
via a VPN TUN interface and routes them through configurable upstreams using
domain-suffix rules. Exposes an AIDL service for 3rd party apps to configure
resolution on-demand with per-app authorization and reference counting.

## Architecture

```
  Client App (or test client)
         |
  binds to DnsVpnControlService (AIDL)
         |
  +------v-------------------------------------+
  |  DnsVpnControlService (exported Service)    |
  |  - per-app UID approval via SharedPreferences|
  |  - reference counting (active clients set)   |
  |  - AddRule/FlushRules per calling UID        |
  |  - starts/stops DnsVpnService internally     |
  +------+-------------------------------------+
         |
  +------v-------------------------------------+
  |  DnsVpnService (VpnService, not exported)   |
  |  - creates TUN fd (10.0.0.1/24, DNS 10.0.0.2)|
  |  - passes fd to Go engine                    |
  |  - handles stop signal via ACTION_STOP_VPN   |
  +------+-------------------------------------+
         |
  +------v-------------------------------------+
  |  ResolvedBridge.kt (gomobile JNI)           |
  +------+-------------------------------------+
         |
  +------v-------------------------------------+
  |  Go Engine (mobile/resolved.go)             |
  |                                             |
  |  +------------+  +--------+  +----------+  |
  |  | TUN handler |  | Router |  | Resolver |  |
  |  | (tun.go)    |  | (trie) |  | s        |  |
  |  +------------+  +--------+  +----------+  |
  |        |              |            |         |
  |        v              v            v         |
  |  +------------+  +--------+  +----------+  |
  |  | Cache (LRU) |  | mDNS   |  | Stats    |  |
  |  +------------+  +--------+  +----------+  |
  +---------------------------------------------+
```

**Data flow:**

1. Android system sends DNS query to `10.0.0.2:53` (set via `addDnsServer`)
2. TUN interface captures the IP packet (route `10.0.0.0/24` → tun0)
3. `tun.go` parses IPv4/UDP headers, extracts DNS question
4. Router performs domain-suffix longest-match against rules
5. Upstream resolver executes the query (UDP, TCP, or mDNS)
6. Response is cached, packed into an IP packet, and written back to TUN

## Components

### Go Engine (`internal/`)

| Package | Purpose |
|---|---|
| `server/tun.go` | TUN handler — reads raw IP packets, extracts DNS, builds IP responses |
| `router/trie.go` | Domain-suffix trie for split-DNS routing |
| `cache/cache.go` | TTL-aware LRU cache |
| `resolver/direct.go` | Plain UDP/TCP upstream (port 53 default) |
| `resolver/mdns.go` | mDNS resolver (`.local`) |
| `resolver/build.go` | Shared resolver factory |
| `stats/tracker.go` | Query statistics tracker |
| `config/config.go` | JSON config + validation |
| `dnsmsg/message.go` | DNS wire format helpers |

### Mobile Bridge (`mobile/resolved.go`)

gomobile-exported API callable from Kotlin via the AAR:

- `StartWithTunFd(fd)` — start engine with TUN fd, applies sensible defaults
- `AddRule(owner, ruleJSON)` — add a single rule for a given owner (first-wins per domain)
- `FlushRules(owner)` — remove all rules for the given owner
- `Stop()` — stop engine and TUN handler
- `GetStats()` — JSON with query counts, cache stats
- `GetStatusJSON()` — JSON with running state, rules, stats
- `IsRunning()` — boolean
- `ResolveHostname(name, qtype)` — structured DNS query
- `FlushCaches()` — clear DNS cache
- `ResetStatistics()` — zero counters

### Android App (`app/app/`)

| File | Purpose |
|---|---|
| `DnsVpnControlService.kt` | Exported Service with AIDL, per-app UID approval, reference counting |
| `DnsVpnService.kt` | VpnService subclass — creates TUN, passes fd to Go, not exported |
| `AuthActivity.kt` | Transparent activity — first-run welcome, per-app approval dialog, VPN auth |
| `bridge/ResolvedBridge.kt` | JNI bridge to Go engine |

### Test Client (`app/client/`)

| File | Purpose |
|---|---|
| `MainActivity.kt` | Test UI — authorize, start/stop, AddRule form, flush |
| `activity_main.xml` | Layout with domain/upstream/protocol form fields |

## AIDL API (`IVpnControlService`)

```aidl
interface IVpnControlService {
    String start();
    void stop();
    boolean isRunning();
    String getStatus();
    String getStats();
    String getVersion();
    void addRule(String ruleJson);
    void flushRules();
}
```

Apps bind to `com.androidresolved.DnsVpnControlService` (or via intent filter
`com.androidresolved.action.BIND_VPN_CONTROL`). Before calling `start()`, the
app must first be authorized via `AuthActivity` with action
`com.androidresolved.action.AUTHORIZE_VPN`.

| Method | Purpose |
|---|---|
| `start()` | Start VPN with default config (requires prior authorization) |
| `stop()` | Release reference; last client stops the VPN |
| `addRule(ruleJson)` | Add a rule scoped to the calling UID; first-wins per domain |
| `flushRules()` | Remove all rules scoped to the calling UID |

Rule JSON format for `addRule`:
```json
{"domain": ".example.com", "upstream": "10.0.1.1", "protocol": "udp"}
```

The `upstream` field accepts an IP address only; port 53 is assumed by default.

## Rule Priority

Rules are evaluated in this order (first match wins):

1. **Default rules** — always present (`.local` → mDNS, `.` → `1.1.1.1:53`)
2. **Owner rules** — inserted in order of first `AddRule` call per owner

If two owners add a rule for the same domain, the first owner's rule takes
precedence. When the first owner calls `flushRules()`, the second owner's
previously-skipped rule takes effect automatically.

## Configuration

Default config used at startup (no JSON required):

```json
{
  "rules": [
    {"domain": ".local", "protocol": "mdns"},
    {"domain": ".", "upstream": "1.1.1.1", "protocol": "udp"}
  ],
  "cache": { "size": 4096, "min_ttl": 60, "max_ttl": 3600 }
}
```

Additional rules can be added at runtime via `addRule()`.

## Build

### Prerequisites

[Nix](https://nixos.org/download) with flakes enabled.

### Build APK

```bash
nix run .#build-apk
```

Output: `app/app/build/outputs/apk/debug/app-debug.apk`
Client APK: `app/client/build/outputs/apk/debug/client-debug.apk`

## Limitations

### Single VPN

Android allows at most one VPN to be running at any time. If another VPN
(WireGuard, Tailscale, AdGuard, etc.) is active, `start()` will fail with
`VPN_NOT_AUTHORIZED`. Stop the other VPN first, or stop android-resolved
before connecting another VPN.

This is not a bug — it is a fundamental Android platform constraint that
applies to all VPN-based DNS resolvers.

## Install & Test

```bash
# Install both APKs
adb install -r app/app/build/outputs/apk/debug/app-debug.apk
adb install -r app/client/build/outputs/apk/debug/client-debug.apk

# Launch test client
adb shell am start -n com.androidresolved.client/.MainActivity
```

### Test flow

1. Open the test client, tap **Authorize**
2. Accept the VPN authorization dialog
3. Tap **Start** — VPN connects with default rules
4. Add a custom rule via the domain/upstream/protocol form
5. Tap **Flush My Rules** to remove rules for this client
6. Tap **Stop** to release the VPN

### Verify DNS

```bash
adb shell ping -c 1 google.com
```

Check logs:
```bash
adb logcat -s android-resolved-ctrl android-resolved-vpn GoLog
```

## Defaults

| Setting | Default |
|---|---|
| VPN address | 10.0.0.1/24 |
| DNS server | 10.0.0.2 |
| Cache size | 4096 entries |
| Cache TTL | 60s - 3600s |
| Default rule | `.` → `1.1.1.1:53` (UDP) |
| mDNS rule | `.local` → mDNS |
| Upstream port | 53 (assumed if not specified) |
