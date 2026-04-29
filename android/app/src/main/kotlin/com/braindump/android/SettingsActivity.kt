package com.braindump.android

import android.content.Context
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Button
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import com.braindump.android.data.PREF_SERVER_URL

/**
 * Single-screen settings: enter the home server's Tailscale URL. There's
 * intentionally no auth field — Tailscale gates access at the network layer
 * and this is a single-user system.
 *
 * For power users / first-run automation, the same value can be set via:
 *   adb shell am broadcast -a com.braindump.android.SET_SERVER_URL --es url "http://..."
 * (no broadcast receiver yet — easy follow-up.)
 */
class SettingsActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        val app = application as BraindumpApp
        val prefs = app.container.prefs
        setContent {
            MaterialTheme {
                Surface(modifier = Modifier.fillMaxSize()) {
                    SettingsScreen(
                        currentUrl = prefs.getString(PREF_SERVER_URL, "").orEmpty(),
                        onSave = { url ->
                            prefs.edit().putString(PREF_SERVER_URL, url.trim()).apply()
                            finish()
                        },
                    )
                }
            }
        }
    }
}

@Composable
private fun SettingsScreen(
    currentUrl: String,
    onSave: (String) -> Unit,
) {
    var url by remember { mutableStateOf(currentUrl) }
    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(24.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        Text(
            text = "Sync server",
            style = MaterialTheme.typography.headlineSmall,
        )
        Text(
            text = "Tailscale-only URL for the home sync hub. Empty = local-only.",
            style = MaterialTheme.typography.bodyMedium,
        )
        OutlinedTextField(
            value = url,
            onValueChange = { url = it },
            label = { Text("URL") },
            placeholder = { Text("http://braindump.tail-scale.ts.net:8181") },
            singleLine = true,
            modifier = Modifier.fillMaxWidth(),
        )
        Button(onClick = { onSave(url) }) { Text("Save") }
    }
}

/** Convenience for opening Settings from anywhere in the app. */
fun Context.openSettings() {
    val i = android.content.Intent(this, SettingsActivity::class.java)
        .addFlags(android.content.Intent.FLAG_ACTIVITY_NEW_TASK)
    startActivity(i)
}
