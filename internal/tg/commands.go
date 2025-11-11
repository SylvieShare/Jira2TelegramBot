package tg

import "strings"

// StripCommandText removes any leading telegram command token
// like "/command" or "/command@botname" from the start of text
// and returns the remaining trimmed string.
// Example: "/status_issue@mybot KEY-1" -> "KEY-1"
func StripCommandText(text string) string {
    s := strings.TrimSpace(text)
    if s == "" {
        return ""
    }
    if !strings.HasPrefix(s, "/") {
        return s
    }
    // Remove first token that starts with '/'
    // Token ends at first whitespace.
    if i := strings.IndexAny(s, " \t\n\r"); i >= 0 {
        s = s[i+1:]
    } else {
        // Only command was provided
        s = ""
    }
    return strings.TrimSpace(s)
}