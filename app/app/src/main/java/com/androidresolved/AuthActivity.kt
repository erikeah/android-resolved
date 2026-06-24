package com.androidresolved

import android.app.Activity
import android.app.AlertDialog
import android.content.Intent
import android.content.pm.PackageManager
import android.net.VpnService
import android.os.Bundle
import android.util.Log

class AuthActivity : Activity() {
    companion object {
        const val ACTION_AUTHORIZE = "com.androidresolved.action.AUTHORIZE_VPN"
        const val TAG = "android-resolved-auth"
        private const val REQUEST_VPN = 1001
        private const val PREFS_AUTH = "android-resolved-auth"
        private const val PREFS_FIRST_RUN = "first_run_done"
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        val callerPkg = getCallingPackage()
        val refUri = referrer
        Log.i(TAG, "onCreate: callingPackage=$callerPkg, referrer=$refUri")

        val prefs = getSharedPreferences(PREFS_AUTH, MODE_PRIVATE)
        if (!prefs.getBoolean(PREFS_FIRST_RUN, false)) {
            Log.i(TAG, "first run")
            showFirstRun()
        } else {
            checkCaller()
        }
    }

    private fun resolveCallingUid(): Int {
        getCallingPackage()?.let { pkg ->
            return try { packageManager.getPackageUid(pkg, 0) }
            catch (_: PackageManager.NameNotFoundException) { -1 }
        }
        referrer?.let { uri ->
            if (uri.scheme == "android-app") {
                val pkg = uri.host ?: return@let
                return try { packageManager.getPackageUid(pkg, 0) }
                catch (_: PackageManager.NameNotFoundException) { -1 }
            }
        }
        return -1
    }

    private fun showFirstRun() {
        AlertDialog.Builder(this)
            .setTitle("android-resolved")
            .setMessage("Provides split-DNS resolution to authorized applications.")
            .setCancelable(false)
            .setPositiveButton("Continue") { _, _ ->
                Log.i(TAG, "first run done")
                getSharedPreferences(PREFS_AUTH, MODE_PRIVATE)
                    .edit().putBoolean(PREFS_FIRST_RUN, true).apply()
                checkCaller()
            }
            .show()
    }

    private fun checkCaller() {
        val uid = resolveCallingUid()
        Log.i(TAG, "checkCaller: resolved uid=$uid")
        if (uid < 0) {
            Log.i(TAG, "no caller uid, finishing OK")
            finishOk()
            return
        }
        if (!DnsVpnControlService.isUidApproved(this, uid)) {
            val pm = packageManager
            val pkgs = pm.getPackagesForUid(uid)
            val name = if (!pkgs.isNullOrEmpty()) {
                pm.getApplicationLabel(pm.getApplicationInfo(pkgs[0], 0))
            } else "Unknown ($uid)"
            AlertDialog.Builder(this)
                .setTitle("Authorize Application")
                .setMessage("Allow \"$name\" to use android-resolved?")
                .setCancelable(false)
                .setPositiveButton("Allow") { _, _ ->
                    Log.i(TAG, "approving uid=$uid ($name)")
                    DnsVpnControlService.approveUid(this, uid)
                    checkVpnAuth()
                }
                .setNegativeButton("Deny") { _, _ ->
                    Log.i(TAG, "denied uid=$uid")
                    setResult(RESULT_CANCELED)
                    finish()
                }
                .show()
        } else {
            Log.i(TAG, "uid=$uid already approved")
            checkVpnAuth()
        }
    }

    private fun checkVpnAuth() {
        val intent = VpnService.prepare(this)
        Log.i(TAG, "VpnService.prepare() returned ${if (intent == null) "null (authorized)" else "intent (needs auth)"}")
        if (intent != null) {
            startActivityForResult(intent, REQUEST_VPN)
        } else {
            finishOk()
        }
    }

    override fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?) {
        if (requestCode == REQUEST_VPN) {
            Log.i(TAG, "VPN auth result: $resultCode")
            if (resultCode == RESULT_OK) finishOk()
            else { setResult(RESULT_CANCELED); finish() }
        }
    }

    private fun finishOk() {
        Log.i(TAG, "finishing with RESULT_OK")
        setResult(RESULT_OK)
        finish()
    }
}
