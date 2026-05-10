
# CLAUDE.md

Behavioral guidelines to reduce common LLM coding mistakes. Merge with project-specific instructions as needed.

**Tradeoff:** These guidelines bias toward caution over speed. For trivial tasks, use judgment.

## 1. Think Before Coding

**Don't assume. Don't hide confusion. Surface tradeoffs.**

Before implementing:

- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them - don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

## 2. Simplicity First

**Minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.

Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

## 3. Surgical Changes

**Touch only what you must. Clean up only your own mess.**

When editing existing code:

- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it - don't delete it.

When your changes create orphans:

- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

The test: Every changed line should trace directly to the user's request.

## 4. Goal-Driven Execution

**Define success criteria. Loop until verified.**

Transform tasks into verifiable goals:

- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:

```
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
```

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.

## 5. when working in picoceci source files, follow the same guidelines but also ensure that the code is valid picoceci syntax and semantics. If you are unsure about how to implement something in picoceci, ask for clarification or refer to the picoceci documentation and examples

Make sure that if you're about to make changes to picoceci source files, just describe what yo htink needs to be done so the user can tell picoceci's agent to consider it all and suggest the changes that will provide what Canal needs to be able to run picoceci on the WiFi domain. This is a critical step because if you make changes to picoceci source files without fully understanding how they work, you might introduce bugs or break existing functionality. By describing your intended changes and asking for clarification, you can ensure that your modifications are aligned with picoceci's design and won't cause unintended consequences. Always prioritize clear communication and understanding when working with complex codebases like picoceci
---

**These guidelines are working if:** fewer unnecessary changes in diffs, fewer rewrites due to overcomplication, and clarifying questions come before implementation rather than after mistakes.

## Hardware Baseline (Canal ESP32-S3)

- Assume board is ESP32-S3 N16R8 unless user states otherwise.
- Treat 16MB flash + 8MB PSRAM as canonical hardware.
- Keep IDF config aligned with hardware:
  - `CONFIG_ESPTOOLPY_FLASHSIZE_16MB=y`
  - `CONFIG_ESPTOOLPY_FLASHSIZE="16MB"`
  - `CONFIG_SPIRAM=y`
  - `CONFIG_ESP32S3_SPIRAM_SUPPORT=y`
- If runtime logs show mismatches (for example flash header reports 2MB), fix sdkconfig before debugging higher-level runtime issues.
