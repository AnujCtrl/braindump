package com.braindump.android.data

/**
 * Port of `crates/core/src/parser.rs` — exact same grammar so captures from
 * Android produce the same structured form as the desktop client.
 *
 * Grammar:
 *   input     = (token | text)* [" -- " text]
 *   token     = tag | source | urgent | important | note
 *   tag       = "#" word
 *   source    = "@" word
 *   urgent    = "^^"
 *   important = "^^^"
 *   note      = "--note" quoted | "--note" word
 *   escape    = "\#word" -> literal "#word" in body text
 *
 * Processing order (mirrors Rust byte-for-byte):
 *   1. Extract all --note occurrences.
 *   2. Split on " -- " separator.
 *   3. Tokenize pre-separator portion (whitespace-split, quote-aware).
 *   4. Classify tokens.
 *   5. Append post-separator plain suffix.
 *   6. Join + trim.
 *
 * No tag-table lookup is performed; every #tag is accepted as-is. Tags are
 * deduplicated but otherwise preserved in order.
 */
object CaptureParser {

    data class ParsedCapture(
        val text: String,
        val tags: List<String>,
        val source: String?,
        val urgent: Boolean,
        val important: Boolean,
        val notes: List<String>,
    )

    fun parse(input: String): ParsedCapture? {
        val notes = mutableListOf<String>()
        var tags = mutableListOf<String>()
        var source: String? = null
        var urgent = false
        var important = false

        // Step 1: extract --note occurrences iteratively
        var raw = input
        while (raw.contains("--note")) {
            val (remaining, note) = extractNote(raw)
            raw = remaining
            if (note != null) notes.add(note) else break
        }

        // Step 2: split on separator
        val (preSep, plainSuffix) = splitSeparator(raw)

        // Step 3: tokenize
        val tokens = tokenize(preSep)

        // Step 4: classify
        val textWords = mutableListOf<String>()
        for (tok in tokens) {
            when {
                tok == "^^^" -> important = true
                tok == "^^" -> urgent = true
                tok.startsWith("\\#") -> textWords.add("#${tok.removePrefix("\\#")}")
                tok.startsWith("#") -> {
                    val name = tok.removePrefix("#")
                    if (name.isNotEmpty()) tags.add(name)
                }
                tok.startsWith("@") -> {
                    val name = tok.removePrefix("@")
                    if (name.isNotEmpty()) source = name
                }
                else -> textWords.add(tok)
            }
        }

        // Step 5: append plain suffix
        if (plainSuffix.isNotEmpty()) textWords.add(plainSuffix)

        // Step 6: join + trim
        val text = textWords.joinToString(" ").trim()

        if (text.isEmpty() && tags.isEmpty() && notes.isEmpty() && source == null) return null

        // Auto-tag with "braindump" when no explicit tag was provided
        if (tags.isEmpty()) tags.add("braindump")

        return ParsedCapture(
            text = text,
            tags = tags.distinct(),
            source = source,
            urgent = urgent,
            important = important,
            notes = notes,
        )
    }

    /** Returns (remaining, note?) after extracting one --note occurrence. */
    private fun extractNote(s: String): Pair<String, String?> {
        val idx = s.indexOf("--note").takeIf { it >= 0 } ?: return s to null
        val before = s.substring(0, idx)
        val after = s.substring(idx + "--note".length).trimStart()

        if (after.isEmpty()) return before.trimEnd() to null

        val (note, rest) = if (after.startsWith('"')) {
            val body = after.removePrefix("\"")
            val closeIdx = body.indexOf('"')
            if (closeIdx >= 0) {
                body.substring(0, closeIdx) to body.substring(closeIdx + 1).trimStart()
            } else {
                body to ""
            }
        } else {
            val spaceIdx = after.indexOf(' ')
            if (spaceIdx >= 0) {
                after.substring(0, spaceIdx) to after.substring(spaceIdx + 1)
            } else {
                after to ""
            }
        }

        val remaining = "$before $rest".trim()
        return remaining to note
    }

    /** Returns (preSeparator, plainSuffix). */
    private fun splitSeparator(raw: String): Pair<String, String> {
        if (raw.startsWith("-- ")) return "" to raw.removePrefix("-- ")
        if (raw == "--") return "" to ""
        val midIdx = raw.indexOf(" -- ")
        if (midIdx >= 0) return raw.substring(0, midIdx) to raw.substring(midIdx + 4)
        if (raw.endsWith(" --")) return raw.dropLast(3) to ""
        return raw to ""
    }

    /**
     * Whitespace-split, quote-aware tokenizer. Quotes are delimiters; unclosed
     * quote makes the rest one token.
     */
    private fun tokenize(s: String): List<String> {
        val trimmed = s.trim()
        if (trimmed.isEmpty()) return emptyList()

        val tokens = mutableListOf<String>()
        val current = StringBuilder()
        var inQuote = false

        for (ch in trimmed) {
            when {
                ch == '"' -> {
                    if (inQuote) {
                        if (current.isNotEmpty()) { tokens.add(current.toString()); current.clear() }
                        inQuote = false
                    } else {
                        if (current.isNotEmpty()) { tokens.add(current.toString()); current.clear() }
                        inQuote = true
                    }
                }
                ch == ' ' && !inQuote -> {
                    if (current.isNotEmpty()) { tokens.add(current.toString()); current.clear() }
                }
                else -> current.append(ch)
            }
        }
        if (current.isNotEmpty()) tokens.add(current.toString())
        return tokens
    }
}
