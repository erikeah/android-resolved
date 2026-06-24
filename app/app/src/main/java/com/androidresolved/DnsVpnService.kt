package com.androidresolved

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Intent
import android.net.VpnService
import android.os.ParcelFileDescriptor
import android.provider.Settings
import android.util.Log
import com.androidresolved.bridge.ResolvedBridge

class DnsVpnService : VpnService() {
    private var tunFd: ParcelFileDescriptor? = null

    override fun onCreate() {
        super.onCreate()
        Log.i(TAG, "onCreate")
        createChannel()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        if (intent?.action == DnsVpnControlService.ACTION_STOP_VPN) {
            Log.i(TAG, "received stop signal")
            teardown()
            stopSelf()
            return START_NOT_STICKY
        }
        Log.i(TAG, "onStartCommand: flags=$flags startId=$startId intent=$intent")
        if (tunFd == null) {
            Log.i(TAG, "starting VPN")
            startVpn()
        } else {
            Log.i(TAG, "TUN already established")
        }
        if (tunFd != null) {
            Log.i(TAG, "promoting to foreground service")
            startForeground(NOTIFICATION_ID, buildNotification())
        } else {
            Log.w(TAG, "TUN not established, stopping self")
            stopSelf()
        }
        return START_NOT_STICKY
    }

    override fun onRevoke() {
        Log.w(TAG, "VPN revoked by system")
        teardown()
        stopSelf()
    }

    override fun onDestroy() {
        Log.i(TAG, "onDestroy")
        teardown()
        super.onDestroy()
    }

    private fun startVpn() {
        val builder = Builder()
            .setSession("android-resolved")
            .setMtu(1500)
            .addAddress(VPN_IP, 24)
            .addRoute(VPN_NET, 24)
            .addDnsServer(DNS_IP)
        Log.i(TAG, "establishing TUN interface (${VPN_IP}/24, DNS=$DNS_IP)")
        val pfd = builder.establish()
        if (pfd == null) {
            Log.e(TAG, "builder.establish() returned null!")
            return
        }
        Log.i(TAG, "TUN established, fd=${pfd.fd}, starting Go engine")
        ResolvedBridge.startWithTunFd(pfd.detachFd().toLong())
        tunFd = pfd
        Log.i(TAG, "Go engine started")
    }

    private fun teardown() {
        if (tunFd == null) {
            Log.i(TAG, "teardown: no TUN fd, skipping")
            return
        }
        Log.i(TAG, "teardown: closing TUN and stopping Go engine")
        tunFd = null
        ResolvedBridge.stop()
        stopForeground(STOP_FOREGROUND_REMOVE)
        Log.i(TAG, "teardown complete")
    }

    private fun buildNotification(): Notification {
        val intent = Intent(Settings.ACTION_VPN_SETTINGS).apply {
            addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
        }
        val pi = PendingIntent.getActivity(this, 0, intent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE)
        return Notification.Builder(this, CHANNEL_ID)
            .setContentTitle("android-resolved")
            .setContentText("Split-DNS resolver running")
            .setSmallIcon(android.R.drawable.ic_menu_search)
            .setContentIntent(pi)
            .setOngoing(true)
            .build()
    }

    private fun createChannel() {
        val nm = getSystemService(NotificationManager::class.java)
        nm.createNotificationChannel(
            NotificationChannel(CHANNEL_ID, "android-resolved", NotificationManager.IMPORTANCE_LOW)
        )
    }

    companion object {
        const val TAG = "android-resolved-vpn"
        private const val VPN_IP = "10.0.0.1"
        private const val VPN_NET = "10.0.0.0"
        private const val DNS_IP = "10.0.0.2"
        private const val CHANNEL_ID = "android-resolved-vpn"
        private const val NOTIFICATION_ID = 1001
    }
}
