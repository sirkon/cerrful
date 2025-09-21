# cerrful

Linter that ensures an error processing respects the "protocol", where:

- You must "wrap" an error on return if a current function has more than two sources of errors.
- You must log an error if it is not returned out of a current function.
- Error annotation must have different texts of annotation for different sources in a current function.
- Usage of `:` is prohibited in annotation text.
- There must not be more than one log calls for an error.
- Errors that are leaving a function must not be logged within it.
- Some error types are above these rules.

## Current status.

This linter is in its early stages of development and does not support configuration yet.

Current defaults are:

### Error wrapping.

It is one of calls:

- `fmt.Errorf("<annotation>: %w", …, err)`
- `github.com/sirkon/errors.Wrap(err, "<annotation>")`
- `github.com/sirkon/errors.errors.Wrapf(err, "<annotation>", …)`

### Error logging.

Error is checked as logged if there are one of the following calls:

- `fmt.AAA(…, <err>, …)`, where AAA is one of `Print`, `Printf`, `Println`, `Fatal`, `Fatalf`, `Fatalffn`.
- `github.com/sirkon/message.BBB(…, <err>, …)`, where BBB is one of `Warning`, `Warningf`, `Error`, `Errorf`, `Fatal`, `Fatalf`, `Critical`, `Criticalf`.
- `XXX.CCC(…, <err>, …)`, where XXX is either `log` package or an instance of `*log.Logger` type and CCC is one of `Print`, `Printf`, `Println`, `Fatal`, `Fatalf`, `Fatalffn`.
- `YYY.CCC(…, <err>, …)`, where YYY is either `log/slog` package or an instance of `*log/slog.Logger` type and CCC is one of `Warn`, `WarnContext`, `Error`, `ErrorContext`.

Here `<err>` is (very much grammar like):

- An error variable that is being investigated.
- A wrap call with `<err>`, meaning this would be considered legal: `errors.Wrap(errors.Wrap(err, "do inner thing"), "do outer thing")`
- On of `errors.Join(…, <err>, …)` or `github.com/sirkon/errors.Join(…, <err>, …)` calls.

### Ignored error types/values.

Error of some types and/or values are free from obeying these rules. It is only `io.EOF` now though.

## Roadmap.

- [ ] Configuration support for adding new wrap calls via `<FuncPath>(<SigStyle>)`. `<SigStyle>` will be either `Errorf` or `Wrap`. 
- [ ] Configuration support for adding new logging calls via `<Src>.<Name>`, where `<Src>` is either package path or type path and `<Name>` is logging function/method name. With the same call tuple `(…, <err>, …)` of course.
- [ ] Configuration support for adding new god types/error values.
- [ ] Configuration support for adding functions via `<Src>.<Name>`, whose errors can be returned without a wrap.
