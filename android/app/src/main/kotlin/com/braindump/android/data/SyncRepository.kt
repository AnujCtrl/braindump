package com.braindump.android.data

import com.braindump.android.net.BraindumpApi
import com.braindump.android.net.SyncPush
import com.braindump.android.net.TodoWire
import kotlinx.serialization.builtins.ListSerializer
import kotlinx.serialization.builtins.serializer
import kotlinx.serialization.json.Json
import java.time.Instant
import java.time.format.DateTimeFormatter
import java.util.UUID

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
 */
class SyncRepository(
    private val todoDao: TodoDao,
    private val queueDao: QueueDao,
    private val apiProvider: () -> BraindumpApi?,
) {
    /** Capture a single todo. Returns the resulting Todo. */
    suspend fun capture(text: String, source: String = "android"): TodoEntity {
        val now = nowRfc3339()
        val tags = listOf("braindump")
        val notes = emptyList<String>()
        val todo = TodoEntity(
            id = UUID.randomUUID().toString(),
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

    /** Drain the outbound queue. No-op if no API is configured. */
    suspend fun drain(): DrainOutcome {
        val api = apiProvider() ?: return DrainOutcome.Disabled
        val pending = queueDao.head()
        if (pending.isEmpty()) return DrainOutcome.Idle
        val todos = pending.mapNotNull {
            runCatching { json.decodeFromString(TodoWire.serializer(), it.payload) }.getOrNull()
        }
        return try {
            api.push(SyncPush(todos = todos, tags = emptyList()))
            queueDao.delete(pending.map { it.id })
            DrainOutcome.Pushed(todos.size)
        } catch (e: Exception) {
            queueDao.markFailed(pending.map { it.id }, e.message ?: "unknown")
            DrainOutcome.Failed(e.message ?: "unknown")
        }
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
}

sealed class DrainOutcome {
    data object Disabled : DrainOutcome()
    data object Idle : DrainOutcome()
    data class Pushed(val count: Int) : DrainOutcome()
    data class Failed(val reason: String) : DrainOutcome()
}

private fun nowRfc3339(): String =
    DateTimeFormatter.ISO_INSTANT.format(Instant.now())
