package com.androidresolved.bridge

import resolved.Resolved

object ResolvedBridge {
    private var loaded = false

    val isLoaded: Boolean get() = loaded

    fun startWithTunFd(fd: Long) {
        Resolved.startWithTunFd(fd)
        loaded = true
    }

    fun stop() {
        Resolved.stop()
        loaded = false
    }

    fun getStats(): String {
        return Resolved.getStats()
    }

    fun isRunning(): Boolean {
        return Resolved.isRunning()
    }

    fun resolveHostname(name: String, qtype: Int = 1): String {
        return Resolved.resolveHostname(name, qtype.toLong())
    }

    fun getStatusJSON(): String {
        return Resolved.getStatusJSON()
    }

    fun flushCaches() {
        Resolved.flushCaches()
    }

    fun resetStatistics() {
        Resolved.resetStatistics()
    }

    fun addRule(owner: String, ruleJSON: String) {
        Resolved.addRule(owner, ruleJSON)
    }

    fun flushRules(owner: String) {
        Resolved.flushRules(owner)
    }
}
