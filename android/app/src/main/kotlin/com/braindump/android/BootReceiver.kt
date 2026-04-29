package com.braindump.android

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import com.braindump.android.sync.SyncScheduler

/**
 * Re-arm the periodic sync worker after device reboot. WorkManager mostly
 * survives reboots on its own these days, but registering this receiver is
 * cheap insurance and makes the intent explicit.
 */
class BootReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        if (intent.action == Intent.ACTION_BOOT_COMPLETED) {
            SyncScheduler.scheduleSync(context.applicationContext)
        }
    }
}
