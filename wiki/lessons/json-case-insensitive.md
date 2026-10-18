---
type: Lesson
title: Go's JSON struct decoding is case-insensitive
description: Decoding MCP messages into a struct would let a "Method" key shadow "method" and slip a tools/call past the proxy; exact-key maps fix it.
tags: [lesson, go, json, proxy, security]
timestamp: 2026-10-18T07:35:00Z
---

# Go's JSON struct decoding is case-insensitive

`encoding/json` matches object keys to struct fields **case-insensitively**, and when a key appears
twice the last one wins. So for

```json
{"jsonrpc":"2.0","id":8,"method":"tools/call","Method":"ping","params":{...}}
```

a `struct{ Method string \`json:"method"\` }` decoder sees `ping`, while a server whose parser is
case-sensitive (JavaScript's `JSON.parse`, for one) sees `tools/call` and runs the tool. That is a
policy bypass.

**Fix.** The proxy decodes into `map[string]json.RawMessage` and looks up `"method"`, `"id"` and
`"params"` exactly. The legacy transcript includes this message (id 8), and the test expects a
deny.

**Takeaway.** When a security decision depends on a JSON field that another program will also read,
parse it the way that program does, or read it more strictly, never more loosely.
