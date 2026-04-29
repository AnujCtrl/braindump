package com.braindump.android.data

import android.content.Context
import android.content.SharedPreferences
import androidx.room.Room
import com.braindump.android.net.ApiClient
import com.braindump.android.net.BraindumpApi
import okhttp3.OkHttpClient

private const val PREFS_NAME = "braindump-settings"
const val PREF_SERVER_URL = "server_url"

/**
 * Manual DI container. Holds singletons constructed at app start: Room DB,
 * OkHttp/Retrofit, repository. Avoid Hilt — the graph is shallow.
 */
class AppContainer(context: Context) {
    private val appContext = context.applicationContext

    val prefs: SharedPreferences =
        appContext.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)

    val database: AppDatabase = Room.databaseBuilder(
        appContext,
        AppDatabase::class.java,
        "braindump.db",
    )
        .fallbackToDestructiveMigration()
        .build()

    private val httpClient: OkHttpClient = ApiClient.buildClient()

    /** Returns null when the server URL hasn't been configured yet. */
    fun api(): BraindumpApi? {
        val raw = prefs.getString(PREF_SERVER_URL, null)?.takeIf { it.isNotBlank() } ?: return null
        return ApiClient.buildRetrofit(raw, httpClient).create(BraindumpApi::class.java)
    }

    val repository: SyncRepository by lazy {
        SyncRepository(database.todoDao(), database.queueDao(), prefs) { api() }
    }
}
