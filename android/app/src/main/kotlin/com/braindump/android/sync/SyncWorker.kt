package com.braindump.android.sync

import android.content.Context
import android.util.Log
import androidx.work.CoroutineWorker
import androidx.work.WorkerParameters
import com.braindump.android.BraindumpApp
import com.braindump.android.data.DrainOutcome

/**
 * Periodic sync drainer. WorkManager wakes us up roughly every 30 minutes
 * (see [SyncScheduler]) to push any queued captures and pull server updates.
 *
 * Network failures return [Result.retry] so WorkManager applies exponential
 * backoff. Auth/config failures (no API configured) return [Result.success]
 * — re-running won't help until the user fixes config.
 */
class SyncWorker(
    appContext: Context,
    params: WorkerParameters,
) : CoroutineWorker(appContext, params) {
    override suspend fun doWork(): Result {
        val container = (applicationContext as BraindumpApp).container
        return when (val outcome = container.repository.drain()) {
            DrainOutcome.Disabled, DrainOutcome.Idle -> Result.success()
            is DrainOutcome.Pushed -> {
                Log.i(TAG, "drained ${outcome.count} todos")
                Result.success()
            }
            is DrainOutcome.Failed -> {
                Log.w(TAG, "drain failed: ${outcome.reason}")
                if (runAttemptCount < MAX_ATTEMPTS) Result.retry() else Result.failure()
            }
        }
    }

    companion object {
        const val MAX_ATTEMPTS = 5
        private const val TAG = "SyncWorker"
    }
}
