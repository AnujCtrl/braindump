package com.braindump.android.data

import androidx.room.Dao
import androidx.room.Entity
import androidx.room.Insert
import androidx.room.PrimaryKey
import androidx.room.Query

/**
 * Outbound sync queue. Each row holds a fully-formed Todo JSON snapshot
 * that the WorkManager-driven sync worker will POST to /sync/push.
 *
 * On success, rows are deleted by id. On failure, attempts/lastError are
 * updated and the worker retries with exponential backoff.
 */
@Entity(tableName = "queue")
data class QueueEntry(
    @PrimaryKey(autoGenerate = true) val id: Long = 0,
    val todoId: String,
    /** JSON snapshot — exactly what the server expects in `SyncPush.todos[*]`. */
    val payload: String,
    val createdAt: Long,
    val attempts: Int = 0,
    val lastError: String? = null,
)

@Dao
interface QueueDao {
    @Insert
    suspend fun insert(entry: QueueEntry): Long

    @Query("SELECT * FROM queue ORDER BY id ASC LIMIT :limit")
    suspend fun head(limit: Int = 100): List<QueueEntry>

    @Query("DELETE FROM queue WHERE id IN (:ids)")
    suspend fun delete(ids: List<Long>)

    @Query("UPDATE queue SET attempts = attempts + 1, lastError = :error WHERE id IN (:ids)")
    suspend fun markFailed(ids: List<Long>, error: String)

    @Query("SELECT COUNT(*) FROM queue")
    suspend fun pendingCount(): Int
}
