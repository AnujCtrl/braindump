package com.braindump.android

import android.app.Activity
import android.content.Intent
import android.os.Bundle
import android.speech.RecognizerIntent
import android.view.WindowManager
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Mic
import androidx.compose.material.icons.outlined.Settings
import androidx.compose.material3.Button
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.unit.dp
import androidx.lifecycle.lifecycleScope
import com.braindump.android.sync.SyncScheduler
import kotlinx.coroutines.launch

/**
 * Capture window. Shown over the lock screen when summoned from the
 * quick-settings tile. Single-instance + excludeFromRecents so the user
 * never sees a stale capture sitting in the recent-apps switcher.
 */
class CaptureActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        // Show the IME immediately when the activity opens; saves a tap.
        window.setSoftInputMode(WindowManager.LayoutParams.SOFT_INPUT_STATE_ALWAYS_VISIBLE)

        val initialText = when (intent?.action) {
            Intent.ACTION_SEND -> intent.getStringExtra(Intent.EXTRA_TEXT).orEmpty()
            ACTION_CAPTURE -> intent.getStringExtra(EXTRA_TEXT).orEmpty()
            else -> ""
        }

        setContent {
            BraindumpTheme {
                Surface(modifier = Modifier.fillMaxSize()) {
                    CaptureScreen(
                        initialText = initialText,
                        onSubmit = { text -> submit(text) },
                        onDismiss = { finish() },
                        onVoice = { startVoice() },
                    )
                }
            }
        }
    }

    private fun submit(text: String) {
        if (text.isBlank()) {
            finish()
            return
        }
        val container = (application as BraindumpApp).container
        lifecycleScope.launch {
            container.repository.capture(text)
            // Kick the sync worker so the user's capture lands on the server
            // without waiting for the next periodic tick.
            SyncScheduler.requestImmediateSync(this@CaptureActivity)
            finish()
        }
    }

    // --- Voice ---

    private val voiceLauncher = registerForActivityResult(
        ActivityResultContracts.StartActivityForResult(),
    ) { result ->
        if (result.resultCode == Activity.RESULT_OK) {
            val transcript = result.data
                ?.getStringArrayListExtra(RecognizerIntent.EXTRA_RESULTS)
                ?.firstOrNull()
                .orEmpty()
            if (transcript.isNotBlank()) {
                voiceTranscript.value = transcript
            }
        }
    }

    /** Mutable state shared with the Compose tree to feed transcripts back. */
    private val voiceTranscript = androidx.compose.runtime.mutableStateOf<String?>(null)

    private fun startVoice() {
        val intent = Intent(RecognizerIntent.ACTION_RECOGNIZE_SPEECH).apply {
            putExtra(
                RecognizerIntent.EXTRA_LANGUAGE_MODEL,
                RecognizerIntent.LANGUAGE_MODEL_FREE_FORM,
            )
            putExtra(RecognizerIntent.EXTRA_PROMPT, getString(R.string.voice_prompt))
        }
        runCatching { voiceLauncher.launch(intent) }.onFailure {
            // Device has no recognizer — silently fall back to text-only.
        }
    }

    @Composable
    private fun CaptureScreen(
        initialText: String,
        onSubmit: (String) -> Unit,
        onDismiss: () -> Unit,
        onVoice: () -> Unit,
    ) {
        var text by remember { mutableStateOf(initialText) }
        val focus = remember { FocusRequester() }
        val scope = rememberCoroutineScope()

        // If a voice transcript arrives, append/replace.
        LaunchedEffect(voiceTranscript.value) {
            voiceTranscript.value?.let { transcribed ->
                text = if (text.isBlank()) transcribed else "$text $transcribed"
                voiceTranscript.value = null
            }
        }

        LaunchedEffect(Unit) { focus.requestFocus() }

        Box(
            modifier = Modifier
                .fillMaxSize()
                .imePadding()
                .padding(16.dp),
        ) {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .align(Alignment.BottomCenter),
                verticalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                OutlinedTextField(
                    value = text,
                    onValueChange = { text = it },
                    label = { Text(stringResource(R.string.capture_label)) },
                    placeholder = { Text(stringResource(R.string.capture_placeholder)) },
                    modifier = Modifier
                        .fillMaxWidth()
                        .focusRequester(focus),
                    minLines = 3,
                )
                Row(verticalAlignment = Alignment.CenterVertically) {
                    IconButton(onClick = onVoice) {
                        Icon(
                            Icons.Outlined.Mic,
                            contentDescription = stringResource(R.string.voice_cd),
                        )
                    }
                    IconButton(onClick = { openSettings() }) {
                        Icon(
                            Icons.Outlined.Settings,
                            contentDescription = stringResource(R.string.settings_cd),
                        )
                    }
                    Spacer(Modifier.width(8.dp))
                    Button(onClick = { scope.launch { onSubmit(text) } }) {
                        Text(stringResource(R.string.capture_submit))
                    }
                    Spacer(Modifier.width(8.dp))
                    Button(onClick = onDismiss) {
                        Text(stringResource(R.string.capture_dismiss))
                    }
                }
            }
        }
    }

    companion object {
        const val ACTION_CAPTURE = "com.braindump.android.CAPTURE"
        const val EXTRA_TEXT = "text"
    }
}

@Composable
private fun stringResource(id: Int): String =
    androidx.compose.ui.res.stringResource(id)

@Composable
private fun BraindumpTheme(content: @Composable () -> Unit) {
    MaterialTheme(content = content)
}
