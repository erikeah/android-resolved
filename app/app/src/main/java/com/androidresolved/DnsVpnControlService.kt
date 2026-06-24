package com.androidresolved

import android.app.Service
import android.content.Context
import android.content.Intent
import android.net.VpnService
import android.os.Binder
import android.os.IBinder
import android.util.Log
import com.androidresolved.bridge.ResolvedBridge

class DnsVpnControlService : Service() {
    private val activeClients = java.util.Collections.synchronizedSet(mutableSetOf<Int>())

    private val binder = object : IVpnControlService.Stub() {
        override fun start(): String {
            val uid = Binder.getCallingUid()
            Log.i(TAG, "start() called by uid=$uid")
            if (!isUidApproved(this@DnsVpnControlService, uid)) {
                Log.w(TAG, "uid=$uid not approved")
                return "NOT_APPROVED"
            }
            activeClients += uid
            Log.i(TAG, "uid=$uid registered, active=$activeClients")
            if (ResolvedBridge.isRunning()) {
                Log.i(TAG, "already running")
                return ""
            }
            Log.i(TAG, "checking VPN authorization...")
            if (VpnService.prepare(this@DnsVpnControlService) != null) {
                Log.w(TAG, "VPN not authorized")
                return "VPN_NOT_AUTHORIZED"
            }
            Log.i(TAG, "VPN authorized, starting DnsVpnService...")
            startForegroundService(Intent(this@DnsVpnControlService, DnsVpnService::class.java))
            Log.i(TAG, "DnsVpnService start requested")
            return ""
        }

        override fun addRule(ruleJson: String) {
            val uid = Binder.getCallingUid().toString()
            Log.i(TAG, "addRule() called by uid=$uid")
            try {
                ResolvedBridge.addRule(uid, ruleJson)
                Log.i(TAG, "rule added for uid=$uid")
            } catch (e: Exception) {
                Log.e(TAG, "addRule failed: $e")
            }
        }

        override fun flushRules() {
            val uid = Binder.getCallingUid().toString()
            Log.i(TAG, "flushRules() called by uid=$uid")
            try {
                ResolvedBridge.flushRules(uid)
                Log.i(TAG, "rules flushed for uid=$uid")
            } catch (e: Exception) {
                Log.e(TAG, "flushRules failed: $e")
            }
        }

        override fun stop() {
            val uid = Binder.getCallingUid()
            Log.i(TAG, "stop() called by uid=$uid")
            activeClients -= uid
            Log.i(TAG, "active after stop: $activeClients")
            if (activeClients.isEmpty()) {
                Log.i(TAG, "last client, signaling DnsVpnService to stop")
                startService(Intent(this@DnsVpnControlService, DnsVpnService::class.java).apply {
                    action = ACTION_STOP_VPN
                })
            }
        }

        override fun isRunning() = ResolvedBridge.isRunning()
        override fun getStatus() = ResolvedBridge.getStatusJSON()
        override fun getStats() = ResolvedBridge.getStats()
        override fun getVersion() = "0.1.0"
    }

    override fun onCreate() {
        super.onCreate()
        Log.i(TAG, "DnsVpnControlService created")
    }

    override fun onBind(intent: Intent): IBinder {
        Log.i(TAG, "onBind called, intent=$intent, returning binder")
        return binder
    }

    override fun onUnbind(intent: Intent): Boolean {
        Log.i(TAG, "onUnbind: active=$activeClients, clearing and stopping VPN")
        activeClients.clear()
        startService(Intent(this, DnsVpnService::class.java).apply {
            action = ACTION_STOP_VPN
        })
        return false
    }

    companion object {
        const val TAG = "android-resolved-ctrl"
        const val ACTION_STOP_VPN = "com.androidresolved.action.STOP_VPN"
        private const val PREFS_AUTH = "android-resolved-auth"
        private const val PREFS_APPROVED = "approved_uids"

        fun isUidApproved(context: Context, uid: Int): Boolean {
            val prefs = context.getSharedPreferences(PREFS_AUTH, Context.MODE_PRIVATE)
            return uid.toString() in (prefs.getStringSet(PREFS_APPROVED, emptySet()) ?: emptySet())
        }

        fun approveUid(context: Context, uid: Int) {
            val prefs = context.getSharedPreferences(PREFS_AUTH, Context.MODE_PRIVATE)
            val set = (prefs.getStringSet(PREFS_APPROVED, mutableSetOf()) ?: mutableSetOf()).toMutableSet()
            set += uid.toString()
            prefs.edit().putStringSet(PREFS_APPROVED, set).apply()
        }
    }
}
