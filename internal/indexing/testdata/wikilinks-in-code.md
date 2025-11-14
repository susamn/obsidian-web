# Wikilinks in Code Blocks

This tests that wikilinks inside code blocks are ignored.

Valid link: [[Valid Note]]

Code block with wikilinks (should be ignored):
```markdown
This is an example of a wikilink: [[Ignored Note]]
Another example: [[Also Ignored]]
```

Inline code with wikilink: `[[Inline Ignored]]` should be ignored too.

But this [[Another Valid Note]] should work.

```
[[Code Block Note]]
[[Another Code Note]]
```

Final valid link: [[Final Note]]
