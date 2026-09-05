# Agent file access

Agents MUST use targeted file access — never broad globs on large directories.

### DO

- Use Explore agent wherever reasonable
- `Glob("core/aws/{resource}*.go")` — find a specific fetcher
- `Glob("tests/unit/*{resource}*")` — find tests for a specific resource
- `Grep("mock.*{InterfaceName}", "tests/unit/mocks_test.go")` — find a specific mock
- `Glob("core/demo/fixtures/*.go")` — find a per-service fixture file
- `Glob("core/demo/fakes/*.go")` — find a typed-fake implementation
- `Grep("func Test.*{Resource}", "tests/unit/qa_yaml_child_views_test.go")` — find append point

### DON'T

- `Glob("tests/unit/*.go")` — returns 800 files, most irrelevant
- `Glob("core/aws/*.go")` — returns 377 files, most irrelevant
- `Glob("core/demo/*.go")` — only 4 files remain (client.go, handlers.go, costs_handlers.go, transport.go)
- Reading entire cross-cutting files (mocks_test.go, qa_detail_test.go) — grep for the section first

### Delegate to Explore for broad investigations

When a single task would require reading 5+ files totaling >500 lines, OR when you need to trace a feature across multiple packages (fetcher → view → related → test), dispatch an `Explore` agent and ask for a summarized report rather than reading everything into main context. Direct Grep/Glob/Read remain correct for targeted lookups (known file, specific symbol, < 3 queries). This protects the main context window for synthesis and decision-making.

#### Per-service fixture files are here (`core/demo/fixtures/`)
