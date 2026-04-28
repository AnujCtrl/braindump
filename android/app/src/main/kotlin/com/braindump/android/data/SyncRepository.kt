package com.braindump.android.data

import android.content.SharedPreferences
import android.util.Log
import com.braindump.android.net.BraindumpApi
import com.braindump.android.net.SyncPush
import com.braindump.android.net.TodoWire
import kotlinx.serialization.builtins.ListSerializer
import kotlinx.serialization.builtins.serializer
import kotlinx.serialization.json.Json
import java.time.Instant
import java.time.format.DateTimeFormatter

private const val TAG = "SyncRepository"
private const val PREF_LAST_PULL = "last_pull_at"
private const val EPOCH = "1970-01-01T00:00:00Z"

private val json = Json {
    ignoreUnknownKeys = true
    encodeDefaults = true
}

private val stringListSerializer = ListSerializer(String.serializer())

/**
 * One-stop API for the rest of the app to capture todos and drain the queue.
 *
 * Capture path is offline-tolerant: it always writes to the local DB and the
 * outbound queue, then triggers a sync attempt. If the network is down, the
 * queue persists; the [com.braindump.android.sync.SyncWorker] retries.
 *
 * Sync is push-then-pull. The `since` cursor for pulling is stored in
 * [prefs] under [PREF_LAST_PULL] so each device only fetches deltas.
 * Conflict resolution is last-write-wins by `updated_at` (matches the
 * server and desktop clients).
 */
class SyncRepository(
    private val todoDao: TodoDao,
    private val queueDao: QueueDao,
    private val prefs: SharedPreferences,
    private val apiProvider: () -> BraindumpApi?,
) {
    /** Capture a single todo. Returns the resulting Todo. */
    suspend fun capture(text: String, source: String = "android"): TodoEntity {
        val now = nowRfc3339()
        val tags = listOf("braindump")
        val notes = emptyList<String>()
        val todo = TodoEntity(
            id = java.util.UUID.randomUUID().toString(),
            text = text.trim(),
            source = source,
            status = "inbox",
            createdAt = now,
            statusChangedAt = now,
            urgent = false,
            important = false,
            staleCount = 0,
            tagsJson = json.encodeToString(stringListSerializer, tags),
            notesJson = json.encodeToString(stringListSerializer, notes),
            done = false,
            updatedAt = now,
        )
        todoDao.upsert(todo)
        queueDao.insert(
            QueueEntry(
                todoId = todo.id,
                payload = json.encodeToString(TodoWire.serializer(), todo.toWire()),
                createdAt = System.currentTimeMillis(),
            ),
        )
        return todo
    }

    /**
     * Push the outbound queue, then pull updates from the server since the
     * last successful pull. Both steps are attempted; a push failure aborts
     * early (no point pulling if we couldn't push), but a pull failure only
     * logs — the cursor is not advanced, so the next sync retries it.
     */
    suspend fun drain(): DrainOutcome {
        val api = apiProvider() ?: return DrainOutcome.Disabled

        // Push outbound queue
        val pending = queueDao.head()
        var pushed = 0
        if (pending.isNotEmpty()) {
            val todos = pending.mapNotNull {
                runCatching { json.decodeFromString(TodoWire.serializer(), it.payload) }.getOrNull()
            }
            try {
                api.push(SyncPush(todos = todos, tags = emptyList()))
                queueDao.delete(pending.map { it.id })
                pushed = todos.size
                Log.i(TAG, "pushed $pushed todos")
            } catch (e: Exception) {
                queueDao.markFailed(pending.map { it.id }, e.message ?: "unknown")
                return DrainOutcome.Failed(e.message ?: "unknown")
            }
        }

        // Pull updates since the last successful pull (LWW by updated_at)
        val since = prefs.getString(PREF_LAST_PULL, null) ?: EPOCH
        val now = nowRfc3339()
        var pulled = 0
        try {
            val syncPull = api.pull(since)
            for (remote in syncPull.todos) {
                val local = todoDao.get(remote.id)
                val remoteTs = runCatching { Instant.parse(remote.updatedAt) }.getOrNull()
                val localTs = local?.let { runCatching { Instant.parse(it.updatedAt) }.getOrNull() }
                if (remoteTs != null && (localTs == null || remoteTs > localTs)) {
                    todoDao.upsert(remote.toEntity())
                    pulled++
                }
            }
            prefs.edit().putString(PREF_LAST_PULL, now).apply()
            Log.i(TAG, "pulled $pulled todos since $since")
        } catch (e: Exception) {
            // Pull failure is non-fatal — push succeeded; worker will retry on next cycle
            Log.w(TAG, "pull failed (cursor not advanced): ${e.message}")
        }

        return if (pushed == 0 && pulled == 0) DrainOutcome.Idle
        else DrainOutcome.Pushed(pushed + pulled)
    }

    suspend fun pendingCount(): Int = queueDao.pendingCount()

    private fun TodoEntity.toWire(): TodoWire = TodoWire(
        id = id,
        text = text,
        source = source,
        status = status,
        createdAt = createdAt,
        statusChangedAt = statusChangedAt,
        urgent = urgent,
        important = important,
        staleCount = staleCount,
        tags = json.decodeFromString(stringListSerializer, tagsJson),
        notes = json.decodeFromString(stringListSerializer, notesJson),
        done = done,
        updatedAt = updatedAt,
    )

    private fun TodoWire.toEntity(): TodoEntity = TodoEntity(
        id = id,
        text = text,
        source = source,
        status = status,
        createdAt = createdAt,
        statusChangedAt = statusChangedAt,
        urgent = urgent,
        important = important,
        staleCount = staleCount,
        tagsJson = json.encodeToString(stringListSerializer, tags),
        notesJson = json.encodeToString(stringListSerializer, notes),
        done = done,
        updatedAt = updatedAt,
    )
}

sealed class DrainOutcome {
    data object Disabled : DrainOutcome()
    data object Idle : DrainOutcome()
    data class Pushed(val count: Int) : DrainOutcome()
    data class Failed(val reason: String) : DrainOutcome()
}

private fun nowRfc3339(): String =
    DateTimeFormatter.ISO_INSTANT.format(Instant.now())
