---
description: Generate a conventional commit message for staged/unstaged changes
---

## Context

- Git diff: !`git diff HEAD`
- Recent commits: !`git log --oneline -5`

## Your task

Look at the changes and output a single commit message in this exact format:

`type: short description`

Where `type` is one of: `feat`, `fix`, `style`, `refactor`, `test`, `chore`, `docs`

Rules:
- All lowercase
- No period at the end
- Max ~60 characters
- Just output the message, nothing else
