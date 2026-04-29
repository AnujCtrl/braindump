package com.braindump.android

import android.app.Application
import com.braindump.android.data.AppContainer
import com.braindump.android.sync.SyncScheduler

/**
 * Application entry point.
 *
 * Holds the singleton [AppContainer] (Room DB, Retrofit client, repository).
 * Hilt is intentionally avoided — the dependency graph is small enough that
 * a hand-rolled holder is clearer and one less plugin to debug.
 */
class BraindumpApp : Application() {
    lateinit var container: AppContainer
        private set

    override fun onCreate() {
        super.onCreate()
        container = AppContainer(this)
        SyncScheduler.scheduleSync(this)
    }
}
