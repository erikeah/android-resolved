package com.androidresolved.client

import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.content.ServiceConnection
import android.os.Bundle
import android.os.IBinder
import android.widget.ArrayAdapter
import android.widget.Button
import android.widget.EditText
import android.widget.Spinner
import android.widget.TextView
import androidx.appcompat.app.AppCompatActivity
import com.androidresolved.IVpnControlService
import org.json.JSONObject

class MainActivity : AppCompatActivity() {
    private var service: IVpnControlService? = null
    private var bound = false

    private lateinit var statusText: TextView
    private lateinit var logText: TextView
    private lateinit var authBtn: Button
    private lateinit var startBtn: Button
    private lateinit var stopBtn: Button
    private lateinit var domainInput: EditText
    private lateinit var upstreamInput: EditText
    private lateinit var protocolSpinner: Spinner
    private lateinit var addRuleBtn: Button
    private lateinit var flushRulesBtn: Button

    private val authLauncher = registerForActivityResult(
        androidx.activity.result.contract.ActivityResultContracts.StartActivityForResult()
    ) { result ->
        if (result.resultCode == RESULT_OK) {
            log("VPN authorized, binding...")
            bindToService()
        } else {
            log("Authorization denied")
        }
    }

    private val connection = object : ServiceConnection {
        override fun onServiceConnected(name: ComponentName, binder: IBinder) {
            service = IVpnControlService.Stub.asInterface(binder)
            bound = true
            log("Bound to android-resolved")
            runOnUiThread {
                statusText.text = "Status: Connected"
                updateButtons()
            }
        }

        override fun onServiceDisconnected(name: ComponentName) {
            service = null
            bound = false
            log("Service disconnected")
            runOnUiThread {
                statusText.text = "Status: Disconnected"
                updateButtons()
            }
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_main)

        statusText = findViewById(R.id.statusText)
        logText = findViewById(R.id.logText)
        authBtn = findViewById(R.id.authBtn)
        startBtn = findViewById(R.id.startBtn)
        stopBtn = findViewById(R.id.stopBtn)
        domainInput = findViewById(R.id.domainInput)
        upstreamInput = findViewById(R.id.upstreamInput)
        protocolSpinner = findViewById(R.id.protocolSpinner)
        addRuleBtn = findViewById(R.id.addRuleBtn)
        flushRulesBtn = findViewById(R.id.flushRulesBtn)

        ArrayAdapter.createFromResource(
            this, R.array.protocols, android.R.layout.simple_spinner_item
        ).also { adapter ->
            adapter.setDropDownViewResource(android.R.layout.simple_spinner_dropdown_item)
            protocolSpinner.adapter = adapter
        }

        updateButtons()

        authBtn.setOnClickListener {
            val intent = Intent("com.androidresolved.action.AUTHORIZE_VPN").apply {
                `package` = "com.androidresolved"
            }
            log("Launching authorization...")
            authLauncher.launch(intent)
        }

        startBtn.setOnClickListener {
            try {
                val result = service?.start() ?: "Service not bound"
                if (result.isEmpty()) {
                    log("VPN started")
                } else {
                    log("Start failed: $result")
                }
            } catch (e: Exception) {
                log("Error: ${e.message}")
            }
            updateButtons()
        }

        stopBtn.setOnClickListener {
            try {
                service?.stop()
                log("VPN stop requested")
            } catch (e: Exception) {
                log("Error: ${e.message}")
            }
            updateButtons()
        }

        addRuleBtn.setOnClickListener {
            val domain = domainInput.text.toString().trim()
            if (domain.isEmpty()) {
                log("Enter a domain first")
                return@setOnClickListener
            }

            val protocol = protocolSpinner.selectedItem.toString()
            val upstream = upstreamInput.text.toString().trim()

            try {
                val rule = JSONObject().apply {
                    put("domain", domain)
                    put("protocol", protocol)
                    if (upstream.isNotEmpty()) {
                        put("upstream", upstream)
                    }
                }
                service?.addRule(rule.toString())
                log("Rule added: $domain ($protocol)")
                domainInput.text.clear()
                upstreamInput.text.clear()
                protocolSpinner.setSelection(0)
            } catch (e: Exception) {
                log("AddRule error: ${e.message}")
            }
        }

        flushRulesBtn.setOnClickListener {
            try {
                service?.flushRules()
                log("Rules flushed")
            } catch (e: Exception) {
                log("FlushRules error: ${e.message}")
            }
        }
    }

    override fun onDestroy() {
        if (bound) unbindService(connection)
        super.onDestroy()
    }

    private fun bindToService() {
        val resolveIntent = Intent("com.androidresolved.action.BIND_VPN_CONTROL").apply {
            `package` = "com.androidresolved"
        }
        val info = packageManager.resolveService(resolveIntent, 0)
        if (info != null) {
            log("Service resolved: ${info.serviceInfo.name}")
        } else {
            log("Service NOT resolvable via intent filter")
        }

        val intent = Intent().apply {
            component = ComponentName("com.androidresolved", "com.androidresolved.DnsVpnControlService")
        }
        val ok = bindService(intent, connection, Context.BIND_AUTO_CREATE)
        log("bindService returned $ok")
    }

    private fun updateButtons() {
        authBtn.isEnabled = !bound
        startBtn.isEnabled = bound
        stopBtn.isEnabled = bound
        addRuleBtn.isEnabled = bound
        flushRulesBtn.isEnabled = bound
    }

    private fun log(msg: String) {
        runOnUiThread {
            logText.append("\n> $msg")
            val scrollContainer = findViewById<android.widget.ScrollView>(R.id.logScroll)
            scrollContainer.post { scrollContainer.fullScroll(android.view.View.FOCUS_DOWN) }
        }
    }
}
