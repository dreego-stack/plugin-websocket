# Change files

Every pull request adds exactly one Markdown file in this directory.

```markdown
---
version: patch
---

- Feat: add X
```

Use `version: none` only when the release tag must not change. After merge,
the release workflow combines all pending files into one changelog entry and
removes them.