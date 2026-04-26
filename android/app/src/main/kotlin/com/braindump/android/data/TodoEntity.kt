package com.braindump.android.data

import androidx.room.ColumnInfo
import androidx.room.Dao
import androidx.room.Entity
import androidx.room.Insert
import androidx.room.OnConflictStrategy
import androidx.room.PrimaryKey
import androidx.room.Query

/**
 * Local snapshot of a todo. Field shapes match the wire format the server
 * (and the desktop client) expect — see `core::Todo` in the Rust workspace.
 *
 * `tagsJson` and `notesJson` store JSON arrays as strings to mirror the
 * server schema exactly. `createdAt` etc. are RFC 3339 strings (also wire
 * format) — converting to `Instant` happens at the JSON boundary, not here.
 */
@Entity(tableName = "todos")
data class TodoEntity(
    @PrimaryKey val id: String,
    val text: String,
    val source: String,
    val status: String,
    @ColumnInfo(name = "created_at") val createdAt: String,
    @ColumnInfo(name = "status_changed_at") val statusChangedAt: String,
    val urgent: Boolean,
    val important: Boolean,
    @ColumnInfo(name = "stale_count") val staleCount: Long,
    @ColumnInfo(name = "tags_json") val tagsJson: String,
    @ColumnInfo(name = "notes_json") val notesJson: String,
    val done: Boolean,
    @ColumnInfo(name = "updated_at") val updatedAt: String,
)

@Dao
interface TodoDao {
    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun upsert(todo: TodoEntity)

    @Query("SELECT * FROM todos WHERE id = :id LIMIT 1")
    suspend fun get(id: String): TodoEntity?

    @Query("SELECT COUNT(*) FROM todos")
    suspend fun count(): Int
}
