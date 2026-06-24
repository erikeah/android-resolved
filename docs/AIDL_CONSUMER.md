# AIDL Consumer Guide

This document explains how 3rd party Android apps integrate with
**android-resolved** to obtain split-DNS resolution via the VPN service.

## Overview

The integration requires three steps:

1. **Authorize** — launch the `AuthActivity` to approve your app and VPN access
2. **Bind** — connect to `DnsVpnControlService` via AIDL
3. **Start** — start the VPN resolver and optionally add custom rules

When your app disconnects, it releases its reference. The VPN stops
automatically when the last connected app disconnects (reference counting).

## Prerequisites

### 1. Copy the AIDL file

Place [`IVpnControlService.aidl`](app/app/src/main/aidl/com/androidresolved/IVpnControlService.aidl)
in your app's `src/main/aidl/com/androidresolved/` directory.

### 2. Add package query (Android 11+)

In your `AndroidManifest.xml`, add a `<queries>` element so your app can
resolve the service:

```xml
<queries>
    <package android:name="com.androidresolved" />
</queries>
```

### 3. Service intent filter

Use the explicit `ComponentName` binding (required for API 34 cross-app
binding), or bind via the intent action:

- **Action**: `com.androidresolved.action.BIND_VPN_CONTROL`
- **Package**: `com.androidresolved`

## Authorization Flow

Before your app can start the VPN, it must be authorized by the user. Launch
the authorization activity using `startActivityForResult` (or the Activity
Result API):

```kotlin
val intent = Intent("com.androidresolved.action.AUTHORIZE_VPN").apply {
    `package` = "com.androidresolved"
}
launchAuth.launch(intent)
```

The `AuthActivity` handles:

1. **First-run welcome** — shown once system-wide
2. **Per-app approval** — the user is prompted to allow **your app** to use
   the resolver (stored by UID)
3. **VPN authorization** — the system VPN permission dialog

If the result code is `RESULT_OK`, authorization is complete and you may
bind to the control service.

## Binding to the Service

```kotlin
private val connection = object : ServiceConnection {
    override fun onServiceConnected(name: ComponentName, binder: IBinder) {
        service = IVpnControlService.Stub.asInterface(binder)
        // service is now ready
    }

    override fun onServiceDisconnected(name: ComponentName) {
        service = null
    }
}

fun bindToService() {
    val intent = Intent().apply {
        component = ComponentName(
            "com.androidresolved",
            "com.androidresolved.DnsVpnControlService"
        )
    }
    bindService(intent, connection, Context.BIND_AUTO_CREATE)
}
```

## API Reference

### `String start()`

Starts the VPN with sensible defaults:

- `.local` → mDNS
- `.` → `1.1.1.1:53` (UDP)

Returns an empty string on success, or an error message on failure.

**Possible errors:**

| Return value | Meaning |
|---|---|
| `""` (empty) | Success |
| `"NOT_APPROVED"` | Your app's UID has not been authorized via `AuthActivity` |
| `"VPN_NOT_AUTHORIZED"` | System VPN permission not granted |

**Reference counting:** Each call to `start()` increments your app's
reference. The VPN remains running as long as at least one app holds a
reference.

### `void stop()`

Decrements your app's reference. When the last connected app calls
`stop()` (or disconnects via `onUnbind`), the VPN is torn down.

### `void addRule(String ruleJson)`

Adds a DNS resolution rule scoped to your app's UID. The rule JSON format:

```json
{"domain": ".example.com", "upstream": "10.0.1.1", "protocol": "udp"}
```

| Field | Required | Description |
|---|---|---|
| `domain` | Yes | Domain suffix (e.g. `.example.com`) or catch-all `.` |
| `upstream` | Conditional | Upstream IP address (port 53 assumed by default). Required for `udp`/`tcp`, leave empty or omit for `mdns` |
| `protocol` | Yes | One of `"udp"`, `"tcp"`, `"mdns"` |

**First-wins semantics:** If another app has already registered a rule for
the same domain, your rule is silently skipped. When that app flushes its
rules, yours takes effect automatically.

Rules persist for the lifetime of the engine (until `Stop()` or app crash).

### `void flushRules()`

Removes all rules previously added by your app's UID. If another app had
a rule for the same domain that was previously skipped due to first-wins,
it now takes effect.

### `boolean isRunning()`

Returns `true` if the VPN engine is currently running.

### `String getStatus()`

Returns a JSON object:

```json
{
  "running": true,
  "total_queries": 142,
  "cache_hits": 89,
  "cache_misses": 53,
  "rules": 4
}
```

### `String getStats()`

Returns a compact JSON object with the same fields as `getStatus()`.

### `String getVersion()`

Returns the version string (e.g. `"0.1.0"`).

## Complete Integration Example

```kotlin
// build.gradle.kts dependencies
// implementation("androidx.activity:activity-ktx:1.9.+")

class MyDnsClient : AppCompatActivity() {
    private var service: IVpnControlService? = null
    private var bound = false

    private val authLauncher = registerForActivityResult(
        ActivityResultContracts.StartActivityForResult()
    ) { result ->
        if (result.resultCode == RESULT_OK) {
            bindToService()
        }
    }

    private val connection = object : ServiceConnection {
        override fun onServiceConnected(name: ComponentName, binder: IBinder) {
            service = IVpnControlService.Stub.asInterface(binder)
            bound = true
            startVpn()
        }

        override fun onServiceDisconnected(name: ComponentName) {
            service = null
            bound = false
        }
    }

    fun authorizeAndBind() {
        val intent = Intent("com.androidresolved.action.AUTHORIZE_VPN").apply {
            `package` = "com.androidresolved"
        }
        authLauncher.launch(intent)
    }

    private fun bindToService() {
        val intent = Intent().apply {
            component = ComponentName(
                "com.androidresolved",
                "com.androidresolved.DnsVpnControlService"
            )
        }
        bindService(intent, connection, Context.BIND_AUTO_CREATE)
    }

    private fun startVpn() {
        val result = service?.start()
        if (result.isNullOrEmpty()) {
            // Success — add custom rules
            service?.addRule("""{"domain":".corp.example","upstream":"10.0.1.1","protocol":"udp"}""")
        } else {
            // Handle error: "NOT_APPROVED" or "VPN_NOT_AUTHORIZED"
        }
    }

    override fun onDestroy() {
        if (bound) {
            service?.stop()   // release reference
            unbindService(connection)
        }
        super.onDestroy()
    }
}
```

## Lifecycle Summary

```
Authorize → Bind → start() → addRule() ... stop() → unbind → [VPN stops if last]
```

- Authorization is persistent (stored by UID in SharedPreferences) — done once
- Binding establishes a service connection (your AIDL proxy)
- `start()` registers your reference and starts the VPN (if not already running)
- `addRule()` / `flushRules()` manage your app's DNS rules
- `stop()` releases your reference
- `onUnbind` in the service automatically releases references if your
  app crashes without calling `stop()`
- The VPN stops when the last reference is released

## Important Limitation

### Single VPN

Android allows at most one VPN to be active at any time. If the user has
another VPN running (WireGuard, Tailscale, AdGuard, etc.), `start()` will
return `"VPN_NOT_AUTHORIZED"`. The other VPN must be disconnected first.

This is an Android platform constraint, not a bug. All VPN-based DNS
resolvers share this limitation.

## Manifest Requirements Summary

```xml
<queries>
    <package android:name="com.androidresolved" />
</queries>
```

No other permissions are required by the consuming app. The service app
holds `INTERNET`, `FOREGROUND_SERVICE`, and `BIND_VPN_SERVICE` permissions.
