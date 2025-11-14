# Tags in Code Blocks

This should extract #real-tag but not tags in code.

Here's some inline code with `#not-a-tag` that should be ignored.

```python
# This is a comment with #fake-tag
def function():
    # Another #fake-tag
    pass
```

But this #actual-tag should be extracted.
