# 🧩 Cerrful — Error Processing Discipline for Go

> **Cerrful** is a Go linter enforcing deliberate, high-signal error handling.  
> It aims to keep error handling deliberate and meaningful across projects with developers of different experience levels, letting seniors focus on the parts of code design that actually matter.

---

## Origin and Purpose

Cerrful comes as the result of years of hands-on experience chasing real-world errors across Go codebases.  
When you spend long enough trying to make errors more diagnosable, traceable, and researchable,  
you start noticing patterns that have nothing to do with the language itself but everything to do with developer discipline.  
Even though following these rules proves especially powerful with feature-rich error libraries,  
they remain immensely useful even with the simplest ones — like Go’s standard `errors` package or `pkg/errors`.

Over time, a set of principles crystallized — small, practical rules that dramatically improved both the clarity and SNR of error messages.  
Cerrful turns these principles into a set of enforceable rules and checks your code against them.

---

## Philosophy

Cerrful’s foundation is simple:

*Every error in your program deserves a clear owner, a clear context, and a clear fate.*

This means:

- Errors are never silently dropped.
- Every boundary — where control passes between semantic layers — must add its own, meaningful context.
- Error messages must stay orthogonal: no redundant layers, no repeated wording, no “failed to do X” noise.
- Logs and returns are mutually exclusive paths of ownership.

This discipline keeps errors lightweight but rich in meaning — compact narratives rather than noise storms.

---

### Scope of Use

Cerrful’s principles apply to *developer-facing* error handling — the kind used inside services, libraries, and complex system interactions.  
They are not meant for user-facing messages in CLI tools, GUIs, or APIs that communicate directly with end users.  
In those contexts, errors are part of presentation, not diagnostics, and should follow their own communication design.

End-user errors are a different kind of beast in general.  
They are not runtime errors but expected outcomes — results that need to be reported, not returned.  
Conceptually, it looks more like this:

```go
type Reporter interface {
    Report(code ReportCode, ctx map[string]any)
}
```

taking care of i18n and whatever.

---

### Understanding the Rules

Cerrful’s rules are small and direct. Each exists to remove a specific kind of noise or ambiguity that commonly sneaks into Go code.  
Together they define a style of error handling that stays consistent across packages and teams, without forcing new idioms on anyone.

- Errors must never be ignored. Once a function returns one, it has to be either handled or propagated. That’s the foundation — they must be either returned or logged.
- When an error crosses a semantic boundary, it should be wrapped, giving the higher level of code the chance to explain what it was trying to do when that error occurred.
- Returning an unmodified error is fine if there’s only one such return path. If there are multiple, each needs its own annotation so that when something fails, it’s obvious which operation failed and why. These local propagation rules make error traces readable without excessive verbosity.
- Both external errors and multi-path locals must be annotated, ensuring that every boundary adds context once and only once.
- Some values, such as `io.EOF`, are not real failures. They indicate an expected condition. Cerrful allows marking such values as “non-errors,” keeping the signal clean.
- Message formatting and wording rules keep text short, factual, and consistent, avoiding redundancy.
- An error must be either logged or returned — never both. Duplicating output pollutes logs and makes the same failure appear twice (or, even worse, more).

All of this might sound strict, but the intent is simple: each rule preserves signal, context, and ownership.  
The outcome is a codebase where errors read like structured stories rather than noise.

---

### P.S. — A Nice Consequence

The rule *“wrap errors when they cross a semantic boundary”* turned out to be surprisingly telling.

At first, it was just a practical rule to keep wrapping where it makes sense — at package boundaries.  
But soon it revealed something deeper: when error messages began to overlap between packages, it reliably pointed to non-orthogonal code design —  
packages that didn’t own their responsibility cleanly.

In practice, it became a quiet design-quality indicator.  
A healthy codebase shows crisp, non-overlapping contexts; messy ones start echoing their neighbors.

---

### Addendum — Where Wrapping Belongs

Wrapping belongs to the caller, not the callee.

If a function feels the need to wrap the errors it just produced, that’s usually a sign that something’s wrong with its code design.  
It’s trying to describe context that already belongs outside its scope — mixing creation and interpretation.  
Each function should either create errors or reframe them, not both.

In practice, wrapping should mark the boundary where meaning changes — where a higher-level concept takes ownership of a lower-level one.

There are also cases where wrapping is intentionally not expected.  
Some functions naturally produce self-descriptive errors and don’t need extra context:
- Low-level “base meaning” providers, such as validators, decoders, or parsers.
- Recursive functions, where each invocation represents the same logical layer — wrapping them repeatedly would only multiply identical context.
- Authorization checks are a classic example: their errors already *are* the message.  
  Wrapping them adds nothing but noise.

---

## ⚙️ Rule Index

| ID            | Name                                           | Purpose                                                                        |
| ------------- | ---------------------------------------------- | ------------------------------------------------------------------------------ |
| **CER000**    | **NoSilentDrop**                               | Errors must never be ignored.                                                  |
| **CER010**    | **AnnotateExternal**                           | Wrap errors when they cross a semantic boundary.                               |
| **CER020**    | **SingleLocalPassthrough**                     | Local errors may be returned bare only if there’s a single propagation path.   |
| **CER030**    | **MultiReturnMustAnnotate**                    | Multiple return sites → each propagated error must be annotated.               |
| **CER040**    | **AnnotationRequiredForExternalAndMultiLocal** | Enforce annotation for externals and multi-propagation locals.                 |
| **CER050**    | **HandleInNonErrorFunc**                       | Errors in non-error-returning funcs must be logged or panicked.                |
| **CER060**    | **NoShadowing / Aliasing**                     | Reassigning or aliasing tracked errors is forbidden.                           |
| **CER070**    | **RespectSentinels**                           | Recognize configured sentinel values (e.g. `io.EOF`) as non-errors — they carry meaning but no failure. |
| **CER080**    | **RecognizeCustomIsAs**                        | Custom `Is` / `As` predicates count as handled.                                |
| **CER090**    | **CustomWrappers**                             | Recognize configured custom wrappers as valid annotation.                      |
| **CER100–CER145** | **Text and Style Rules**                   | Message formatting, punctuation, and forbidden terms.                          |
| **CER150**    | **NoLogAndReturn**                             | Error must be either logged or returned — never both.                          |

---

## Appendix: Error Handling Approaches in Go

The following are the most common approaches to error handling in Go, each with its own strengths and weaknesses.

### 1. Panic and recover

`panic` is fast (when error paths are not part of normal control flow) and perfect for traceability — every stack frame is visible.  
But it carries no additional context and offers little for researchability once printed.

### 2. `fmt.Errorf`, `errors.New`, and other text-based error packages

`fmt.Errorf` and similar tools provide context through text.  
They work well for short messages, but mixing context and cause in a single string breaks the natural “common → specific” reasoning flow.  
When messages pile up, meaning blurs.

### 3. Logging on error sites

Logs restore structure but fragment context across lines.  
They scale poorly in both volume and clarity, making them costly for both storage and comprehension.

### 4. Structured error libraries

Libraries like [`sirkon/errors`](https://github.com/sirkon/errors) combine traceability and researchability.  
They store context as data, not just formatted text — similar to structured logging.  
When logged, they produce compact, layered views of an error’s path. For example:

```json
{
  "time": "2025-10-03T02:56:10.326488+03:00",
  "level": "INFO",
  "source": {
    "function": "main.LogGrouped",
    "file": "/Users/d.cheremisov/Sources/work/errors/internal/example/main.go",
    "line": 106
  },
  "msg": "logging test with error context grouped by the places it was added",
  "grouped-structure": true,
  "err": "ask to do something: failed to do something",
  "@err": {
    "CTX": {
      "@location": "/Users/user/Sources/work/errors/internal/example/main.go:49",
      "pi": 3.141592653589793,
      "e": 2.718281828459045
    },
    "WRAP: ask to do something": {
      "@location": "/Users/user/Sources/work/errors/internal/example/main.go:46",
      "insert-locations": true
    },
    "NEW: failed to do something": {
      "@location": "/Users/user/Sources/work/errors/internal/example/main.go:41",
      "int-value": 13,
      "string-value": "world"
    }
  }
}
