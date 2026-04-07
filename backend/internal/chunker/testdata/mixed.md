# Mixed Markdown Test

This file tests the extraction of nested code blocks within Markdown.

## Go Code Block

We demonstrate a Go function here:

```go
package demo
func SayHello(name string) {
    fmt.Printf("Hello, %s\n", name)
}
```

## SQL Code Block

We demo some SQL structure:

```sql
CREATE TABLE results (
    id INT,
    score FLOAT
);
SELECT id FROM results;
```

## Another language?

Maybe a small JSON:

```json
{
  "status": "ready",
  "data": [1, 2, 3]
}
```
