# cerrful CIR Brief (v19)

**Version:** 19  
**Date:** 2025‑10‑22  
**Scope:** Formal specification of the Cerrful Compiler Intermediate Representation (CIR).  
**Purpose:** Defines the structure, node semantics, and translation rules used by cerrful to represent Go error-handling logic.

---

## 1. CIR Model

### 1.1 Hierarchy

```
CIRProgram
 └── CIRFunction*
      └── Node*
```

### 1.2 Definitions

| Entity | Meaning |
|---------|----------|
| **CIRProgram** | Represents a single Go source file’s CIR output. |
| **CIRFunction** | Represents one Go function and its flattened error-handling control flow. |
| **Node** | Atomic unit of error logic (assignment, wrap, return, check, log, condition). |

---

## 2. Node Kinds

| Node | Description |
|-------|--------------|
| **Assign** | Error assignment statement. |
| **Wrap** | Wrapping of an existing error with additional context (e.g. `fmt.Errorf("msg: %w", err)`). |
| **Return** | Return of an error (direct or named). |
| **If** | Conditional branching over expressions like `if err != nil`. |
| **Log** | Logging call involving one or more known error variables. |
| **Check** | Predicate or checker function (e.g., `os.IsNotExist(err)`). |

---

## 3. Assign Sources (ADT)

### 3.1 Variants

| Variant | Description | Example |
|----------|--------------|----------|
| **AssignSourceCtor** | A constructor call creating a new error. | `errors.New("msg")`, `fmt.Errorf("msg")` without `%w` |
| **AssignSourceCall** | Function or method returning an error. | `doThing()` |
| **AssignSourceSentinel** | A constant or exported package-level error. | `os.ErrNotExist` |
| **AssignSourceAlias** | Alias of another variable. | `err := otherErr` |
| **AssignSourceTypeAssert** | Type assertion producing an error. | `err := t.(error)` |

Each variant implements `AssignSource`.

---

## 4. Naming Rules

| Context | Rule |
|----------|------|
| **Synthetic name** | `@err` is used when there is no explicit LHS variable (e.g. direct returns). |
| **Named return errors** | Functions with a named error result use that name for all in‑function error references. |
| **Aliasing** | Direct aliasing never rewrites to other names (no reverse assigns). |

---

## 5. Translation Rules

### 5.1 Constructors
Detected via `errors.New`, `fmt.Errorf` (without `%w`), and any configured constructors.

```
err := errors.New("msg")
→ Assign [err] <- NewError msg="msg" (via errors.New)
```

### 5.2 Wrappers
Detected via `fmt.Errorf` with `%w` format verb.

```
return fmt.Errorf("context: %w", err)
→ Assign [@err] <- err
→ Wrap [@err] msg="context" (via fmt.Errorf)
→ Return [@err]
```

### 5.3 Calls
Foreign and local calls returning an error:

```
_, err := os.Open(path)
→ Assign [err] <- os.Open(…) (foreign call)
```

### 5.4 Sentinels

```
err := os.ErrNotExist
→ Assign [err] <- os.ErrNotExist (foreign sentinel)
```

### 5.5 Aliases

```
err := otherErr
→ Assign [err] <- otherErr
```

### 5.6 Type Assertions

```
err := t.(error)
→ Assign [err] <- t.(error) (type assertion)
```

and for direct returns:

```
return t.(error)
→ Assign [@err] <- t.(error) (type assertion)
→ Return [@err]
```

### 5.7 Logging

Recognized via configured loggers (e.g., `fmt.Printf`, `log.Error`, `t.Fatal`).  
If an argument is a known error variable, it produces a `Log` node:

```
fmt.Printf("err: %v", err)
→ Log [err] level=warn (via fmt.Printf)
```

### 5.8 Checkers

```
os.IsNotExist(err)
→ Check [err] class=os.ErrNotExist (via os.IsNotExist)
```

### 5.9 Conditions

```
if err != nil { … }
→ If "err != nil":
    …
```

### 5.10 Direct Returns

```
return fmt.Errorf("wrap: %w", err)
→ Assign [@err] <- err
→ Wrap [@err] msg="wrap" (via fmt.Errorf)
→ Return [@err]
```

If no `%w` is present, it’s treated as a constructor instead of a wrap.

### 5.11 Success Returns (v18.3+)

All success-path returns (where the final result is `nil`) are omitted entirely:

```
return nil
return data, nil
→ (no CIR nodes generated)
```

---

## 6. Invariants

- Only **functions whose last return type is `error`** are analyzed.  
  (Other results are ignored unless explicitly configured.)
- The translator does **not** model success paths.
- The CIR is **flattened**: nested `if` bodies appear inline with indentation or braces depending on Pretty mode.
- All `Wrap` nodes refer to an existing error (never invent a new one).
- Every `AssignSource` must have exactly one concrete variant.

---

## 7. Output

### 7.1 Pretty Format (Indented)

```
Function foo:
  Assign [err] <- os.Open(…) (foreign call)
  If "err != nil":
    Wrap [@err] msg="open file" (via fmt.Errorf)
    Return [@err]
```

### 7.2 Compact Format (Curly Blocks)

```
Function foo {
  Assign [err] <- os.Open(…) (foreign call)
  If "err != nil" {
    Wrap [@err] msg="open file" (via fmt.Errorf)
    Return [@err]
  }
}
```

---

## 8. Configuration Keys (Summary)

| Field | Type | Description |
|--------|------|--------------|
| **Constructors** | `[]Ref` | Known error constructors. |
| **Loggers** | `[]LoggerSpec` | Recognized logging calls and severity. |
| **Checkers** | `[]CheckerSpec` | Recognized predicate functions. |
| **Ref** | `{Package, Name}` | Qualifier for function or constant. |

---

## 9. Version Log

| Version | Highlights |
|----------|-------------|
| v17 | Initial formal brief (Program → Function → Node hierarchy). |
| v18 | Unified AssignSource ADT; correct alias, wrap, and type-assert handling. |
| v18.3 | Removed phantasy assigns, enforced last-result-only error detection, and pruned success returns. |
| v19 | Formal specification form, distilled for reference. |

---

© 2025 cerrful project.
