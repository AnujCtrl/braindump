package com.braindump.android.net

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import retrofit2.http.Body
import retrofit2.http.POST

/**
 * Wire-format types matching `core::sync::SyncPush` and `core::Todo` in the
 * Rust workspace. Field names are snake_case via [SerialName] so the JSON
 * shape lines up byte-for-byte with what the server emits/accepts.
 */
@Serializable
data class TodoWire(
    val id: String,
    val text: String,
    val source: String,
    val status: String,
    @SerialName("created_at") val createdAt: String,
    @SerialName("status_changed_at") val statusChangedAt: String,
    val urgent: Boolean,
    val important: Boolean,
    @SerialName("stale_count") val staleCount: Long,
    val tags: List<String>,
    val notes: List<String>,
    val done: Boolean,
    @SerialName("updated_at") val updatedAt: String,
)

@Serializable
data class TagWire(
    val name: String,
    @SerialName("created_at") val createdAt: Long,
    @SerialName("updated_at") val updatedAt: Long,
)

@Serializable
data class SyncPush(
    val todos: List<TodoWire>,
    val tags: List<TagWire> = emptyList(),
)

@Serializable
data class SyncPushResponse(
    @SerialName("applied_todos") val appliedTodos: Int = 0,
    @SerialName("skipped_todos") val skippedTodos: Int = 0,
    @SerialName("applied_tags") val appliedTags: Int = 0,
    @SerialName("skipped_tags") val skippedTags: Int = 0,
)

interface BraindumpApi {
    @POST("sync/push")
    suspend fun push(@Body body: SyncPush): SyncPushResponse
}
