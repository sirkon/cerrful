# 🧩 Cerrful v3.8 Rulebook

> **Custom Go linter enforcing careful, consistent, and researchable error handling.**
> Inspired by [`github.com/sirkon/errors`](https://github.com/sirkon/errors).
>
> **Goal:** Maintain structured, traceable, and high-SNR (signal-to-noise ratio) error propagation across Go codebases.

---

## 🔍 Overview

Cerrful enforces *discipline* in Go error handling:

* **No silent drops** — every error is either handled, wrapped, or logged.
* **No redundant noise** — logs remain concise and meaningful.
* **No ambiguity** — ownership of each error is always clear.
* **Structured context** — for high observability and machine processing.

Errors become compact, readable narratives:

```
err: get config: get config data: connection-refused
@err.context: # available when using structured error libs (e.g. github.com/sirkon/errors)
@WRAP: get config data
@location: /your/project/config/load.go:42
blob-key: "config:service-name"
```

---

## ✅ Philosophy

| Principle                                   | Meaning                                                              |
| ------------------------------------------- | -------------------------------------------------------------------- |
| **One owner per error**                     | Each error is either logged or propagated, never both.               |
| **Every step adds context**                 | No empty or redundant wrapping.                                      |
| **Text for humans, structure for machines** | Messages read naturally; context is machine-readable.                |
| **Stable under refactors**                  | Business-level descriptions, not function names.                     |
| **High log SNR**                            | Every word adds signal — no “failed”, “error”, or boilerplate noise. |

---

## ⚙️ Rule Index

| ID            | Name                                           | Purpose                                                                        |
| ------------- | ---------------------------------------------- | ------------------------------------------------------------------------------ |
| CER000        | **NoSilentDrop**                               | Errors must never be ignored.                                                  |
| CER010        | **AnnotateExternal**                           | External errors must be wrapped/annotated.                                     |
| CER020        | **SingleLocalPassthrough**                     | Local errors may be bare if single propagation point.                          |
| CER030        | **MultiReturnMustAnnotate**                    | Multiple return sites → each propagated error must be annotated.               |
| CER040        | **AnnotationRequiredForExternalAndMultiLocal** | Enforce annotation for externals and multi-propagation locals.                 |
| CER050        | **HandleInNonErrorFunc**                       | Errors in non-error-returning funcs must be logged or panicked.                |
| CER060        | **NoShadowing / Aliasing**                     | Reassigning unhandled `err` variables or aliasing tracked errors is forbidden. |
| CER070        | **RespectSentinels**                           | Allowed to drop configured benign sentinels (e.g. `io.EOF`).                   |
| CER080        | **RecognizeCustomIsAs**                        | Recognize custom `Is` / `As` as handled.                                       |
| CER090        | **CustomWrappers**                             | Recognize configured error wrappers as valid annotation.                       |
| CER100–CER145 | **Text and Style Rules**                       | Message formatting, forbidden words, punctuation, and case.                    |
| CER150        | **NoLogAndReturn**                             | Log *or* return — never both.                                                  |

---

## 🧬 Interpreter Model (CIR)

Cerrful transforms Go source into a **Contextual Intermediate Representation (CIR)**.
It captures only what matters for error semantics.

### CIR Nodes

* **Assign(name, [rhs])** — variable assignment or alias creation.
* **Wrap(name, msg)** — error wrapped with context.
* **Return(name)** — error returned.
* **Log(names[], level)** — one or more error variables logged.
* **If(then, else)** — conditional flow.
* **Loop(body)** / **Switch(cases)** — control structures.

Defers are inlined right before the `Return` node.
Non-error `if`, `for`, `switch`, or `select` are treated as *common* (ignored) blocks.

---

## 🧠 Aliasing and Reference Semantics

When a new variable references an existing tracked error:

```go
refErr := err
```

Cerrful creates a reference relationship rather than duplicating ownership.

### CIR

```
Assign("refErr", "err")
```

### Interpreter State

| Field   | Meaning                                                       |
| ------- | ------------------------------------------------------------- |
| `State` | Ownership status (`Open`, `Decorated`, `Logged`, `Returned`). |
| `Exact` | Known sentinel (`io.EOF`), if any.                            |
| `Class` | Error class (e.g., `io.ErrNoProgress`).                       |
| `Ref`   | Name of the variable this one refers to.                      |

### Behavior

* A change in any alias propagates to its entire reference graph.
* Propagation follows all links recursively, guarded by a `visited` set to prevent cycles.
* Cycles such as `newErr := err; err = newErr` are safe — each variable updates once.

### Example

```go
refErr := err
return errors.Wrap(refErr, "msg")
```

CIR:

```
Assign("refErr", "err")
Wrap("refErr", "msg")
Return("refErr")
```

Diagnostics:

```
CER060: aliasing error variable (err -> refErr)
```

Result: one clean report, consistent propagation, no duplicates.

---

## 🔧 Configuration

```yaml
sentinels:
  - io.EOF

wrappers:
  - package: github.com/sirkon/errors
    name: Wrap
    kind: wrap
  - package: github.com/sirkon/errors
    name: Annotate
    kind: wrap
  - package: github.com/sirkon/errors
    name: Just
    kind: transparent
  - package: fmt
    name: Errorf
    kind: format

loggers:
  - package: ""
    name: panic
    level: error

  - package: log
    name: Printf
    level: warn
  - package: log
    name: Print
    level: warn
  - package: log
    name: Println
    level: warn
  - package: log
    name: Fatal
    level: error
  - package: log
    name: Fatalf
    level: error
  - package: log
    name: Fatalln
    level: error

  - package: log/slog
    name: Debug
    level: other
  - package: log/slog
    name: Info
    level: other
  - package: log/slog
    name: Log
    level: other
  - package: log/slog
    name: Warn
    level: warn
  - package: log/slog
    name: Error
    level: error

  - package: testing
    name: Log
    level: warn
  - package: testing
    name: Logf
    level: warn
  - package: testing
    name: Error
    level: error
  - package: testing
    name: Errorf
    level: error
  - package: testing
    name: Fatal
    level: error
  - package: testing
    name: Fatalf
    level: error
```

---

## 🔍 Text Rules (Warnings by Default)

All text rules (CER100–CER145) emit warnings unless promoted to errors via configuration.

| Rule                | Default | Description                                          |
| ------------------- | ------- | ---------------------------------------------------- |
| **StartCase**       | warning | Must start lowercase or with acronym.                |
| **TrailingDot**     | warning | Must not end with a dot.                             |
| **ForbiddenTerms**  | warning | Disallow redundant terms (failed, error, etc.).      |
| **NoColonsInText**  | warning | Colons are reserved for wrapping separator.          |
| **NonEmptyMessage** | warning | Wrap/error messages must not be empty or whitespace. |

---

## 🔍 Integration

Add Cerrful to your GolangCI-Lint configuration:

```yaml
linters:
  enable:
    - cerrful
```

Supports `//nolint:cerrful` for intentional violations or transitional code.

---

## 🔟 License

MIT © 2025 Cerrful Authors

Cerrful v3.8 — enforce **meaning**, not mechanics.
High-SNR error handling for Go services and libraries.
