# 🧱 Materialize Engram

Evaluates Go templates over a provided context and returns a small inline result
that bobrapet Stories can use for guards, routing, or computed metadata.

## 🌟 Highlights

- Supports `condition` and `object` modes for boolean checks and computed objects.
- Uses Sprig helpers plus custom `coalesce` utilities for safe accesses.
- Works with inline JSON or storage-ref inputs that are hydrated before evaluation.

## 🚀 Quick Start

```bash
go test ./...
```

Include the template and execution mode in your Story step, pointing `vars`
at the context you want to inspect.

## ⚙️ Configuration (`Engram.spec.with`)

This engram currently exposes no component-level `Engram.spec.with` options.
Its `configSchema` is empty, so each execution supplies its own evaluation
inputs.

## 📥 Inputs

The runtime accepts:

- `mode`: evaluation mode. `object` is the default when omitted.
- `template`: Go template string for `condition` mode, or any JSON/object value
  to resolve for `object` mode.
- `vars`: map of contextual inputs (often includes `inputs`, `steps`,
  `metadata`), including hydrated storage references.

Example:

```json
{
  "mode": "condition",
  "template": "{{ gt (len .steps.fetch.output.items) 0 }}",
  "vars": {"steps": {"fetch": {"output": {"items": [1,2,3]}}}}
}
```

## 📤 Outputs

- `result`: boolean or templated object, depending on `mode`.

## 🔄 Streaming Mode

- Deployment mode accepts JSON requests from `inputs`, `payload`, or binary
  frames.
- Empty and control frames are ignored without emitting a result.
- Responses are emitted as JSON in the outgoing `inputs`, `payload`, and binary
  frame while preserving inbound metadata keys.

## 🧪 Local Development

- `go test ./...` – Unit tests cover condition/object templates and error cases.
- `go vet ./...` – Ensure static analysis passes prior to release.

## 🤝 Community & Support

- [Contributing](./CONTRIBUTING.md)
- [Support](./SUPPORT.md)
- [Security Policy](./SECURITY.md)
- [Code of Conduct](./CODE_OF_CONDUCT.md)
- [Discord](https://discord.gg/dysrB7D8H6)

## 📄 License

Copyright 2025 BubuStack.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
